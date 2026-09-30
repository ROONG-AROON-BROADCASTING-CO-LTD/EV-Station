package provider

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rbc/ev-station/apps/api/internal/cache"
	"github.com/rbc/ev-station/apps/api/internal/domain"
)

var ErrInvalidGeocodingQuery = errors.New("geocoding query must contain between 3 and 200 characters")

type Geocoder interface {
	Search(context.Context, string, int) ([]GeocodingResult, error)
}

// ReverseGeocoder is deliberately separate from Geocoder so callers that only
// need address search (including tests and alternate providers) do not have to
// implement reverse lookup.  A reverse lookup is used only to offer an
// editable, human-readable site name after a customer has confirmed a pin.
type ReverseGeocoder interface {
	Reverse(context.Context, float64, float64) (ReverseGeocodingResult, error)
}

type NominatimConfig struct {
	Endpoint     string
	UserAgent    string
	CountryCodes string
	CacheTTL     time.Duration
}

type NominatimGeocoder struct {
	config      NominatimConfig
	client      *http.Client
	cache       cache.Cache
	requestMu   sync.Mutex
	nextRequest time.Time
}

type GeocodingResult struct {
	DisplayName string            `json:"displayName"`
	Latitude    float64           `json:"latitude"`
	Longitude   float64           `json:"longitude"`
	Category    string            `json:"category,omitempty"`
	PlaceType   string            `json:"placeType,omitempty"`
	Status      domain.DataStatus `json:"status"`
	Source      domain.DataSource `json:"source"`
	Assumptions []string          `json:"assumptions"`
}

type ReverseGeocodingResult struct {
	Road     string `json:"road,omitempty"`
	District string `json:"district,omitempty"`
	Province string `json:"province,omitempty"`
}

type nominatimResult struct {
	DisplayName string `json:"display_name"`
	Latitude    string `json:"lat"`
	Longitude   string `json:"lon"`
	Category    string `json:"category"`
	PlaceType   string `json:"type"`
}

type nominatimReverseResult struct {
	Address map[string]string `json:"address"`
}

func NewNominatimGeocoder(config NominatimConfig, client *http.Client, externalCache cache.Cache) *NominatimGeocoder {
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	if externalCache == nil {
		externalCache = cache.Noop{}
	}
	if config.Endpoint == "" {
		config.Endpoint = "https://nominatim.openstreetmap.org/search"
	}
	if config.CacheTTL <= 0 {
		config.CacheTTL = 30 * 24 * time.Hour
	}
	return &NominatimGeocoder{config: config, client: client, cache: externalCache}
}

func (g *NominatimGeocoder) Search(ctx context.Context, query string, limit int) ([]GeocodingResult, error) {
	query = strings.TrimSpace(query)
	if len([]rune(query)) < 3 || len([]rune(query)) > 200 {
		return nil, ErrInvalidGeocodingQuery
	}
	if limit < 1 || limit > 5 {
		limit = 5
	}

	cacheKey := nominatimCacheKey(query, limit, g.config.CountryCodes)
	if payload, found, err := g.cache.Get(ctx, cacheKey); err == nil && found {
		var cached []GeocodingResult
		if json.Unmarshal(payload, &cached) == nil {
			return cached, nil
		}
	}

	g.waitForPublicRateLimit(ctx)
	requestURL, err := url.Parse(g.config.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("invalid Nominatim endpoint: %w", err)
	}
	params := requestURL.Query()
	params.Set("q", query)
	params.Set("format", "jsonv2")
	params.Set("limit", strconv.Itoa(limit))
	params.Set("addressdetails", "1")
	params.Set("accept-language", "th,en")
	if g.config.CountryCodes != "" {
		params.Set("countrycodes", g.config.CountryCodes)
	}
	requestURL.RawQuery = params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", g.config.UserAgent)

	response, err := g.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		return nil, fmt.Errorf("Nominatim returned status %d", response.StatusCode)
	}
	payload, err := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	var raw []nominatimResult
	if err = json.Unmarshal(payload, &raw); err != nil {
		return nil, fmt.Errorf("decode Nominatim response: %w", err)
	}

	now := time.Now().UTC()
	results := make([]GeocodingResult, 0, len(raw))
	for _, item := range raw {
		latitude, latErr := strconv.ParseFloat(item.Latitude, 64)
		longitude, lonErr := strconv.ParseFloat(item.Longitude, 64)
		if latErr != nil || lonErr != nil {
			continue
		}
		results = append(results, GeocodingResult{
			DisplayName: item.DisplayName,
			Latitude:    latitude,
			Longitude:   longitude,
			Category:    item.Category,
			PlaceType:   item.PlaceType,
			Status:      domain.DataPreliminary,
			Source: domain.DataSource{
				Name:         "OpenStreetMap Nominatim",
				Type:         "open_data_api",
				ReferenceURI: "https://operations.osmfoundation.org/policies/nominatim/",
				RetrievedAt:  now,
				Methodology:  "User-triggered forward geocoding result returned by Nominatim; no autocomplete or bulk geocoding is performed.",
				License:      "Open Data Commons Open Database License (ODbL) 1.0",
			},
			Assumptions: []string{
				"The match is preliminary and must be confirmed by the user before site analysis.",
				"OpenStreetMap address coverage may be incomplete or imprecise.",
			},
		})
	}
	if cachedPayload, marshalErr := json.Marshal(results); marshalErr == nil {
		_ = g.cache.Set(ctx, cacheKey, cachedPayload, g.config.CacheTTL)
	}
	return results, nil
}

// Reverse finds the nearest mapped address context for a confirmed pin. It is
// not used to validate ownership or a legal parcel boundary; it only supplies
// a practical default name such as "พื้นที่เสนอ · ทล.212 · เมืองบึงกาฬ".
func (g *NominatimGeocoder) Reverse(ctx context.Context, latitude, longitude float64) (ReverseGeocodingResult, error) {
	if latitude < -90 || latitude > 90 || longitude < -180 || longitude > 180 {
		return ReverseGeocodingResult{}, errors.New("coordinates are out of range")
	}
	cacheKey := nominatimReverseCacheKey(latitude, longitude, g.config.CountryCodes)
	if payload, found, err := g.cache.Get(ctx, cacheKey); err == nil && found {
		var cached ReverseGeocodingResult
		if json.Unmarshal(payload, &cached) == nil {
			return cached, nil
		}
	}

	g.waitForPublicRateLimit(ctx)
	endpoint, err := reverseEndpoint(g.config.Endpoint)
	if err != nil {
		return ReverseGeocodingResult{}, err
	}
	params := endpoint.Query()
	params.Set("lat", strconv.FormatFloat(latitude, 'f', 7, 64))
	params.Set("lon", strconv.FormatFloat(longitude, 'f', 7, 64))
	params.Set("format", "jsonv2")
	params.Set("addressdetails", "1")
	params.Set("accept-language", "th,en")
	if g.config.CountryCodes != "" {
		params.Set("countrycodes", g.config.CountryCodes)
	}
	endpoint.RawQuery = params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return ReverseGeocodingResult{}, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", g.config.UserAgent)
	response, err := g.client.Do(req)
	if err != nil {
		return ReverseGeocodingResult{}, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		return ReverseGeocodingResult{}, fmt.Errorf("Nominatim reverse returned status %d", response.StatusCode)
	}
	payload, err := io.ReadAll(io.LimitReader(response.Body, 256<<10))
	if err != nil {
		return ReverseGeocodingResult{}, err
	}
	var raw nominatimReverseResult
	if err := json.Unmarshal(payload, &raw); err != nil {
		return ReverseGeocodingResult{}, fmt.Errorf("decode Nominatim reverse response: %w", err)
	}
	result := ReverseGeocodingResult{
		Road: firstAddress(raw.Address, "road", "pedestrian", "footway"),
		// In Thailand city_district often identifies a tambon, while county
		// identifies the amphoe required by district-level official datasets.
		District: firstAddress(raw.Address, "county", "district", "city_district", "municipality", "city", "town", "village"),
		Province: firstAddress(raw.Address, "state", "province"),
	}
	if cachedPayload, marshalErr := json.Marshal(result); marshalErr == nil {
		_ = g.cache.Set(ctx, cacheKey, cachedPayload, g.config.CacheTTL)
	}
	return result, nil
}

func reverseEndpoint(raw string) (*url.URL, error) {
	endpoint, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid Nominatim endpoint: %w", err)
	}
	endpoint.Path = strings.TrimSuffix(endpoint.Path, "/search") + "/reverse"
	return endpoint, nil
}

func firstAddress(address map[string]string, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(address[key]); value != "" {
			return value
		}
	}
	return ""
}

func (g *NominatimGeocoder) waitForPublicRateLimit(ctx context.Context) {
	g.requestMu.Lock()
	defer g.requestMu.Unlock()
	if delay := time.Until(g.nextRequest); delay > 0 {
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
	}
	g.nextRequest = time.Now().Add(time.Second)
}

func nominatimCacheKey(query string, limit int, countryCodes string) string {
	value := strings.ToLower(strings.TrimSpace(query)) + "|" + strconv.Itoa(limit) + "|" + countryCodes
	hash := sha256.Sum256([]byte(value))
	return "nominatim:search:" + hex.EncodeToString(hash[:])
}

func nominatimReverseCacheKey(latitude, longitude float64, countryCodes string) string {
	value := strconv.FormatFloat(latitude, 'f', 5, 64) + "|" + strconv.FormatFloat(longitude, 'f', 5, 64) + "|" + countryCodes
	hash := sha256.Sum256([]byte(value))
	// Older cached results could select a subdistrict instead of the county.
	return "nominatim:reverse:v2:" + hex.EncodeToString(hash[:])
}
