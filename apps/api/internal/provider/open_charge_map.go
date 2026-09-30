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
	"strconv"
	"strings"
	"time"

	"github.com/rbc/ev-station/apps/api/internal/cache"
	"github.com/rbc/ev-station/apps/api/internal/domain"
	"github.com/rbc/ev-station/apps/api/internal/telemetry"
)

type OpenChargeMapConfig struct {
	APIKey    string
	Endpoint  string
	CacheTTL  time.Duration
	UserAgent string
}

type OpenChargeMapProvider struct {
	config OpenChargeMapConfig
	client *http.Client
	cache  cache.Cache
}

type openChargeMapRecord struct {
	ID          int64 `json:"ID"`
	AddressInfo struct {
		Title     string  `json:"Title"`
		Latitude  float64 `json:"Latitude"`
		Longitude float64 `json:"Longitude"`
	} `json:"AddressInfo"`
	OperatorInfo *struct {
		Title string `json:"Title"`
	} `json:"OperatorInfo"`
}

func NewOpenChargeMapProvider(config OpenChargeMapConfig, client *http.Client, externalCache cache.Cache) *OpenChargeMapProvider {
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	if externalCache == nil {
		externalCache = cache.Noop{}
	}
	if config.Endpoint == "" {
		config.Endpoint = "https://api.openchargemap.io/v3/poi/"
	}
	if config.CacheTTL <= 0 {
		config.CacheTTL = 24 * time.Hour
	}
	return &OpenChargeMapProvider{config: config, client: client, cache: externalCache}
}

func (p *OpenChargeMapProvider) Collect(ctx context.Context, site domain.Site, _ int) ([]Observation, error) {
	ctx = telemetry.WithOperation(ctx, "open_charge_map_lookup")
	observations, positions := unavailableObservations()
	if site.Latitude == nil || site.Longitude == nil {
		observations[positions["competition"]] = p.missing("Valid coordinates are required to query Open Charge Map.")
		return observations, nil
	}
	if strings.TrimSpace(p.config.APIKey) == "" {
		observations[positions["competition"]] = p.missing("Open Charge Map is not configured. Set OPEN_CHARGE_MAP_API_KEY on the API server.")
		return observations, nil
	}
	const competitionRadiusMeters = 1000
	records, err := p.search(ctx, *site.Latitude, *site.Longitude, competitionRadiusMeters)
	if err != nil {
		observations[positions["competition"]] = p.missing("Open Charge Map was unavailable; the system retained other competition evidence and did not infer a value.")
		return observations, nil
	}
	places := make([]competitionPlace, 0, len(records))
	for _, record := range records {
		if record.ID <= 0 || record.AddressInfo.Latitude == 0 || record.AddressInfo.Longitude == 0 {
			continue
		}
		operator := ""
		if record.OperatorInfo != nil {
			operator = strings.TrimSpace(record.OperatorInfo.Title)
		}
		places = append(places, competitionPlace{RecordType: "open_charge_map", RecordID: "ocm:" + strconv.FormatInt(record.ID, 10), Name: strings.TrimSpace(record.AddressInfo.Title), Category: "charging_station", Latitude: record.AddressInfo.Latitude, Longitude: record.AddressInfo.Longitude, Operator: operator, SourceNames: []string{"Open Charge Map"}, SourceRecordCount: 1})
	}
	places = deduplicateCompetitionPlaces(places)
	value := competitionMetricValue{Count: len(places), RadiusMeters: competitionRadiusMeters, CoverageMatched: true, Places: places, Sources: []competitionSourceBreakdown{{Name: "Open Charge Map", ReferenceURI: "https://www.openchargemap.org/develop/api", Coverage: "Community-maintained global charging-location database", Count: len(places), RetrievedAt: time.Now().UTC()}}}
	raw, _ := json.Marshal(value)
	observations[positions["competition"]] = Observation{MetricType: "competition", RawValue: raw, Status: domain.DataPreliminary, Source: domain.DataSource{Name: "Open Charge Map", Type: "community_charging_api", ReferenceURI: "https://www.openchargemap.org/develop/api", RetrievedAt: time.Now().UTC(), Methodology: "Search charging locations within 1 kilometre of the submitted coordinate, then deterministically deduplicate records with other sources."}, Assumptions: []string{
		"Open Charge Map is community-maintained supplemental evidence; coverage and operational status can be incomplete.",
		"A zero result does not prove there are no competitors. It is merged with OpenStreetMap and available provincial data before deterministic scoring.",
		"The provider supplies evidence only; deterministic preliminary-v1 scoring is applied separately by backend logic.",
	}}
	return observations, nil
}

func (p *OpenChargeMapProvider) search(ctx context.Context, latitude, longitude float64, radiusMeters int) ([]openChargeMapRecord, error) {
	params := url.Values{}
	params.Set("output", "json")
	params.Set("latitude", strconv.FormatFloat(latitude, 'f', 6, 64))
	params.Set("longitude", strconv.FormatFloat(longitude, 'f', 6, 64))
	params.Set("distance", strconv.FormatFloat(float64(radiusMeters)/1000, 'f', 3, 64))
	params.Set("distanceunit", "KM")
	params.Set("maxresults", "100")
	params.Set("compact", "true")
	// Open Charge Map authenticates public API requests with the `key` query
	// parameter. Keep the key out of logs; the cache key below is a SHA-256
	// digest rather than the raw request URL.
	params.Set("key", p.config.APIKey)
	hash := sha256.Sum256([]byte(params.Encode()))
	cacheKey := "open-charge-map:" + hex.EncodeToString(hash[:])
	if cached, found, cacheErr := p.cache.Get(ctx, cacheKey); cacheErr == nil && found {
		var records []openChargeMapRecord
		if err := json.Unmarshal(cached, &records); err == nil {
			return records, nil
		}
	}
	endpoint := strings.TrimRight(p.config.Endpoint, "?")
	separator := "?"
	if strings.Contains(endpoint, "?") {
		separator = "&"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+separator+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-API-Key", p.config.APIKey)
	if p.config.UserAgent != "" {
		req.Header.Set("User-Agent", p.config.UserAgent)
	}
	response, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 5<<20))
	if err != nil {
		return nil, err
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("Open Charge Map returned %d", response.StatusCode)
	}
	var records []openChargeMapRecord
	if err := json.Unmarshal(body, &records); err != nil {
		return nil, err
	}
	_ = p.cache.Set(ctx, cacheKey, body, p.config.CacheTTL)
	return records, nil
}

func (p *OpenChargeMapProvider) missing(assumption string) Observation {
	return Observation{MetricType: "competition", Status: domain.DataMissing, Source: domain.DataSource{Name: "Open Charge Map", Type: "community_charging_api", ReferenceURI: "https://www.openchargemap.org/develop/api", RetrievedAt: time.Now().UTC()}, Assumptions: []string{assumption}}
}
