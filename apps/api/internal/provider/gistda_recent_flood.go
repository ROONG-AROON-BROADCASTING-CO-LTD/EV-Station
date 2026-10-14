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
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rbc/ev-station/apps/api/internal/cache"
	"github.com/rbc/ev-station/apps/api/internal/domain"
)

const gistdaDisasterReference = "https://data.go.th/dataset/flood-disaster-data"

// The Disaster API uses an API-Key header. A Sphere elevation key is a
// different credential and must never be silently reused here.
type GISTDARecentFloodConfig struct {
	BaseURL   string
	APIKey    string
	UserAgent string
	CacheTTL  time.Duration
}

type GISTDARecentFloodProvider struct {
	config GISTDARecentFloodConfig
	client *http.Client
	cache  cache.Cache
}

type recentFloodWindow struct {
	Days             int        `json:"days"`
	Status           string     `json:"status"`
	Reason           string     `json:"reason,omitempty"`
	FeatureCount     *int       `json:"featureCount,omitempty"`
	SiteOverlapCount *int       `json:"siteOverlapCount,omitempty"`
	Complete         bool       `json:"complete"`
	RetrievedAt      *time.Time `json:"retrievedAt,omitempty"`
	SourceURI        string     `json:"sourceUri"`
}

type recentFloodValue struct {
	AssessmentType  string              `json:"assessmentType"`
	SearchBounds    [4]float64          `json:"searchBounds"`
	Windows         []recentFloodWindow `json:"windows"`
	ScoringEligible bool                `json:"scoringEligible"`
}

func NewGISTDARecentFloodProvider(config GISTDARecentFloodConfig, client *http.Client, externalCache cache.Cache) *GISTDARecentFloodProvider {
	if client == nil {
		client = &http.Client{Timeout: 8 * time.Second}
	}
	if externalCache == nil {
		externalCache = cache.Noop{}
	}
	if config.BaseURL == "" {
		config.BaseURL = "https://api-gateway.gistda.or.th/api/2.0/resources"
	}
	if config.CacheTTL <= 0 {
		config.CacheTTL = time.Hour
	}
	return &GISTDARecentFloodProvider{config: config, client: client, cache: externalCache}
}

func (p *GISTDARecentFloodProvider) Collect(ctx context.Context, site domain.Site, radius int) ([]Observation, error) {
	value := recentFloodValue{AssessmentType: "gistda_recent_flood", Windows: make([]recentFloodWindow, 4)}
	validCoordinates := site.Latitude != nil && site.Longitude != nil && !math.IsNaN(*site.Latitude) && !math.IsNaN(*site.Longitude) && math.Abs(*site.Latitude) <= 90 && math.Abs(*site.Longitude) <= 180
	if radius <= 0 {
		radius = 3000
	}
	if validCoordinates {
		lat, lon := *site.Latitude, *site.Longitude
		dLat := float64(radius) / 111320
		dLon := dLat / math.Max(math.Cos(lat*math.Pi/180), 0.01)
		value.SearchBounds = [4]float64{lon - dLon, lat - dLat, lon + dLon, lat + dLat}
	}
	var wg sync.WaitGroup
	for i, days := range []int{1, 3, 7, 30} {
		path := "1day"
		if days != 1 {
			path = strconv.Itoa(days) + "days"
		}
		endpoint := strings.TrimRight(p.config.BaseURL, "/") + "/features/flood/" + path
		value.Windows[i] = recentFloodWindow{Days: days, Status: "missing", SourceURI: endpoint}
		if !validCoordinates {
			value.Windows[i].Reason = "invalid_coordinates"
			continue
		}
		if strings.TrimSpace(p.config.APIKey) == "" {
			value.Windows[i].Reason = "not_configured"
			continue
		}
		wg.Add(1)
		go func(index int, endpoint string) {
			defer wg.Done()
			window := p.fetch(ctx, endpoint, value.SearchBounds, *site.Longitude, *site.Latitude)
			window.Days = value.Windows[index].Days
			value.Windows[index] = window
		}(i, endpoint)
	}
	wg.Wait()
	status := domain.DataMissing
	for _, window := range value.Windows {
		if window.Status == "available" {
			status = domain.DataVerified
		}
	}
	raw, _ := json.Marshal(value)
	return []Observation{{MetricType: "flood_current", RawValue: raw, Status: status,
		Source: domain.DataSource{Name: "GISTDA Disaster Platform — recent observed flood polygons", Type: "api_key_gis", Authority: "GISTDA", GeographicScope: "search_bounds", ReferenceURI: gistdaDisasterReference, RetrievedAt: time.Now().UTC(),
			Methodology: "Queries 1/3/7/30-day polygon collections within the search bounding box and checks whether returned polygons contain the site pin. Empty results are not proof of flood-free conditions. Retrieval time is not satellite observation time.", License: "Licence not specified in the official dataset catalogue; context only, excluded from investment scoring."},
		Assumptions: []string{"Recent satellite collections are situational evidence, not a complete annual flood history or a forecast.", "Bounding-box matches do not establish that the frontage road or an actual access route flooded."}}}, nil
}

func (p *GISTDARecentFloodProvider) fetch(ctx context.Context, endpoint string, bounds [4]float64, lon, lat float64) recentFloodWindow {
	w := recentFloodWindow{Status: "missing", SourceURI: endpoint}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	parts := make([]string, 4)
	for i, v := range bounds {
		parts[i] = strconv.FormatFloat(v, 'f', 6, 64)
	}
	values := url.Values{"bbox": {strings.Join(parts, ",")}, "limit": {"1000"}, "offset": {"0"}}
	// Cache per credential scope without ever storing or logging the key itself.
	fingerprint := sha256.Sum256([]byte(p.config.APIKey + "|" + endpoint + "?" + values.Encode()))
	cacheKey := "gistda:recent-flood:v1:" + hex.EncodeToString(fingerprint[:])
	if raw, found, err := p.cache.Get(ctx, cacheKey); err == nil && found {
		var cached recentFloodWindow
		if json.Unmarshal(raw, &cached) == nil && cached.Status == "available" && cached.RetrievedAt != nil {
			return cached
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"?"+values.Encode(), nil)
	if err != nil {
		w.Reason = "invalid_endpoint"
		return w
	}
	req.Header.Set("API-Key", p.config.APIKey)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", p.config.UserAgent)
	resp, err := p.client.Do(req)
	if err != nil {
		w.Reason = "service_unavailable"
		return w
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		switch resp.StatusCode {
		case 401, 403:
			w.Reason = "unauthorized"
		case 429:
			w.Reason = "rate_limited"
		default:
			w.Reason = "service_unavailable"
		}
		return w
	}
	payload, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20+1))
	if err != nil || len(payload) > 8<<20 {
		w.Reason = "invalid_response"
		return w
	}
	count, overlap, complete, err := parseRecentFlood(payload, lon, lat)
	if err != nil {
		w.Reason = "invalid_response"
		return w
	}
	now := time.Now().UTC()
	w.Status = "available"
	w.FeatureCount = &count
	w.SiteOverlapCount = &overlap
	w.Complete = complete
	w.RetrievedAt = &now
	if !complete {
		w.Reason = "partial_results"
	}
	raw, _ := json.Marshal(w)
	_ = p.cache.Set(ctx, cacheKey, raw, p.config.CacheTTL)
	return w
}

// Reject arbitrary JSON/error objects and unsupported geometry. An HTTP 200
// response alone must never turn an upstream failure into "no flooding".
func parseRecentFlood(payload []byte, lon, lat float64) (int, int, bool, error) {
	var collection struct {
		Type     string `json:"type"`
		Features *[]struct {
			Type     string `json:"type"`
			Geometry struct {
				Type        string          `json:"type"`
				Coordinates json.RawMessage `json:"coordinates"`
			} `json:"geometry"`
		} `json:"features"`
		NumberMatched *int `json:"numberMatched"`
		Links         []struct {
			Rel string `json:"rel"`
		} `json:"links"`
	}
	if json.Unmarshal(payload, &collection) != nil || collection.Type != "FeatureCollection" || collection.Features == nil {
		return 0, 0, false, fmt.Errorf("invalid feature collection")
	}
	count, overlap := len(*collection.Features), 0
	for _, f := range *collection.Features {
		if f.Type != "Feature" {
			return 0, 0, false, fmt.Errorf("invalid feature")
		}
		var polygons [][][][]float64
		switch f.Geometry.Type {
		case "Polygon":
			var polygon [][][]float64
			if json.Unmarshal(f.Geometry.Coordinates, &polygon) != nil {
				return 0, 0, false, fmt.Errorf("invalid polygon")
			}
			polygons = append(polygons, polygon)
		case "MultiPolygon":
			if json.Unmarshal(f.Geometry.Coordinates, &polygons) != nil {
				return 0, 0, false, fmt.Errorf("invalid multipolygon")
			}
		default:
			return 0, 0, false, fmt.Errorf("unsupported geometry")
		}
		if len(polygons) == 0 {
			return 0, 0, false, fmt.Errorf("empty geometry")
		}
		contains := false
		for _, polygon := range polygons {
			if len(polygon) == 0 {
				return 0, 0, false, fmt.Errorf("empty polygon")
			}
			inside := false
			for i, ring := range polygon {
				if len(ring) < 4 {
					return 0, 0, false, fmt.Errorf("invalid ring")
				}
				for _, point := range ring {
					if len(point) < 2 || !validFloodPoint(point) {
						return 0, 0, false, fmt.Errorf("invalid coordinate")
					}
				}
				if ring[0][0] != ring[len(ring)-1][0] || ring[0][1] != ring[len(ring)-1][1] {
					return 0, 0, false, fmt.Errorf("unclosed ring")
				}
				if i == 0 {
					inside = floodRingContains(ring, lon, lat)
				} else if floodRingContains(ring, lon, lat) {
					inside = false
				}
			}
			contains = contains || inside
		}
		if contains {
			overlap++
		}
	}
	complete := count < 1000
	if collection.NumberMatched != nil {
		if *collection.NumberMatched < count {
			return 0, 0, false, fmt.Errorf("invalid matched count")
		}
		complete = *collection.NumberMatched == count
	}
	for _, link := range collection.Links {
		if link.Rel == "next" {
			complete = false
		}
	}
	return count, overlap, complete, nil
}

func validFloodPoint(p []float64) bool {
	return !math.IsNaN(p[0]) && !math.IsNaN(p[1]) && !math.IsInf(p[0], 0) && !math.IsInf(p[1], 0) && math.Abs(p[0]) <= 180 && math.Abs(p[1]) <= 90
}

func floodRingContains(ring [][]float64, x, y float64) bool {
	inside := false
	for i, j := 0, len(ring)-1; i < len(ring); j, i = i, i+1 {
		a, b := ring[i], ring[j]
		cross := (x-a[0])*(b[1]-a[1]) - (y-a[1])*(b[0]-a[0])
		if math.Abs(cross) < 1e-12 && x >= math.Min(a[0], b[0]) && x <= math.Max(a[0], b[0]) && y >= math.Min(a[1], b[1]) && y <= math.Max(a[1], b[1]) {
			return true
		}
		if (a[1] > y) != (b[1] > y) && x < (b[0]-a[0])*(y-a[1])/(b[1]-a[1])+a[0] {
			inside = !inside
		}
	}
	return inside
}
