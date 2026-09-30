package provider

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/rbc/ev-station/apps/api/internal/cache"
	"github.com/rbc/ev-station/apps/api/internal/domain"
	"github.com/rbc/ev-station/apps/api/internal/telemetry"
)

const drrAADTReference = "https://datagov.mot.go.th/th/dataset/aadt1"

// DRRAADTProvider combines the official rural-road AADT CSV with the DRR
// national road geometry service. The CSV identifies routes rather than
// control sections, so ambiguous duplicate route codes are deliberately not
// guessed.
type DRRAADTConfig struct {
	CSVURL       string
	RoadLayerURL string
	DataYear     int
	CacheTTL     time.Duration
	UserAgent    string
}

type DRRAADTProvider struct {
	config DRRAADTConfig
	client *http.Client
	cache  cache.Cache
}

type drrAADTRecord struct {
	RoadCode  string
	RoadName  string
	TotalAADT int
}

type drrRoadResponse struct {
	Features []drrRoadFeature `json:"features"`
}

type drrRoadFeature struct {
	Attributes struct {
		RoadCode string `json:"road_code"`
		RoadName string `json:"route_name"`
	} `json:"attributes"`
	Geometry struct {
		Paths [][][]float64 `json:"paths"`
	} `json:"geometry"`
}

type drrAADTMetricValue struct {
	AADT                 int     `json:"aadt"`
	DataYear             int     `json:"dataYear"`
	RoadAuthority        string  `json:"roadAuthority"`
	RoadCode             string  `json:"roadCode"`
	RoadName             string  `json:"roadName"`
	DistanceToRoadMeters float64 `json:"distanceToRoadMeters"`
	MatchRadiusMeters    int     `json:"matchRadiusMeters"`
}

func NewDRRAADTProvider(config DRRAADTConfig, client *http.Client, externalCache cache.Cache) *DRRAADTProvider {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	if externalCache == nil {
		externalCache = cache.Noop{}
	}
	if config.CSVURL == "" {
		config.CSVURL = "https://dataportal.drr.go.th/dataset/3833590c-a0f4-4c99-baf1-8c540aadebb5/resource/0d612f16-e188-4604-b61c-c0da07854ef9/download/untitled.csv"
	}
	if config.RoadLayerURL == "" {
		config.RoadLayerURL = "https://gis.drr.go.th/arcgis/rest/services/DRR_Feature/FeatureServer/2/query"
	}
	if config.DataYear == 0 {
		config.DataYear = 2568
	}
	if config.CacheTTL <= 0 {
		config.CacheTTL = 24 * time.Hour
	}
	return &DRRAADTProvider{config: config, client: client, cache: externalCache}
}

func (p *DRRAADTProvider) Collect(ctx context.Context, site domain.Site, radius int) ([]Observation, error) {
	observations, positions := unavailableObservations()
	if site.Latitude == nil || site.Longitude == nil {
		observations[positions["traffic"]] = p.missing("Valid coordinates are required to match an official DRR AADT road.")
		return observations, nil
	}
	if radius <= 0 {
		radius = 3000
	}
	records, err := p.fetchAADT(ctx)
	if err != nil {
		observations[positions["traffic"]] = p.missing("The official DRR AADT dataset could not be retrieved; no traffic value was produced.")
		return observations, nil
	}
	roads, err := p.fetchNearbyRoads(ctx, *site.Latitude, *site.Longitude, radius)
	if err != nil {
		observations[positions["traffic"]] = p.missing("The official DRR road layer could not be queried; no AADT value was produced.")
		return observations, nil
	}
	var matched *drrAADTMetricValue
	for _, road := range roads {
		candidates := records[normalizeDRRCode(road.Attributes.RoadCode)]
		if len(candidates) != 1 {
			continue
		}
		distance := distanceToRoadMeters(*site.Latitude, *site.Longitude, road.Geometry.Paths)
		if math.IsInf(distance, 1) || distance > float64(radius) {
			continue
		}
		candidate := drrAADTMetricValue{AADT: candidates[0].TotalAADT, DataYear: p.config.DataYear, RoadAuthority: "DRR", RoadCode: road.Attributes.RoadCode, RoadName: candidates[0].RoadName, DistanceToRoadMeters: distance, MatchRadiusMeters: radius}
		if matched == nil || candidate.DistanceToRoadMeters < matched.DistanceToRoadMeters {
			matched = &candidate
		}
	}
	if matched == nil {
		observations[positions["traffic"]] = p.missing("No nearby official DRR road with an unambiguous AADT route match was found; the system did not infer a traffic count.")
		return observations, nil
	}
	raw, _ := json.Marshal(matched)
	observations[positions["traffic"]] = Observation{MetricType: "traffic", RawValue: raw, Status: domain.DataVerified, Source: domain.DataSource{
		Name: "Department of Rural Roads AADT 2568 + road network", Type: "official_open_data_and_gis", ReferenceURI: drrAADTReference, DatasetVersion: fmt.Sprintf("DRR AADT %d", p.config.DataYear), RetrievedAt: time.Now().UTC(), Methodology: "Nearest official DRR road geometry within the requested radius, matched to the annual AADT route code.",
	}, Assumptions: []string{
		"AADT is the Department of Rural Roads annual average daily traffic measurement for the matched rural-road route, not a live count at the site entrance.",
		"The result is used only when the DRR route code has one unambiguous AADT record; duplicate route-code records are not guessed.",
		"The provider supplies evidence only; deterministic preliminary-v1 scoring is applied separately by backend logic.",
	}}
	return observations, nil
}

func (p *DRRAADTProvider) fetchAADT(ctx context.Context) (map[string][]drrAADTRecord, error) {
	ctx = telemetry.WithOperation(ctx, "drr_aadt_download")
	key := "drr:aadt:" + hashText(p.config.CSVURL)
	if value, found, err := p.cache.Get(ctx, key); err == nil && found {
		return parseDRRAADT(value)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.config.CSVURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/csv")
	req.Header.Set("User-Agent", p.config.UserAgent)
	res, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("DRR AADT returned status %d", res.StatusCode)
	}
	payload, err := io.ReadAll(io.LimitReader(res.Body, 16<<20))
	if err != nil {
		return nil, err
	}
	if _, err = parseDRRAADT(payload); err != nil {
		return nil, err
	}
	_ = p.cache.Set(ctx, key, payload, p.config.CacheTTL)
	return parseDRRAADT(payload)
}

func (p *DRRAADTProvider) fetchNearbyRoads(ctx context.Context, latitude, longitude float64, radius int) ([]drrRoadFeature, error) {
	ctx = telemetry.WithOperation(ctx, "drr_road_lookup")
	return fetchTrafficRoads[drrRoadFeature](ctx, p.client, p.cache, p.config.RoadLayerURL, "road_code,route_name", "OBJECTID", p.config.UserAgent, p.config.CacheTTL, latitude, longitude, radius)
}

func parseDRRAADT(payload []byte) (map[string][]drrAADTRecord, error) {
	reader := csv.NewReader(strings.NewReader(strings.TrimPrefix(string(payload), "\ufeff")))
	reader.TrimLeadingSpace = true
	reader.FieldsPerRecord = -1
	rows, err := reader.ReadAll()
	if err != nil || len(rows) < 2 {
		return nil, fmt.Errorf("invalid DRR AADT CSV")
	}
	header := map[string]int{}
	for i, name := range rows[0] {
		header[strings.ToLower(strings.TrimSpace(name))] = i
	}
	for _, required := range []string{"road_code", "road_name", "sum_aadt"} {
		if _, ok := header[required]; !ok {
			return nil, fmt.Errorf("DRR AADT CSV missing %s", required)
		}
	}
	result := map[string][]drrAADTRecord{}
	for _, row := range rows[1:] {
		if len(row) <= header["sum_aadt"] || len(row) <= header["road_code"] {
			continue
		}
		code := normalizeDRRCode(row[header["road_code"]])
		total, parseErr := parseDOHNumber(row[header["sum_aadt"]])
		if code == "" || parseErr != nil {
			continue
		}
		name := strings.TrimSpace(row[header["road_name"]])
		result[code] = append(result[code], drrAADTRecord{RoadCode: code, RoadName: name, TotalAADT: total})
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("DRR AADT CSV contains no usable records")
	}
	return result, nil
}

func normalizeDRRCode(value string) string {
	return strings.ToUpper(strings.Join(strings.Fields(strings.TrimSpace(value)), ""))
}

func (p *DRRAADTProvider) missing(assumption string) Observation {
	return Observation{MetricType: "traffic", Status: domain.DataMissing, Source: domain.DataSource{Name: "Department of Rural Roads AADT and road network", Type: "official_open_data_and_gis", ReferenceURI: drrAADTReference, RetrievedAt: time.Now().UTC()}, Assumptions: []string{assumption}}
}
