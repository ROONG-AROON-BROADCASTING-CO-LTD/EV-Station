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
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rbc/ev-station/apps/api/internal/cache"
	"github.com/rbc/ev-station/apps/api/internal/domain"
	"github.com/rbc/ev-station/apps/api/internal/telemetry"
)

type OSMConfig struct {
	Endpoint          string
	FallbackEndpoints []string
	UserAgent         string
	CacheTTL          time.Duration
	EndpointCooldown  time.Duration
	RequestBudget     time.Duration
	AttemptTimeout    time.Duration
}

type OSMProvider struct {
	config OSMConfig
	client *http.Client
	cache  cache.Cache
	health *overpassHealthTracker
	now    func() time.Time
}

const defaultOverpassEndpointCooldown = 2 * time.Minute

type overpassEndpointState struct {
	consecutiveFailures int
	cooldownUntil       time.Time
	averageLatency      time.Duration
}

type overpassEndpointCandidate struct {
	endpoint           string
	health             string
	cooldownRemaining  time.Duration
	probeAfterCooldown bool
	averageLatency     time.Duration
}

// overpassHealthTracker is deliberately process-local. A restart begins with
// the configured endpoint order and lets fresh requests establish health again;
// endpoint incidents are transient and should not be persisted indefinitely.
type overpassHealthTracker struct {
	mu     sync.Mutex
	states map[string]overpassEndpointState
}

type osmResponse struct {
	Version   float64      `json:"version"`
	Generator string       `json:"generator"`
	Elements  []osmElement `json:"elements"`
}

type osmElement struct {
	Type     string            `json:"type"`
	ID       int64             `json:"id"`
	Lat      float64           `json:"lat,omitempty"`
	Lon      float64           `json:"lon,omitempty"`
	Center   *osmCenter        `json:"center,omitempty"`
	Geometry []osmCenter       `json:"geometry,omitempty"`
	Tags     map[string]string `json:"tags"`
}

type osmCenter struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

type osmPlace struct {
	OSMType   string  `json:"osmType"`
	OSMID     int64   `json:"osmId"`
	Name      string  `json:"name,omitempty"`
	Category  string  `json:"category"`
	Latitude  float64 `json:"latitude,omitempty"`
	Longitude float64 `json:"longitude,omitempty"`
}

type osmRoadMetricValue struct {
	RadiusMeters           int            `json:"radiusMeters"`
	MappedMajorRoadCount   int            `json:"mappedMajorRoadCount"`
	NearestMajorRoadMeters *float64       `json:"nearestMajorRoadMeters,omitempty"`
	NearestRoadName        string         `json:"nearestRoadName,omitempty"`
	NearestRoadRef         string         `json:"nearestRoadRef,omitempty"`
	NearestRoadClass       string         `json:"nearestRoadClass,omitempty"`
	RoadClassCounts        map[string]int `json:"roadClassCounts"`
}

// osmTrafficPotentialValue deliberately contains no vehicle count.  It is used
// only when an official AADT road match is unavailable, so the UI can still
// describe the traffic potential implied by mapped road classes without
// presenting a modelled value as an official count.
type osmTrafficPotentialValue struct {
	AssessmentType         string         `json:"assessmentType"`
	PotentialScore         float64        `json:"potentialScore"`
	RadiusMeters           int            `json:"radiusMeters"`
	MappedMajorRoadCount   int            `json:"mappedMajorRoadCount"`
	NearestMajorRoadMeters *float64       `json:"nearestMajorRoadMeters,omitempty"`
	NearestRoadName        string         `json:"nearestRoadName,omitempty"`
	NearestRoadRef         string         `json:"nearestRoadRef,omitempty"`
	NearestRoadClass       string         `json:"nearestRoadClass,omitempty"`
	RoadClassCounts        map[string]int `json:"roadClassCounts"`
}

func NewOSMProvider(config OSMConfig, client *http.Client, externalCache cache.Cache) *OSMProvider {
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	if externalCache == nil {
		externalCache = cache.Noop{}
	}
	if config.CacheTTL <= 0 {
		config.CacheTTL = 15 * time.Minute
	}
	if config.EndpointCooldown <= 0 {
		config.EndpointCooldown = defaultOverpassEndpointCooldown
	}
	if config.RequestBudget <= 0 {
		config.RequestBudget = 8 * time.Second
	}
	if config.AttemptTimeout <= 0 {
		config.AttemptTimeout = 4 * time.Second
	}
	return &OSMProvider{
		config: config, client: client, cache: externalCache,
		health: &overpassHealthTracker{states: make(map[string]overpassEndpointState)}, now: time.Now,
	}
}

func (p *OSMProvider) Collect(ctx context.Context, site domain.Site, radius int) ([]Observation, error) {
	observations, positions := unavailableObservations()
	if site.Latitude == nil || site.Longitude == nil {
		setMissingOSMObservation(observations, positions, "road_accessibility", "Valid coordinates are required to query OpenStreetMap road-accessibility data.")
		setMissingOSMObservation(observations, positions, "competition", "Valid coordinates are required to query OpenStreetMap charging stations.")
		return observations, nil
	}
	if radius <= 0 {
		radius = 3000
	}

	type queryResult struct {
		kind    string
		payload []byte
		err     error
	}
	results := make(chan queryResult, 2)
	var wait sync.WaitGroup
	for kind, query := range map[string]string{
		"context": buildOverpassContextQuery(*site.Latitude, *site.Longitude, radius),
		"roads":   buildOverpassRoadQuery(*site.Latitude, *site.Longitude, radius),
	} {
		wait.Add(1)
		go func(kind, query string) {
			defer wait.Done()
			payload, err := p.fetch(telemetry.WithOperation(ctx, "overpass_"+kind), query)
			results <- queryResult{kind: kind, payload: payload, err: err}
		}(kind, query)
	}
	wait.Wait()
	close(results)

	retrievedAt := time.Now().UTC()
	source := domain.DataSource{
		Name:         "OpenStreetMap via Overpass API",
		Type:         "open_data_api",
		ReferenceURI: "https://www.openstreetmap.org/copyright",
		RetrievedAt:  retrievedAt,
		Methodology:  "Count of currently returned OpenStreetMap elements within the requested radius, deduplicated by OSM type and ID.",
		License:      "Open Data Commons Open Database License (ODbL) 1.0",
	}

	for result := range results {
		if result.err != nil {
			if result.kind == "roads" {
				setMissingOSMObservation(observations, positions, "road_accessibility", "OpenStreetMap road query was unavailable; no road-accessibility value or score was produced.")
			} else {
				setMissingOSMObservation(observations, positions, "competition", "OpenStreetMap charging-station query was unavailable; no competitor value or score was produced.")
			}
			continue
		}
		var response osmResponse
		if err := json.Unmarshal(result.payload, &response); err != nil {
			if result.kind == "roads" {
				setMissingOSMObservation(observations, positions, "road_accessibility", "OpenStreetMap returned an unreadable road response; no road-accessibility value or score was produced.")
			} else {
				setMissingOSMObservation(observations, positions, "competition", "OpenStreetMap returned an unreadable charging-station response; no competitor value or score was produced.")
			}
			continue
		}
		if result.kind == "roads" {
			roadSource := source
			roadSource.Methodology = "Count mapped motorway, trunk, primary, secondary and tertiary ways within the radius and approximate the nearest distance from returned way geometry vertices."
			roads := classifyOSMRoads(response.Elements)
			roadObservation := osmRoadObservation(roads, *site.Latitude, *site.Longitude, radius, roadSource)
			observations[positions["road_accessibility"]] = roadObservation
			observations[positions["traffic"]] = osmTrafficPotentialObservation(roadObservation, roadSource)
			continue
		}
		chargers := classifyOSMChargingStations(response.Elements)
		competitionRadius := 1000
		nearbyChargers := make([]osmPlace, 0, len(chargers))
		for _, charger := range chargers {
			if haversineMeters(*site.Latitude, *site.Longitude, charger.Latitude, charger.Longitude) <= float64(competitionRadius) {
				nearbyChargers = append(nearbyChargers, charger)
			}
		}
		competition := osmCompetitionObservation(nearbyChargers, competitionRadius, source, []string{
			"Only charging stations mapped in OpenStreetMap are included; unmapped operators may be missing.",
			"Competition is counted only within 1 kilometre of the submitted coordinates.",
			"Connector availability, power, pricing, and operational status are not verified by this query.",
			"The provider supplies evidence only; deterministic preliminary-v1 scoring is applied separately by backend logic.",
		})
		// OSM can provide evidence of mapped competitors, but an empty OSM
		// result cannot establish that a province has no competitors.
		if len(nearbyChargers) == 0 {
			competition.Status = domain.DataPreliminary
		}
		observations[positions["competition"]] = competition
	}
	return observations, nil
}

func osmTrafficPotentialObservation(road Observation, source domain.DataSource) Observation {
	var value osmRoadMetricValue
	if json.Unmarshal(road.RawValue, &value) != nil || value.MappedMajorRoadCount == 0 || value.NearestMajorRoadMeters == nil {
		return Observation{MetricType: "traffic", Status: domain.DataMissing, Source: source, Assumptions: []string{"No official AADT match was found and mapped major-road evidence was insufficient to estimate traffic potential."}}
	}
	base := 0.0
	for roadClass, score := range map[string]float64{"motorway": 80, "trunk": 70, "primary": 60, "secondary": 45, "tertiary": 30} {
		if value.RoadClassCounts[roadClass] > 0 && score > base {
			base = score
		}
	}
	proximity := 0.0
	switch {
	case *value.NearestMajorRoadMeters <= 100:
		proximity = 15
	case *value.NearestMajorRoadMeters <= 500:
		proximity = 10
	case *value.NearestMajorRoadMeters <= 1000:
		proximity = 5
	}
	connectivity := math.Min(5, math.Log1p(float64(value.MappedMajorRoadCount)))
	potential := math.Min(100, base+proximity+connectivity)
	raw, _ := json.Marshal(osmTrafficPotentialValue{AssessmentType: "osm_road_traffic_potential", PotentialScore: potential, RadiusMeters: value.RadiusMeters, MappedMajorRoadCount: value.MappedMajorRoadCount, NearestMajorRoadMeters: value.NearestMajorRoadMeters, NearestRoadName: value.NearestRoadName, NearestRoadRef: value.NearestRoadRef, NearestRoadClass: value.NearestRoadClass, RoadClassCounts: value.RoadClassCounts})
	source.Name = "OpenStreetMap road network — traffic potential estimate"
	source.Methodology = "Fallback when no official AADT match is available: road-class potential (motorway 80, trunk 70, primary 60, secondary 45, tertiary 30), plus proximity (up to 15) and mapped-network connectivity (up to 5). It never estimates vehicles per day."
	return Observation{MetricType: "traffic", RawValue: raw, Status: domain.DataEstimated, Source: source, Assumptions: []string{
		"Road-based traffic potential is shown when usable official counts are unavailable. Review source results for coverage or retrieval failures.",
		"Road classes and geometry come from OpenStreetMap and can be incomplete or differently classified from official Thai roads.",
		"The provider supplies evidence only; deterministic preliminary screening uses this estimated potential only when official AADT is unavailable.",
	}}
}

func (p *OSMProvider) fetch(ctx context.Context, query string) ([]byte, error) {
	// The budget includes all mirrors, cache I/O and response-body reads.
	// Sequential retries must never multiply the user's waiting time.
	ctx, cancelBudget := context.WithTimeout(ctx, p.config.RequestBudget)
	defer cancelBudget()
	hash := sha256.Sum256([]byte(query))
	cacheKey := "osm:overpass:" + hex.EncodeToString(hash[:])
	if value, found, err := p.cache.Get(ctx, cacheKey); err == nil && found {
		return value, nil
	}

	form := url.Values{"data": {query}}
	operation := strings.TrimPrefix(telemetry.Operation(ctx), "overpass_")
	if operation == "" {
		operation = "unknown"
	}
	endpoints := append([]string{p.config.Endpoint}, p.config.FallbackEndpoints...)
	candidates := p.endpointCandidates(operation, endpoints)
	var lastErr error
	for index, candidate := range candidates {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		endpoint := candidate.endpoint
		started := p.now().UTC()
		fallbackReason := ""
		if index > 0 {
			fallbackReason = "previous_endpoint_failed"
		}
		telemetry.Record(ctx, telemetry.Event{
			Provider: "osm", Operation: "overpass_endpoint_attempt", Endpoint: telemetry.Endpoint(endpoint),
			SelectedEndpoint: telemetry.Endpoint(endpoint), EndpointHealth: candidate.health,
			FallbackTriggered: index > 0, FallbackReason: fallbackReason,
			CooldownRemainingMS: candidate.cooldownRemaining.Milliseconds(), StartedAt: started, FinishedAt: started,
			Status: "started",
		})
		attemptCtx, cancelAttempt := context.WithTimeout(ctx, p.config.AttemptTimeout)
		req, err := http.NewRequestWithContext(attemptCtx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
		if err != nil {
			cancelAttempt()
			lastErr = err
			p.recordEndpointFailure(operation, endpoint)
			p.recordEndpointResult(ctx, endpoint, candidate, index > 0, fallbackReason, started, 0, err, 0)
			continue
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", p.config.UserAgent)
		response, err := p.client.Do(req)
		if err != nil {
			cancelAttempt()
			lastErr = err
			p.recordEndpointFailure(operation, endpoint)
			p.recordEndpointResult(ctx, endpoint, candidate, index > 0, fallbackReason, started, p.now().UTC().Sub(started), err, 0)
			continue
		}
		payload, readErr := io.ReadAll(io.LimitReader(response.Body, 10<<20))
		response.Body.Close()
		cancelAttempt()
		if readErr != nil {
			lastErr = readErr
			p.recordEndpointFailure(operation, endpoint)
			p.recordEndpointResult(ctx, endpoint, candidate, index > 0, fallbackReason, started, p.now().UTC().Sub(started), readErr, response.StatusCode)
			continue
		}
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			lastErr = fmt.Errorf("overpass returned status %d", response.StatusCode)
			if shouldCooldownOverpassStatus(response.StatusCode) {
				p.recordEndpointFailure(operation, endpoint)
			}
			p.recordEndpointResult(ctx, endpoint, candidate, index > 0, fallbackReason, started, p.now().UTC().Sub(started), lastErr, response.StatusCode)
			continue
		}
		// Overpass can return HTTP 200 with a runtime-error remark. Never cache
		// that response as successful evidence (or as zero competitors).
		var valid struct {
			Elements json.RawMessage `json:"elements"`
			Remark   string          `json:"remark"`
		}
		if json.Unmarshal(payload, &valid) != nil || len(valid.Elements) == 0 || valid.Elements[0] != '[' || valid.Remark != "" {
			lastErr = fmt.Errorf("overpass returned incomplete or invalid evidence")
			p.recordEndpointFailure(operation, endpoint)
			p.recordEndpointResult(ctx, endpoint, candidate, index > 0, fallbackReason, started, p.now().UTC().Sub(started), lastErr, response.StatusCode)
			continue
		}
		p.recordEndpointSuccess(operation, endpoint, p.now().UTC().Sub(started))
		p.recordEndpointResult(ctx, endpoint, candidate, index > 0, fallbackReason, started, p.now().UTC().Sub(started), nil, response.StatusCode)
		if err := p.cache.Set(ctx, cacheKey, payload, p.config.CacheTTL); err != nil {
			// Cache failures must not turn a successful public-data lookup into a missing observation.
			telemetry.Record(ctx, telemetry.Event{Provider: "osm", Operation: "overpass_cache_set", StartedAt: p.now().UTC(), FinishedAt: p.now().UTC(), Status: "error", ErrorType: telemetry.ErrorType(err)})
		}
		return payload, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no Overpass endpoint configured")
	}
	return nil, lastErr
}

func shouldCooldownOverpassStatus(status int) bool {
	return status == http.StatusTooManyRequests || status == http.StatusBadGateway || status == http.StatusServiceUnavailable || status == http.StatusGatewayTimeout
}

func (p *OSMProvider) endpointCandidates(operation string, configured []string) []overpassEndpointCandidate {
	now := p.now().UTC()
	p.health.mu.Lock()
	defer p.health.mu.Unlock()

	probes, healthy, cooling := make([]overpassEndpointCandidate, 0, len(configured)), make([]overpassEndpointCandidate, 0, len(configured)), make([]overpassEndpointCandidate, 0, len(configured))
	for _, rawEndpoint := range configured {
		endpoint := strings.TrimSpace(rawEndpoint)
		if endpoint == "" {
			continue
		}
		key := operation + ":" + telemetry.Endpoint(endpoint)
		state := p.health.states[key]
		candidate := overpassEndpointCandidate{endpoint: endpoint, health: "healthy", averageLatency: state.averageLatency}
		if state.cooldownUntil.After(now) {
			candidate.health = "temporarily_unavailable"
			candidate.cooldownRemaining = state.cooldownUntil.Sub(now)
			cooling = append(cooling, candidate)
			continue
		}
		if state.consecutiveFailures > 0 {
			candidate.health = "probe"
			candidate.probeAfterCooldown = true
			probes = append(probes, candidate)
			continue
		}
		healthy = append(healthy, candidate)
	}
	if len(probes) > 0 {
		return append(append(probes, healthy...), cooling...)
	}
	if len(healthy) > 0 {
		// Preserve the configured order until both endpoints have real measurements.
		// Afterwards prefer the quicker healthy endpoint without hard-coding a provider.
		sort.SliceStable(healthy, func(i, j int) bool {
			if healthy[i].averageLatency == 0 || healthy[j].averageLatency == 0 {
				return false
			}
			return healthy[i].averageLatency < healthy[j].averageLatency
		})
		return append(healthy, cooling...)
	}
	// Do not make an analysis fail solely because all known endpoints are cooling down.
	// Probe the one that recovers soonest, then retain the other endpoints as fallbacks.
	if len(cooling) > 0 {
		sort.SliceStable(cooling, func(i, j int) bool {
			return cooling[i].cooldownRemaining < cooling[j].cooldownRemaining
		})
		return cooling
	}
	return nil
}

func (p *OSMProvider) recordEndpointFailure(operation, endpoint string) {
	p.health.mu.Lock()
	defer p.health.mu.Unlock()
	key := operation + ":" + telemetry.Endpoint(endpoint)
	state := p.health.states[key]
	state.consecutiveFailures++
	state.cooldownUntil = p.now().UTC().Add(p.config.EndpointCooldown)
	p.health.states[key] = state
}

func (p *OSMProvider) recordEndpointSuccess(operation, endpoint string, latency time.Duration) {
	p.health.mu.Lock()
	defer p.health.mu.Unlock()
	key := operation + ":" + telemetry.Endpoint(endpoint)
	state := p.health.states[key]
	state.consecutiveFailures = 0
	state.cooldownUntil = time.Time{}
	if state.averageLatency == 0 {
		state.averageLatency = latency
	} else {
		state.averageLatency = (state.averageLatency + latency) / 2
	}
	p.health.states[key] = state
}

func (p *OSMProvider) recordEndpointResult(ctx context.Context, endpoint string, candidate overpassEndpointCandidate, fallback bool, fallbackReason string, started time.Time, duration time.Duration, err error, httpStatus int) {
	finished := started.Add(duration)
	status := "success"
	if err != nil || httpStatus < 200 || httpStatus >= 300 {
		status = "error"
	}
	errorType := telemetry.ErrorType(err)
	if errorType == "" && status == "error" {
		errorType = "http_status"
	}
	telemetry.Record(ctx, telemetry.Event{
		Provider: "osm", Operation: "overpass_endpoint_result", Endpoint: telemetry.Endpoint(endpoint),
		SelectedEndpoint: telemetry.Endpoint(endpoint), EndpointHealth: candidate.health,
		FallbackTriggered: fallback, FallbackReason: fallbackReason, CooldownRemainingMS: candidate.cooldownRemaining.Milliseconds(),
		StartedAt: started, FinishedAt: finished, DurationMS: duration.Milliseconds(), Status: status,
		ErrorType: errorType, HTTPStatus: httpStatus,
	})
}

func buildOverpassContextQuery(latitude, longitude float64, radius int) string {
	lat := strconv.FormatFloat(latitude, 'f', 6, 64)
	lon := strconv.FormatFloat(longitude, 'f', 6, 64)
	r := strconv.Itoa(radius)
	return `[out:json][timeout:20];(` +
		`nwr(around:` + r + `,` + lat + `,` + lon + `)["amenity"="charging_station"];` +
		`);out center tags;`
}

func buildOverpassRoadQuery(latitude, longitude float64, radius int) string {
	lat := strconv.FormatFloat(latitude, 'f', 6, 64)
	lon := strconv.FormatFloat(longitude, 'f', 6, 64)
	r := strconv.Itoa(radius)
	return `[out:json][timeout:20];way(around:` + r + `,` + lat + `,` + lon + `)["highway"~"^(motorway|trunk|primary|secondary|tertiary)$"];out geom tags;`
}

func classifyOSMChargingStations(elements []osmElement) []osmPlace {
	chargers := make([]osmPlace, 0)
	seen := make(map[string]struct{}, len(elements))
	for _, element := range elements {
		if element.Tags["amenity"] != "charging_station" {
			continue
		}
		key := element.Type + ":" + strconv.FormatInt(element.ID, 10)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		place := osmPlace{OSMType: element.Type, OSMID: element.ID, Name: element.Tags["name"], Category: "amenity:charging_station", Latitude: element.Lat, Longitude: element.Lon}
		if element.Center != nil {
			place.Latitude, place.Longitude = element.Center.Lat, element.Center.Lon
		}
		chargers = append(chargers, place)
	}
	return chargers
}

func classifyOSMRoads(elements []osmElement) []osmElement {
	roads := make([]osmElement, 0)
	seen := make(map[int64]struct{})
	for _, element := range elements {
		if element.Type != "way" || element.Tags["highway"] == "" {
			continue
		}
		if _, exists := seen[element.ID]; exists {
			continue
		}
		seen[element.ID] = struct{}{}
		roads = append(roads, element)
	}
	return roads
}

func osmRoadObservation(roads []osmElement, latitude, longitude float64, radius int, source domain.DataSource) Observation {
	classCounts := make(map[string]int)
	var nearest *float64
	var nearestRoad osmElement
	for _, road := range roads {
		classCounts[road.Tags["highway"]]++
		for _, point := range road.Geometry {
			distance := haversineMeters(latitude, longitude, point.Lat, point.Lon)
			if nearest == nil || distance < *nearest {
				value := distance
				nearest = &value
				nearestRoad = road
			}
		}
	}
	raw, _ := json.Marshal(osmRoadMetricValue{RadiusMeters: radius, MappedMajorRoadCount: len(roads), NearestMajorRoadMeters: nearest, NearestRoadName: firstNonEmpty(nearestRoad.Tags["name:th"], nearestRoad.Tags["name"]), NearestRoadRef: nearestRoad.Tags["ref"], NearestRoadClass: nearestRoad.Tags["highway"], RoadClassCounts: classCounts})
	return Observation{
		MetricType: "road_accessibility", RawValue: raw, Status: domain.DataPreliminary, Source: source,
		Assumptions: []string{
			"This is a road accessibility proxy from mapped road classes, not a traffic count, speed, congestion, or AADT measurement.",
			"Nearest-road distance is approximated from the returned OpenStreetMap way geometry vertices.",
			"OpenStreetMap road classification and coverage may be incomplete or differ from official Thai classifications.",
			"The provider supplies evidence only; deterministic preliminary-v1 scoring is applied separately by backend logic.",
		},
	}
}

func haversineMeters(latitude1, longitude1, latitude2, longitude2 float64) float64 {
	const earthRadius = 6371000.0
	lat1 := latitude1 * math.Pi / 180
	lat2 := latitude2 * math.Pi / 180
	deltaLat := (latitude2 - latitude1) * math.Pi / 180
	deltaLon := (longitude2 - longitude1) * math.Pi / 180
	a := math.Sin(deltaLat/2)*math.Sin(deltaLat/2) + math.Cos(lat1)*math.Cos(lat2)*math.Sin(deltaLon/2)*math.Sin(deltaLon/2)
	return earthRadius * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

func osmCompetitionObservation(places []osmPlace, radius int, source domain.DataSource, assumptions []string) Observation {
	raw, _ := json.Marshal(struct {
		Count        int        `json:"count"`
		RadiusMeters int        `json:"radiusMeters"`
		Places       []osmPlace `json:"places"`
	}{Count: len(places), RadiusMeters: radius, Places: places})
	return Observation{MetricType: "competition", RawValue: raw, Status: domain.DataVerified, Source: source, Assumptions: assumptions}
}

func unavailableObservations() ([]Observation, map[string]int) {
	base, _ := (UnavailableProvider{}).Collect(context.Background(), domain.Site{}, 0)
	positions := make(map[string]int, len(base))
	for index := range base {
		positions[base[index].MetricType] = index
	}
	return base, positions
}

func setMissingOSMObservation(observations []Observation, positions map[string]int, metricType, assumption string) {
	observations[positions[metricType]] = Observation{
		MetricType: metricType,
		Status:     domain.DataMissing,
		Source: domain.DataSource{
			Name:        "OpenStreetMap via Overpass API",
			Type:        "open_data_api",
			RetrievedAt: time.Now().UTC(),
			License:     "Open Data Commons Open Database License (ODbL) 1.0",
		},
		Assumptions: []string{assumption},
	}
}
