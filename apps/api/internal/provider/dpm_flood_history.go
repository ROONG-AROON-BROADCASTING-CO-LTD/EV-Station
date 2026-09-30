package provider

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/rbc/ev-station/apps/api/internal/cache"
	"github.com/rbc/ev-station/apps/api/internal/domain"
	"github.com/rbc/ev-station/apps/api/internal/telemetry"
)

const (
	dpmFloodHistoryReference = "https://data.go.th/dataset/gdpublish-dfs_21_02"
	dpmFloodHistoryCSVPart1  = "https://catalog.disaster.go.th/dataset/26674dbc-d656-4f4f-8afd-5a62c5b5e5b0/resource/5ee190e3-e031-4e1c-807d-95dacd40d822/download/dpm-gd015__1.csv"
	dpmFloodHistoryCSVPart2  = "https://catalog.disaster.go.th/dataset/26674dbc-d656-4f4f-8afd-5a62c5b5e5b0/resource/60267cd1-6cdb-43f4-a457-04d5409d1733/download/dpm-gd015__2.csv"
)

//go:embed data/ddpm_flood_snapshot.json
var dpmFloodSnapshotJSON []byte

type dpmProvincialFloodReport struct {
	Province                 string `json:"province"`
	Year                     int    `json:"year"`
	ReportedOccurrences      int    `json:"reportedOccurrences"`
	AffectedDistrictCount    int    `json:"affectedDistrictCount"`
	AffectedSubdistrictCount int    `json:"affectedSubdistrictCount"`
	AffectedCommunityCount   int    `json:"affectedCommunityCount"`
	AffectedPeople           int    `json:"affectedPeople"`
	AffectedHouseholds       int    `json:"affectedHouseholds"`
	GeographicScope          string `json:"geographicScope"`
	License                  string `json:"license"`
	SourceURL                string `json:"sourceUrl"`
}

type dpmFloodSnapshot struct {
	SchemaVersion   int                        `json:"schemaVersion"`
	ImportedAt      time.Time                  `json:"importedAt"`
	License         string                     `json:"license"`
	PeriodStartYear int                        `json:"periodStartYear"`
	PeriodEndYear   int                        `json:"periodEndYear"`
	Districts       []dpmFloodAreaSummary      `json:"districts"`
	ProvinceReports []dpmProvincialFloodReport `json:"provinceReports"`
}

var dpmSnapshotOnce sync.Once
var dpmSnapshot dpmFloodSnapshot
var dpmSnapshotError error

func loadDPMFloodSnapshot() (*dpmFloodSnapshot, error) {
	dpmSnapshotOnce.Do(func() {
		dpmSnapshotError = json.Unmarshal(dpmFloodSnapshotJSON, &dpmSnapshot)
		if dpmSnapshotError == nil && (dpmSnapshot.SchemaVersion != 1 || dpmSnapshot.License != "Open Data Common" || dpmSnapshot.PeriodStartYear != 2562 || dpmSnapshot.PeriodEndYear != 2567 || len(dpmSnapshot.Districts) == 0) {
			dpmSnapshotError = fmt.Errorf("invalid embedded DDPM flood snapshot")
		}
	})
	return &dpmSnapshot, dpmSnapshotError
}

func dpmProvinceReport(province string) *dpmProvincialFloodReport {
	snapshot, err := loadDPMFloodSnapshot()
	if err != nil {
		return nil
	}
	for _, report := range snapshot.ProvinceReports {
		if normalizeDPMAreaName(report.Province) == normalizeDPMAreaName(province) {
			return &report
		}
	}
	return nil
}

// DPMFloodHistoryConfig points at the public DDPM history of village-level flood
// reports. It is intentionally summarized at district scope, not interpreted as
// parcel-level inundation evidence.
type DPMFloodHistoryConfig struct {
	CSVURLs   []string
	CacheTTL  time.Duration
	UserAgent string
}

type DPMFloodHistoryProvider struct {
	config   DPMFloodHistoryConfig
	client   *http.Client
	cache    cache.Cache
	geocoder ReverseGeocoder

	mu          sync.Mutex
	areas       map[string]dpmFloodAreaSummary
	loadedUntil time.Time
}

type dpmFloodAreaSummary struct {
	Province                 string `json:"province"`
	District                 string `json:"district"`
	ReportedYears            []int  `json:"reportedYears"`
	ReportedVillageIncidents int    `json:"reportedVillageIncidents"`
	AffectedSubdistrictCount int    `json:"affectedSubdistrictCount"`
}

type dpmFloodAreaAccumulator struct {
	province     string
	district     string
	years        map[int]struct{}
	incidents    int
	subdistricts map[string]struct{}
}

type dpmFloodMetricValue struct {
	AssessmentType           string                    `json:"assessmentType"`
	Province                 string                    `json:"province"`
	District                 string                    `json:"district"`
	PeriodStartYear          int                       `json:"periodStartYear"`
	PeriodEndYear            int                       `json:"periodEndYear"`
	ReportedFloodYears       []int                     `json:"reportedFloodYears"`
	ReportedFloodYearCount   int                       `json:"reportedFloodYearCount"`
	ReportedVillageIncidents int                       `json:"reportedVillageIncidents"`
	AffectedSubdistrictCount int                       `json:"affectedSubdistrictCount"`
	LatestProvinceReport     *dpmProvincialFloodReport `json:"latestProvinceReport,omitempty"`
}

func NewDPMFloodHistoryProvider(config DPMFloodHistoryConfig, client *http.Client, externalCache cache.Cache, geocoder ReverseGeocoder) *DPMFloodHistoryProvider {
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	if externalCache == nil {
		externalCache = cache.Noop{}
	}
	if len(config.CSVURLs) > 0 {
		urls := make([]string, 0, len(config.CSVURLs))
		for _, url := range config.CSVURLs {
			if url = strings.TrimSpace(url); url != "" {
				urls = append(urls, url)
			}
		}
		config.CSVURLs = urls
	}
	if config.CacheTTL <= 0 {
		config.CacheTTL = 30 * 24 * time.Hour
	}
	return &DPMFloodHistoryProvider{config: config, client: client, cache: externalCache, geocoder: geocoder}
}

func (p *DPMFloodHistoryProvider) Collect(ctx context.Context, site domain.Site, _ int) ([]Observation, error) {
	observations, positions := unavailableObservations()
	if site.Latitude == nil || site.Longitude == nil {
		observations[positions["flood"]] = p.missing("Valid coordinates are required to match the site to a district; no flood-history conclusion was produced.")
		return observations, nil
	}
	if p.geocoder == nil {
		observations[positions["flood"]] = p.missing("District lookup is unavailable; no flood-history conclusion was produced.")
		return observations, nil
	}

	location, err := p.geocoder.Reverse(ctx, *site.Latitude, *site.Longitude)
	if err != nil || strings.TrimSpace(location.Province) == "" || strings.TrimSpace(location.District) == "" {
		observations[positions["flood"]] = p.missing("The submitted coordinates could not be matched to a district; no flood-history conclusion was produced.")
		return observations, nil
	}

	areas, err := p.loadAreas(ctx)
	if err != nil {
		observations[positions["flood"]] = p.missingWithProvince("The DDPM public flood-history file was unavailable or could not be read; no flood-history conclusion was produced.", location.Province)
		return observations, nil
	}
	area, found := findDPMFloodArea(areas, location.Province, location.District)
	if !found {
		observations[positions["flood"]] = p.missingWithProvince("The DDPM file has no exact historical flood record for this district. Missing records are not interpreted as no flood risk.", location.Province)
		return observations, nil
	}

	value := dpmFloodMetricValue{
		AssessmentType: "administrative_district_historical_reports",
		Province:       area.Province, District: area.District,
		PeriodStartYear: 2562, PeriodEndYear: 2567,
		ReportedFloodYears: area.ReportedYears, ReportedFloodYearCount: len(area.ReportedYears),
		ReportedVillageIncidents: area.ReportedVillageIncidents,
		AffectedSubdistrictCount: area.AffectedSubdistrictCount,
		LatestProvinceReport:     dpmProvinceReport(location.Province),
	}
	raw, _ := json.Marshal(value)
	observations[positions["flood"]] = Observation{
		MetricType: "flood", RawValue: raw, Status: domain.DataEstimated,
		Source: domain.DataSource{
			Name: "Department of Disaster Prevention and Mitigation — village flood-history statistics",
			Type: "official_open_data", Authority: "official", GeographicScope: "district",
			SiteVerification: "reported_at_administrative_area", ReferenceURI: dpmFloodHistoryReference,
			DatasetVersion: "B.E. 2562–2567", RetrievedAt: time.Now().UTC(),
			Methodology: "Aggregates DDPM-reported village flood occurrences by district and counts the years with at least one report, 2019–2024.",
			License:     "Open Data Common",
		},
		Assumptions: []string{
			"This is a district-level history of reported village floods, not confirmation that the submitted land parcel flooded.",
			"The dataset lists villages with reported flood impacts, not a complete inventory of every village or land parcel; a district without a matched report is marked as no data, not as flood-free.",
			"Reported years indicate recurrence in the administrative district, not flood depth, duration, forecast, or parcel-specific probability.",
			"The result is a screening indicator for discussion and does not decide whether to invest.",
			"The 2025 provincial summary is additional context only and is excluded from district-history scoring. District counts in that source are totals, not named districts.",
		},
	}
	if len(p.config.CSVURLs) == 0 {
		if snapshot, snapshotErr := loadDPMFloodSnapshot(); snapshotErr == nil {
			observations[positions["flood"]].Source.RetrievedAt = snapshot.ImportedAt
		}
	}
	return observations, nil
}

func (p *DPMFloodHistoryProvider) loadAreas(ctx context.Context) (map[string]dpmFloodAreaSummary, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.areas) > 0 && time.Now().Before(p.loadedUntil) {
		return p.areas, nil
	}
	if len(p.config.CSVURLs) == 0 {
		snapshot, err := loadDPMFloodSnapshot()
		if err != nil {
			return nil, err
		}
		areas := make(map[string]dpmFloodAreaSummary, len(snapshot.Districts))
		for _, area := range snapshot.Districts {
			areas[dpmFloodAreaKey(area.Province, area.District)] = area
		}
		p.areas = areas
		p.loadedUntil = time.Now().Add(p.config.CacheTTL)
		return areas, nil
	}

	payloads := make([][]byte, 0, len(p.config.CSVURLs))
	for _, url := range p.config.CSVURLs {
		payload, err := p.loadCSV(ctx, url)
		if err != nil {
			return nil, err
		}
		payloads = append(payloads, payload)
	}

	areas, err := parseDPMFloodHistoryCSVs(payloads)
	if err != nil {
		return nil, err
	}
	if len(areas) == 0 {
		return nil, fmt.Errorf("DDPM flood-history CSV contains no district records")
	}
	p.areas = areas
	p.loadedUntil = time.Now().Add(p.config.CacheTTL)
	return areas, nil
}

func (p *DPMFloodHistoryProvider) loadCSV(ctx context.Context, url string) ([]byte, error) {
	cacheKey := "dpm:flood-history:v2:" + hashDPMFlood(url)
	if cached, found, err := p.cache.Get(ctx, cacheKey); err == nil && found && len(cached) > 0 {
		return cached, nil
	}

	requestContext := telemetry.WithOperation(ctx, "dpm_flood_history_download")
	request, err := http.NewRequestWithContext(requestContext, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "text/csv,application/octet-stream;q=0.9,*/*;q=0.8")
	request.Header.Set("User-Agent", p.config.UserAgent)
	response, err := p.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("DDPM flood-history source returned status %d", response.StatusCode)
	}
	payload, err := io.ReadAll(io.LimitReader(response.Body, 12<<20))
	if err != nil {
		return nil, err
	}
	if len(payload) >= 12<<20 {
		return nil, fmt.Errorf("DDPM flood-history CSV exceeds the 12 MB safety limit")
	}
	if len(payload) == 0 {
		return nil, fmt.Errorf("DDPM flood-history CSV is empty")
	}
	_ = p.cache.Set(ctx, cacheKey, payload, p.config.CacheTTL)
	return payload, nil
}

func parseDPMFloodHistoryCSV(payload []byte) (map[string]dpmFloodAreaSummary, error) {
	return parseDPMFloodHistoryCSVs([][]byte{payload})
}

func parseDPMFloodHistoryCSVs(payloads [][]byte) (map[string]dpmFloodAreaSummary, error) {
	accumulators := make(map[string]*dpmFloodAreaAccumulator)
	seenRecords := make(map[string]struct{})
	for _, payload := range payloads {
		reader := csv.NewReader(strings.NewReader(strings.TrimPrefix(string(payload), "\ufeff")))
		reader.FieldsPerRecord = -1
		rows, err := reader.ReadAll()
		if err != nil || len(rows) < 2 {
			return nil, fmt.Errorf("invalid DDPM flood-history CSV")
		}

		columns := make(map[string]int, len(rows[0]))
		for index, name := range rows[0] {
			columns[strings.TrimSpace(name)] = index
		}
		for _, required := range []string{"PROVINCE_NAME", "AMPHUR_NAME", "DISTRICT_CODE"} {
			if _, found := columns[required]; !found {
				return nil, fmt.Errorf("DDPM flood-history CSV is missing column %s", required)
			}
		}
		for year := 2562; year <= 2567; year++ {
			if _, found := columns["Y"+strconv.Itoa(year)]; !found {
				return nil, fmt.Errorf("DDPM flood-history CSV is missing year %d", year)
			}
		}

		for _, row := range rows[1:] {
			province := strings.TrimSpace(dpmCSVField(row, columns, "PROVINCE_NAME"))
			district := strings.TrimSpace(dpmCSVField(row, columns, "AMPHUR_NAME"))
			if province == "" || district == "" {
				continue
			}
			recordKey := normalizedDPMRowKey(row)
			if _, duplicate := seenRecords[recordKey]; duplicate {
				continue
			}
			seenRecords[recordKey] = struct{}{}

			key := dpmFloodAreaKey(province, district)
			accumulator := accumulators[key]
			if accumulator == nil {
				accumulator = &dpmFloodAreaAccumulator{province: province, district: district, years: make(map[int]struct{}), subdistricts: make(map[string]struct{})}
				accumulators[key] = accumulator
			}
			rowHasReport := false
			for year := 2562; year <= 2567; year++ {
				occurrences := dpmParseCount(dpmCSVField(row, columns, "Y"+strconv.Itoa(year)))
				if occurrences <= 0 {
					continue
				}
				rowHasReport = true
				accumulator.years[year] = struct{}{}
				accumulator.incidents += occurrences
			}
			if rowHasReport {
				subdistrict := strings.TrimSpace(dpmCSVField(row, columns, "DISTRICT_CODE"))
				if subdistrict == "" {
					subdistrict = strings.TrimSpace(dpmCSVField(row, columns, "DISTRICT_NAME"))
				}
				if subdistrict != "" {
					accumulator.subdistricts[subdistrict] = struct{}{}
				}
			}
		}
	}

	areas := make(map[string]dpmFloodAreaSummary, len(accumulators))
	for key, accumulator := range accumulators {
		if len(accumulator.years) == 0 {
			continue
		}
		years := make([]int, 0, len(accumulator.years))
		for year := range accumulator.years {
			years = append(years, year)
		}
		// The source period is chronological; keep the serialized years stable.
		for i := 0; i < len(years); i++ {
			for j := i + 1; j < len(years); j++ {
				if years[j] < years[i] {
					years[i], years[j] = years[j], years[i]
				}
			}
		}
		areas[key] = dpmFloodAreaSummary{
			Province: accumulator.province, District: strings.TrimSpace(accumulator.district),
			ReportedYears: years, ReportedVillageIncidents: accumulator.incidents,
			AffectedSubdistrictCount: len(accumulator.subdistricts),
		}
	}
	return areas, nil
}

func normalizedDPMRowKey(row []string) string {
	fields := make([]string, len(row))
	for index, value := range row {
		fields[index] = strings.TrimSpace(value)
	}
	return strings.Join(fields, "\x1f")
}

func dpmCSVField(row []string, columns map[string]int, name string) string {
	index, found := columns[name]
	if !found || index < 0 || index >= len(row) {
		return ""
	}
	return row[index]
}

func dpmParseCount(raw string) int {
	raw = strings.TrimSpace(strings.ReplaceAll(raw, ",", ""))
	if raw == "" || raw == "-" {
		return 0
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil || value <= 0 {
		return 0
	}
	return int(math.Round(value))
}

func findDPMFloodArea(areas map[string]dpmFloodAreaSummary, province, district string) (dpmFloodAreaSummary, bool) {
	for _, districtVariant := range dpmAreaNameVariants(district) {
		if area, found := areas[dpmFloodAreaKey(province, districtVariant)]; found {
			return area, true
		}
	}
	return dpmFloodAreaSummary{}, false
}

func dpmAreaNameVariants(value string) []string {
	name := normalizeDPMAreaName(value)
	if name == "" {
		return nil
	}
	variants := []string{name}
	if strings.HasPrefix(name, "เมือง") && len([]rune(name)) > len([]rune("เมือง")) {
		variants = append(variants, strings.TrimPrefix(name, "เมือง"))
	}
	return variants
}

func normalizeDPMAreaName(value string) string {
	value = strings.TrimSpace(value)
	for _, prefix := range []string{"จังหวัด", "อำเภอ", "เขต", "กิ่งอำเภอ"} {
		value = strings.TrimPrefix(value, prefix)
	}
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) || r == '\u00a0' {
			return -1
		}
		return r
	}, strings.TrimSpace(value))
}

func dpmFloodAreaKey(province, district string) string {
	return normalizeDPMAreaName(province) + "|" + normalizeDPMAreaName(district)
}

func hashDPMFlood(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func (p *DPMFloodHistoryProvider) missing(assumption string) Observation {
	return Observation{
		MetricType: "flood", Status: domain.DataMissing,
		Source: domain.DataSource{
			Name: "Department of Disaster Prevention and Mitigation — village flood-history statistics",
			Type: "official_open_data", Authority: "official", GeographicScope: "district",
			ReferenceURI: dpmFloodHistoryReference, RetrievedAt: time.Now().UTC(), License: "Open Data Common",
		},
		Assumptions: []string{assumption},
	}
}

func (p *DPMFloodHistoryProvider) missingWithProvince(assumption, province string) Observation {
	observation := p.missing(assumption)
	if report := dpmProvinceReport(province); report != nil {
		observation.RawValue, _ = json.Marshal(struct {
			AssessmentType       string                    `json:"assessmentType"`
			LatestProvinceReport *dpmProvincialFloodReport `json:"latestProvinceReport"`
		}{"provincial_year_summary", report})
		observation.Source.ReferenceURI = "https://catalog.disaster.go.th/dataset/dpm-gd027"
		observation.Source.Name = "Department of Disaster Prevention and Mitigation — annual provincial flood statistics"
		observation.Source.GeographicScope = "province"
		observation.Source.DatasetVersion = "B.E. 2568; provincial context only"
		observation.Source.Methodology = "Provincial annual totals only; excluded from district-history scoring."
		if snapshot, err := loadDPMFloodSnapshot(); err == nil {
			observation.Source.RetrievedAt = snapshot.ImportedAt
		}
		observation.Assumptions = append(observation.Assumptions, "The 2025 provincial summary is additional context only and is excluded from district-history scoring. District counts in that source are totals, not named districts.")
	}
	return observation
}
