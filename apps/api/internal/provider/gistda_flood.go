package provider

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/rbc/ev-station/apps/api/internal/cache"
	"github.com/rbc/ev-station/apps/api/internal/domain"
	"github.com/rbc/ev-station/apps/api/internal/telemetry"
)

const gistdaFloodRiskReference = "https://gistdaportal.gistda.or.th/arcgis/rest/services/app/GISTDA_flood/MapServer/1"
const gistdaFloodRecurrenceReference = "https://gistdaportal.gistda.or.th/data/rest/services/FL_Flood/flood_freq11_20/MapServer/0"

type GISTDAFloodConfig struct {
	Endpoint           string
	RecurrenceEndpoint string
	CacheTTL           time.Duration
	UserAgent          string
}

type GISTDAFloodProvider struct {
	config GISTDAFloodConfig
	client *http.Client
	cache  cache.Cache
}

type gistdaFloodQueryResponse struct {
	Count *int `json:"count"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

type gistdaFloodRecurrenceResponse struct {
	ExceededTransferLimit bool `json:"exceededTransferLimit"`
	Features              []struct {
		Attributes map[string]any `json:"attributes"`
	} `json:"features"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

type gistdaFloodMetricValue struct {
	MappedFloodRiskAreaCount int    `json:"mappedFloodRiskAreaCount"`
	RadiusMeters             int    `json:"radiusMeters"`
	LayerName                string `json:"layerName"`
	RecurrencePolygonCount   int    `json:"recurrencePolygonCount,omitempty"`
	RecurrenceYears          []int  `json:"recurrenceYears,omitempty"`
	RecurrencePeriodStart    int    `json:"recurrencePeriodStartYear,omitempty"`
	RecurrencePeriodEnd      int    `json:"recurrencePeriodEndYear,omitempty"`
	RecurrenceAvailable      bool   `json:"recurrenceAvailable"`
}

func NewGISTDAFloodProvider(config GISTDAFloodConfig, client *http.Client, externalCache cache.Cache) *GISTDAFloodProvider {
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	if externalCache == nil {
		externalCache = cache.Noop{}
	}
	if config.Endpoint == "" {
		config.Endpoint = gistdaFloodRiskReference + "/query"
	}
	if config.CacheTTL <= 0 {
		config.CacheTTL = 24 * time.Hour
	}
	if config.RecurrenceEndpoint == "" {
		config.RecurrenceEndpoint = gistdaFloodRecurrenceReference + "/query"
	}
	return &GISTDAFloodProvider{config: config, client: client, cache: externalCache}
}

func (p *GISTDAFloodProvider) Collect(ctx context.Context, site domain.Site, radius int) ([]Observation, error) {
	observations, positions := unavailableObservations()
	if site.Latitude == nil || site.Longitude == nil {
		observations[positions["flood"]] = p.missing("Valid coordinates are required to query the published GISTDA flood-risk layer.")
		return observations, nil
	}
	if radius <= 0 {
		radius = 3000
	}

	count, err := p.fetchCount(ctx, *site.Latitude, *site.Longitude, radius)
	if err != nil {
		observations[positions["flood"]] = p.missing("GISTDA flood-risk layer was unavailable; no flood-risk conclusion was produced.")
		return observations, nil
	}
	recurrenceCount, recurrenceYears, recurrenceErr := p.fetchRecurrence(ctx, *site.Latitude, *site.Longitude, radius)

	raw, _ := json.Marshal(gistdaFloodMetricValue{
		MappedFloodRiskAreaCount: count,
		RadiusMeters:             radius,
		LayerName:                "พื้นที่เสี่ยงภัยน้ำท่วม",
		RecurrencePolygonCount:   recurrenceCount,
		RecurrenceYears:          recurrenceYears,
		RecurrencePeriodStart:    2011,
		RecurrencePeriodEnd:      2020,
		RecurrenceAvailable:      recurrenceErr == nil,
	})
	observations[positions["flood"]] = Observation{
		MetricType: "flood",
		RawValue:   raw,
		Status:     domain.DataVerified,
		Source: domain.DataSource{
			Name:         "GISTDA flood-risk and historical-recurrence layers (ArcGIS REST)",
			Type:         "open_gis_api",
			ReferenceURI: gistdaFloodRecurrenceReference,
			RetrievedAt:  time.Now().UTC(),
			Methodology:  "Queries published flood-risk polygons and historical flood-recurrence polygons within the requested radius. The recurrence layer covers 2011–2020; flood years are read from its annual attributes.",
			License:      "Public GISTDA ArcGIS service; service metadata does not specify reuse terms. Confirm reuse conditions before external redistribution.",
		},
		Assumptions: []string{
			"This is a spatial overlap with GISTDA's published flood-risk layer, not a flood forecast, water-depth estimate, loss estimate, or suitability score.",
			"No mapped flood-risk area within the requested radius does not prove that the site has no flood risk.",
			"Layer coverage, update schedule and classification must be checked with GISTDA before an investment decision.",
			"The provider supplies evidence only; deterministic preliminary-v4 scoring is applied separately by backend logic.",
		},
	}
	if recurrenceErr != nil {
		observations[positions["flood"]].Assumptions = append(observations[positions["flood"]].Assumptions, "GISTDA historical-recurrence query was unavailable; only the separately published flood-risk layer is shown.")
	}
	return observations, nil
}

func (p *GISTDAFloodProvider) fetchRecurrence(ctx context.Context, latitude, longitude float64, radius int) (int, []int, error) {
	ctx = telemetry.WithOperation(ctx, "gistda_flood_recurrence_lookup")
	values := url.Values{
		"f": {"json"}, "where": {"1=1"},
		"geometry":     {strconv.FormatFloat(longitude, 'f', 6, 64) + "," + strconv.FormatFloat(latitude, 'f', 6, 64)},
		"geometryType": {"esriGeometryPoint"}, "inSR": {"4326"}, "spatialRel": {"esriSpatialRelIntersects"},
		"distance": {strconv.Itoa(radius)}, "units": {"esriSRUnit_Meter"},
		"outFields":      {"Freq,Y_2011,Y_2012,Y_2013,Y_2014,Y_2015,Y_2016,Y2017,Y_2018,Y_2019,Y_2020"},
		"returnGeometry": {"false"}, "resultRecordCount": {"1000"},
	}
	endpoint := p.config.RecurrenceEndpoint
	cacheKeyHash := sha256.Sum256([]byte(endpoint + "?" + values.Encode()))
	cacheKey := "gistda:flood-recurrence:" + hex.EncodeToString(cacheKeyHash[:])
	payload, found, cacheErr := p.cache.Get(ctx, cacheKey)
	if cacheErr != nil || !found {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"?"+values.Encode(), nil)
		if err != nil {
			return 0, nil, err
		}
		request.Header.Set("Accept", "application/json")
		request.Header.Set("User-Agent", p.config.UserAgent)
		response, err := p.client.Do(request)
		if err != nil {
			return 0, nil, err
		}
		defer response.Body.Close()
		payload, err = io.ReadAll(io.LimitReader(response.Body, 4<<20))
		if err != nil {
			return 0, nil, err
		}
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			return 0, nil, fmt.Errorf("GISTDA recurrence layer returned status %d", response.StatusCode)
		}
	}
	var result gistdaFloodRecurrenceResponse
	if err := json.Unmarshal(payload, &result); err != nil {
		return 0, nil, err
	}
	if result.Error != nil {
		return 0, nil, fmt.Errorf("GISTDA recurrence query failed (%d): %s", result.Error.Code, result.Error.Message)
	}
	if result.Features == nil || result.ExceededTransferLimit {
		return 0, nil, fmt.Errorf("GISTDA recurrence collection missing or truncated")
	}
	_ = p.cache.Set(ctx, cacheKey, payload, p.config.CacheTTL)
	years := make(map[int]bool)
	for _, feature := range result.Features {
		for year := 2011; year <= 2020; year++ {
			key := fmt.Sprintf("Y_%d", year)
			if year == 2017 {
				key = "Y2017"
			}
			value, ok := feature.Attributes[key]
			if !ok {
				continue
			}
			if number, ok := value.(float64); ok && number > 0 {
				years[year] = true
			}
		}
	}
	yearList := make([]int, 0, len(years))
	for year := range years {
		yearList = append(yearList, year)
	}
	sort.Ints(yearList)
	return len(result.Features), yearList, nil
}

func (p *GISTDAFloodProvider) fetchCount(ctx context.Context, latitude, longitude float64, radius int) (int, error) {
	ctx = telemetry.WithOperation(ctx, "gistda_flood_lookup")
	values := url.Values{
		"f":               {"json"},
		"where":           {"1=1"},
		"geometry":        {strconv.FormatFloat(longitude, 'f', 6, 64) + "," + strconv.FormatFloat(latitude, 'f', 6, 64)},
		"geometryType":    {"esriGeometryPoint"},
		"inSR":            {"4326"},
		"spatialRel":      {"esriSpatialRelIntersects"},
		"distance":        {strconv.Itoa(radius)},
		"units":           {"esriSRUnit_Meter"},
		"returnCountOnly": {"true"},
		"returnGeometry":  {"false"},
	}
	endpoint := strings.TrimRight(p.config.Endpoint, "?")
	cacheKeyHash := sha256.Sum256([]byte(endpoint + "?" + values.Encode()))
	cacheKey := "gistda:flood-risk:" + hex.EncodeToString(cacheKeyHash[:])
	if payload, found, cacheErr := p.cache.Get(ctx, cacheKey); cacheErr == nil && found {
		return parseGISTDAFloodCount(payload)
	}

	separator := "?"
	if strings.Contains(endpoint, "?") {
		separator = "&"
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+separator+values.Encode(), nil)
	if err != nil {
		return 0, err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", p.config.UserAgent)
	response, err := p.client.Do(request)
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return 0, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return 0, fmt.Errorf("gistda flood layer returned status %d", response.StatusCode)
	}
	count, err := parseGISTDAFloodCount(payload)
	if err != nil {
		return 0, err
	}
	_ = p.cache.Set(ctx, cacheKey, payload, p.config.CacheTTL)
	return count, nil
}

func parseGISTDAFloodCount(payload []byte) (int, error) {
	var response gistdaFloodQueryResponse
	if err := json.Unmarshal(payload, &response); err != nil {
		return 0, fmt.Errorf("invalid gistda flood response: %w", err)
	}
	if response.Error != nil {
		return 0, fmt.Errorf("gistda flood query failed (%d): %s", response.Error.Code, response.Error.Message)
	}
	if response.Count == nil || *response.Count < 0 {
		return 0, fmt.Errorf("gistda flood response did not include a valid count")
	}
	return *response.Count, nil
}

func (p *GISTDAFloodProvider) missing(assumption string) Observation {
	return Observation{
		MetricType: "flood", Status: domain.DataMissing,
		Source: domain.DataSource{
			Name: "GISTDA Flood Risk Layer (ArcGIS REST)", Type: "open_gis_api",
			ReferenceURI: gistdaFloodRiskReference, RetrievedAt: time.Now().UTC(),
		},
		Assumptions: []string{assumption},
	}
}
