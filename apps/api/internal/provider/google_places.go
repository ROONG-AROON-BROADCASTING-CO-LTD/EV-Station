package provider

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/rbc/ev-station/apps/api/internal/cache"
	"github.com/rbc/ev-station/apps/api/internal/domain"
)

const googlePlacesMaxResults = 20

// GooglePlacesConfig deliberately accepts a server-only key. The browser key
// used for map display must never be sent to this provider.
type GooglePlacesConfig struct {
	APIKey    string
	Endpoint  string
	CacheTTL  time.Duration
	UserAgent string
}

type GooglePlacesProvider struct {
	config GooglePlacesConfig
	client *http.Client
	cache  cache.Cache
}

type googlePlacesResponse struct {
	Places []struct {
		ID          string `json:"id"`
		DisplayName struct {
			Text string `json:"text"`
		} `json:"displayName"`
		PrimaryType string `json:"primaryType"`
		Location    struct {
			Latitude  float64 `json:"latitude"`
			Longitude float64 `json:"longitude"`
		} `json:"location"`
	} `json:"places"`
}

type googlePlacesMetricValue struct {
	Count            int                 `json:"count"`
	RadiusMeters     int                 `json:"radiusMeters"`
	CoverageComplete bool                `json:"coverageComplete"`
	Places           []googlePlacesPlace `json:"places"`
	CategoryCounts   map[string]int      `json:"categoryCounts"`
}

type googlePlacesPlace struct {
	PlaceID   string  `json:"placeId"`
	Name      string  `json:"name"`
	Category  string  `json:"category"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

func NewGooglePlacesProvider(config GooglePlacesConfig, client *http.Client, externalCache cache.Cache) *GooglePlacesProvider {
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	if externalCache == nil {
		externalCache = cache.Noop{}
	}
	if config.Endpoint == "" {
		config.Endpoint = "https://places.googleapis.com/v1/places:searchNearby"
	}
	if config.CacheTTL <= 0 {
		config.CacheTTL = 24 * time.Hour
	}
	return &GooglePlacesProvider{config: config, client: client, cache: externalCache}
}

// Collect uses Nearby Search (New) for a small, explicit set of business
// relevance categories. Each category is requested separately because Google
// limits a nearby-search response to 20 places. A category that reaches that
// ceiling makes the total incomplete, so it is displayed but never scored.
func (p *GooglePlacesProvider) Collect(ctx context.Context, site domain.Site, radius int) ([]Observation, error) {
	observations, positions := unavailableObservations()
	if site.Latitude == nil || site.Longitude == nil {
		observations[positions["poi"]] = p.missing("Valid coordinates are required to query Google Places.")
		return observations, nil
	}
	if strings.TrimSpace(p.config.APIKey) == "" {
		observations[positions["poi"]] = p.missing("Google Places API is not configured. Set GOOGLE_MAPS_SERVER_API_KEY on the API server.")
		return observations, nil
	}
	if radius <= 0 {
		radius = 3000
	}

	// These are Place Types (New) selected for a charging-site context. They
	// are evidence categories, not a claim that every relevant business exists.
	types := []string{"gas_station", "restaurant", "cafe", "shopping_mall", "hospital", "tourist_attraction", "lodging", "university", "school", "market"}
	places := make([]googlePlacesPlace, 0)
	categoryCounts := make(map[string]int)
	seen := make(map[string]struct{})
	coverageComplete := true
	for _, placeType := range types {
		response, err := p.search(ctx, *site.Latitude, *site.Longitude, radius, placeType)
		if err != nil {
			observations[positions["poi"]] = p.missing("Google Places query was unavailable; the system retained other POI evidence and produced no Google Places score.")
			return observations, nil
		}
		if len(response.Places) >= googlePlacesMaxResults {
			coverageComplete = false
		}
		for _, item := range response.Places {
			if item.ID == "" {
				continue
			}
			if _, exists := seen[item.ID]; exists {
				continue
			}
			seen[item.ID] = struct{}{}
			category := item.PrimaryType
			if category == "" {
				category = placeType
			}
			categoryCounts[category]++
			places = append(places, googlePlacesPlace{PlaceID: item.ID, Name: item.DisplayName.Text, Category: category, Latitude: item.Location.Latitude, Longitude: item.Location.Longitude})
		}
	}

	raw, err := json.Marshal(googlePlacesMetricValue{Count: len(places), RadiusMeters: radius, CoverageComplete: coverageComplete, Places: places, CategoryCounts: categoryCounts})
	if err != nil {
		return observations, err
	}
	assumptions := []string{
		"Google Places results are commercial map evidence for the requested place categories, not an independent field verification.",
		"A category capped at 20 Nearby Search results is treated as incomplete and excluded from the POI score rather than undercounted.",
		"The provider supplies evidence only; deterministic preliminary-v1 scoring is applied separately by backend logic.",
	}
	if !coverageComplete {
		assumptions = append(assumptions, "At least one requested place category reached the Google Nearby Search result cap; the list remains visible but is not used for the POI score.")
	}
	observations[positions["poi"]] = Observation{MetricType: "poi", RawValue: raw, Status: domain.DataVerified, Source: domain.DataSource{
		Name: "Google Places API (New)", Type: "commercial_places_api", ReferenceURI: "https://developers.google.com/maps/documentation/places/web-service/nearby-search", RetrievedAt: time.Now().UTC(),
		Methodology: "Nearby Search (New), separately querying selected charging-site context types within the submitted radius; only a non-capped set is eligible for the deterministic POI score.",
	}, Assumptions: assumptions}
	return observations, nil
}

func (p *GooglePlacesProvider) search(ctx context.Context, latitude, longitude float64, radius int, placeType string) (googlePlacesResponse, error) {
	payload, err := json.Marshal(map[string]any{
		"includedTypes":       []string{placeType},
		"maxResultCount":      googlePlacesMaxResults,
		"rankPreference":      "DISTANCE",
		"languageCode":        "th",
		"regionCode":          "TH",
		"locationRestriction": map[string]any{"circle": map[string]any{"center": map[string]float64{"latitude": latitude, "longitude": longitude}, "radius": radius}},
	})
	if err != nil {
		return googlePlacesResponse{}, err
	}
	hash := sha256.Sum256(append([]byte(placeType+":"), payload...))
	cacheKey := "google-places:" + hex.EncodeToString(hash[:])
	if cached, found, cacheErr := p.cache.Get(ctx, cacheKey); cacheErr == nil && found {
		var result googlePlacesResponse
		if err := json.Unmarshal(cached, &result); err == nil {
			return result, nil
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.config.Endpoint, bytes.NewReader(payload))
	if err != nil {
		return googlePlacesResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Goog-Api-Key", p.config.APIKey)
	req.Header.Set("X-Goog-FieldMask", "places.id,places.displayName,places.primaryType,places.location")
	if p.config.UserAgent != "" {
		req.Header.Set("User-Agent", p.config.UserAgent)
	}
	response, err := p.client.Do(req)
	if err != nil {
		return googlePlacesResponse{}, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if err != nil {
		return googlePlacesResponse{}, err
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return googlePlacesResponse{}, fmt.Errorf("Google Places returned %d", response.StatusCode)
	}
	var result googlePlacesResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return googlePlacesResponse{}, err
	}
	_ = p.cache.Set(ctx, cacheKey, body, p.config.CacheTTL)
	return result, nil
}

func (p *GooglePlacesProvider) missing(assumption string) Observation {
	return Observation{MetricType: "poi", Status: domain.DataMissing, Source: domain.DataSource{Name: "Google Places API (New)", Type: "commercial_places_api", ReferenceURI: "https://developers.google.com/maps/documentation/places/web-service/nearby-search", RetrievedAt: time.Now().UTC()}, Assumptions: []string{assumption}}
}
