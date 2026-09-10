package provider

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/rbc/ev-station/apps/api/internal/cache"
	"github.com/rbc/ev-station/apps/api/internal/domain"
)

const gistdaElevationReference = "https://api.sphere.gistda.or.th/services/api/"

type GISTDAElevationConfig struct {
	Endpoint  string
	APIKey    string
	CacheTTL  time.Duration
	UserAgent string
	// UsageRecorder is called only after a non-cached, successful provider
	// request. Recording must never make an analysis fail.
	UsageRecorder func(context.Context, string, int64)
}

type GISTDAElevationProvider struct {
	config GISTDAElevationConfig
	client *http.Client
	cache  cache.Cache
}

type gistdaElevationPoint struct {
	Location  float64 `json:"location"`
	Elevation float64 `json:"elevation"`
}

type gistdaElevationResponse struct {
	Data []gistdaElevationPoint `json:"data"`
}

// GISTDA's elevation endpoint accepts GeoJSON. A short north/south cross through
// the submitted coordinate samples the terrain without pretending that it is a
// cadastral boundary or an engineering survey.
type gistdaTerrainValue struct {
	AssessmentType    string  `json:"assessmentType"`
	SampleSpanMeters  int     `json:"sampleSpanMeters"`
	SampleCount       int     `json:"sampleCount"`
	MinimumElevationM float64 `json:"minimumElevationMeters"`
	MaximumElevationM float64 `json:"maximumElevationMeters"`
	ElevationRangeM   float64 `json:"elevationRangeMeters"`
}

func NewGISTDAElevationProvider(config GISTDAElevationConfig, client *http.Client, externalCache cache.Cache) *GISTDAElevationProvider {
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	if externalCache == nil {
		externalCache = cache.Noop{}
	}
	if config.Endpoint == "" {
		config.Endpoint = "https://api.sphere.gistda.or.th/services/geo/elevation"
	}
	if config.CacheTTL <= 0 {
		config.CacheTTL = 24 * time.Hour
	}
	return &GISTDAElevationProvider{config: config, client: client, cache: externalCache}
}

func (p *GISTDAElevationProvider) Collect(ctx context.Context, site domain.Site, _ int) ([]Observation, error) {
	if site.Latitude == nil || site.Longitude == nil {
		return []Observation{p.missing("ต้องมีพิกัดที่ถูกต้องก่อนจึงจะตรวจความต่างระดับภูมิประเทศได้")}, nil
	}
	if strings.TrimSpace(p.config.APIKey) == "" {
		return []Observation{p.missing("ยังไม่ได้ตั้งค่า GISTDA_API_KEY จึงยังไม่สามารถตรวจความต่างระดับภูมิประเทศอัตโนมัติได้")}, nil
	}

	value, err := p.fetch(ctx, *site.Latitude, *site.Longitude)
	if err != nil {
		return []Observation{p.missing("GISTDA Elevation API ไม่พร้อมใช้งาน จึงไม่มีการสรุปความต่างระดับภูมิประเทศ")}, nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return []Observation{p.missing("ไม่สามารถจัดเก็บผลความต่างระดับจาก GISTDA ได้")}, nil
	}
	return []Observation{{
		MetricType: "site_requirements", RawValue: raw, Status: domain.DataEstimated,
		Source: domain.DataSource{
			Name: "GISTDA Sphere Elevation API", Type: "api_key_gis", Authority: "GISTDA",
			GeographicScope: "point_sampling", ReferenceURI: gistdaElevationReference,
			RetrievedAt: time.Now().UTC(), Methodology: "Samples elevation along a 200-metre north/south line centred on the submitted coordinate.",
		},
		Assumptions: []string{
			"ผลนี้เป็นตัวอย่างค่าระดับภูมิประเทศจากจุดปักหมุด ไม่ใช่การรังวัดขอบแปลงหรือแบบปรับระดับพื้นที่.",
			"ความต่างระดับช่วยคัดกรองความเสี่ยงเท่านั้น ห้ามใช้คำนวณปริมาณดินหรือค่าใช้จ่ายก่อสร้าง.",
		},
	}}, nil
}

func (p *GISTDAElevationProvider) fetch(ctx context.Context, latitude, longitude float64) (gistdaTerrainValue, error) {
	const sampleSpanMeters = 200
	// One degree latitude is approximately 111,320 metres. The two end points
	// are 100m from the pin, making the total sampled span 200m.
	deltaLatitude := (sampleSpanMeters / 2.0) / 111320.0
	geo, err := json.Marshal(map[string]any{"type": "LineString", "coordinates": [][]float64{{longitude, latitude - deltaLatitude}, {longitude, latitude}, {longitude, latitude + deltaLatitude}}})
	if err != nil {
		return gistdaTerrainValue{}, err
	}
	values := url.Values{"key": {p.config.APIKey}, "geo": {string(geo)}}
	endpoint := strings.TrimRight(p.config.Endpoint, "?")
	cacheHash := sha256.Sum256([]byte(endpoint + "?" + values.Encode()))
	cacheKey := "gistda:elevation:" + hex.EncodeToString(cacheHash[:])
	if payload, found, cacheErr := p.cache.Get(ctx, cacheKey); cacheErr == nil && found {
		return parseGISTDAElevation(payload)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"?"+values.Encode(), nil)
	if err != nil {
		return gistdaTerrainValue{}, err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", p.config.UserAgent)
	response, err := p.client.Do(request)
	if err != nil {
		return gistdaTerrainValue{}, err
	}
	defer response.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return gistdaTerrainValue{}, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return gistdaTerrainValue{}, fmt.Errorf("gistda elevation returned status %d", response.StatusCode)
	}
	value, err := parseGISTDAElevation(payload)
	if err != nil {
		return gistdaTerrainValue{}, err
	}
	_ = p.cache.Set(ctx, cacheKey, payload, p.config.CacheTTL)
	if p.config.UsageRecorder != nil {
		p.config.UsageRecorder(ctx, "gistda-elevation", 1)
	}
	return value, nil
}

func parseGISTDAElevation(payload []byte) (gistdaTerrainValue, error) {
	var wrapped gistdaElevationResponse
	if err := json.Unmarshal(payload, &wrapped); err != nil {
		return gistdaTerrainValue{}, fmt.Errorf("invalid gistda elevation response: %w", err)
	}
	points := wrapped.Data
	if len(points) == 0 {
		// Some API deployments return the documented data array directly.
		if err := json.Unmarshal(payload, &points); err != nil {
			return gistdaTerrainValue{}, fmt.Errorf("gistda elevation response did not include data")
		}
	}
	if len(points) < 2 {
		return gistdaTerrainValue{}, fmt.Errorf("gistda elevation response did not include enough samples")
	}
	minimum, maximum := math.Inf(1), math.Inf(-1)
	for _, point := range points {
		if math.IsNaN(point.Elevation) || math.IsInf(point.Elevation, 0) {
			return gistdaTerrainValue{}, fmt.Errorf("gistda elevation response contains an invalid elevation")
		}
		minimum = math.Min(minimum, point.Elevation)
		maximum = math.Max(maximum, point.Elevation)
	}
	return gistdaTerrainValue{AssessmentType: "gistda_elevation_sampling", SampleSpanMeters: 200, SampleCount: len(points), MinimumElevationM: minimum, MaximumElevationM: maximum, ElevationRangeM: maximum - minimum}, nil
}

func (p *GISTDAElevationProvider) missing(assumption string) Observation {
	return Observation{MetricType: "site_requirements", Status: domain.DataMissing,
		Source:      domain.DataSource{Name: "GISTDA Sphere Elevation API", Type: "api_key_gis", Authority: "GISTDA", GeographicScope: "point_sampling", ReferenceURI: gistdaElevationReference, RetrievedAt: time.Now().UTC()},
		Assumptions: []string{assumption},
	}
}
