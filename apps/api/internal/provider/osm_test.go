package provider

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rbc/ev-station/apps/api/internal/cache"
	"github.com/rbc/ev-station/apps/api/internal/domain"
	"github.com/rbc/ev-station/apps/api/internal/telemetry"
)

func TestOSMProviderSkipsCoolingEndpointAndProbesItAfterCooldown(t *testing.T) {
	var primaryCalls, fallbackCalls int
	primaryFails := true
	primary := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		primaryCalls++
		if primaryFails {
			writer.WriteHeader(http.StatusGatewayTimeout)
			return
		}
		_, _ = writer.Write([]byte(`{"elements":[]}`))
	}))
	defer primary.Close()
	fallback := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		fallbackCalls++
		_, _ = writer.Write([]byte(`{"elements":[]}`))
	}))
	defer fallback.Close()

	now := time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC)
	provider := NewOSMProvider(OSMConfig{Endpoint: primary.URL, FallbackEndpoints: []string{fallback.URL}, EndpointCooldown: time.Minute}, primary.Client(), cache.Noop{})
	provider.now = func() time.Time { return now }
	ctx := telemetry.WithOperation(context.Background(), "overpass_context")

	if _, err := provider.fetch(ctx, "first"); err != nil {
		t.Fatal(err)
	}
	if primaryCalls != 1 || fallbackCalls != 1 {
		t.Fatalf("first request should fall back after 504: primary=%d fallback=%d", primaryCalls, fallbackCalls)
	}
	if _, err := provider.fetch(ctx, "second"); err != nil {
		t.Fatal(err)
	}
	if primaryCalls != 1 || fallbackCalls != 2 {
		t.Fatalf("cooldown should skip primary: primary=%d fallback=%d", primaryCalls, fallbackCalls)
	}

	now = now.Add(time.Minute)
	primaryFails = false
	if _, err := provider.fetch(ctx, "third"); err != nil {
		t.Fatal(err)
	}
	if primaryCalls != 2 {
		t.Fatalf("expired cooldown should probe the primary again, got %d calls", primaryCalls)
	}
}

func TestOSMProviderKeepsAnEndpointAvailableWhenAllAreCooling(t *testing.T) {
	provider := NewOSMProvider(OSMConfig{Endpoint: "https://primary.example/api", FallbackEndpoints: []string{"https://fallback.example/api"}, EndpointCooldown: time.Minute}, nil, cache.Noop{})
	now := time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC)
	provider.now = func() time.Time { return now }
	for _, endpoint := range []string{"https://primary.example/api", "https://fallback.example/api"} {
		provider.recordEndpointFailure("context", endpoint)
	}
	candidates := provider.endpointCandidates("context", []string{provider.config.Endpoint, provider.config.FallbackEndpoints[0]})
	if len(candidates) != 2 {
		t.Fatalf("all cooling endpoints must remain available for recovery, got %d", len(candidates))
	}
	for _, candidate := range candidates {
		if candidate.health != "temporarily_unavailable" {
			t.Fatalf("expected cooling endpoint state, got %s for %s", candidate.health, candidate.endpoint)
		}
	}
}

func TestOSMProviderCollectsRoadsAndCompetitionWithoutDuplicatingGooglePOI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", request.Method)
		}
		if request.Header.Get("User-Agent") != "rbc-test" {
			t.Fatalf("unexpected user agent %q", request.Header.Get("User-Agent"))
		}
		query, _ := io.ReadAll(request.Body)
		if strings.Contains(string(query), "restaurant") || strings.Contains(string(query), "landuse") {
			t.Fatalf("Overpass must not request POI or land-use data covered by Google Places: %s", query)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"version":0.6,"generator":"Overpass API","elements":[
			{"type":"node","id":1,"lat":13.1,"lon":100.1,"tags":{"amenity":"restaurant","name":"A"}},
			{"type":"node","id":2,"lat":13.101,"lon":100.101,"tags":{"amenity":"charging_station","name":"B"}},
			{"type":"node","id":2,"lat":13.101,"lon":100.101,"tags":{"amenity":"charging_station","name":"B"}},
			{"type":"way","id":3,"tags":{"highway":"primary","name":"Main Road"},"geometry":[{"lat":13.1,"lon":100.1},{"lat":13.11,"lon":100.11}]}
		]}`))
	}))
	defer server.Close()

	latitude, longitude := 13.1, 100.1
	provider := NewOSMProvider(OSMConfig{Endpoint: server.URL, UserAgent: "rbc-test", CacheTTL: time.Minute}, server.Client(), cache.Noop{})
	observations, err := provider.Collect(context.Background(), domain.Site{Latitude: &latitude, Longitude: &longitude}, 3000)
	if err != nil {
		t.Fatal(err)
	}

	byType := make(map[string]Observation, len(observations))
	for _, observation := range observations {
		byType[observation.MetricType] = observation
	}
	if byType["poi"].Status != domain.DataMissing {
		t.Fatalf("OSM must leave commercial POI collection to Google Places, got %+v", byType["poi"])
	}
	competition := byType["competition"]
	if competition.Status != domain.DataVerified || competition.NormalizedScore != nil || !strings.Contains(string(competition.RawValue), `"count":1`) {
		t.Fatalf("expected one unscored OSM charging competitor, got %+v", competition)
	}
	if byType["road_accessibility"].Status != domain.DataPreliminary || byType["road_accessibility"].NormalizedScore != nil {
		t.Fatalf("road accessibility must be a preliminary road proxy without a score: %+v", byType["road_accessibility"])
	}
	if !strings.Contains(string(byType["road_accessibility"].RawValue), `"mappedMajorRoadCount":1`) {
		t.Fatalf("expected one mapped major road, got %s", byType["road_accessibility"].RawValue)
	}
	if byType["traffic"].Status != domain.DataEstimated || !strings.Contains(string(byType["traffic"].RawValue), `"assessmentType":"osm_road_traffic_potential"`) {
		t.Fatalf("expected an explicitly labelled road-based traffic estimate, got %+v", byType["traffic"])
	}
	if !strings.Contains(string(byType["traffic"].RawValue), `"nearestRoadName":"Main Road"`) || !strings.Contains(string(byType["traffic"].RawValue), `"nearestRoadClass":"primary"`) {
		t.Fatalf("expected the nearest mapped road identity, got %s", byType["traffic"].RawValue)
	}
}

func TestOSMProviderRequiresCoordinates(t *testing.T) {
	provider := NewOSMProvider(OSMConfig{}, nil, cache.Noop{})
	observations, err := provider.Collect(context.Background(), domain.Site{}, 3000)
	if err != nil {
		t.Fatal(err)
	}
	for _, observation := range observations {
		if observation.NormalizedScore != nil {
			t.Fatalf("metric %s unexpectedly received a score", observation.MetricType)
		}
	}
}

func TestOverpassStalledPrimaryFallsBackWithinBudget(t *testing.T) {
	stalled := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		<-r.Context().Done()
	}))
	defer stalled.Close()
	backup := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"elements":[]}`))
	}))
	defer backup.Close()
	p := NewOSMProvider(OSMConfig{Endpoint: stalled.URL, FallbackEndpoints: []string{backup.URL}, RequestBudget: time.Second, AttemptTimeout: 50 * time.Millisecond}, stalled.Client(), cache.Noop{})
	started := time.Now()
	if _, err := p.fetch(context.Background(), "test"); err != nil {
		t.Fatal(err)
	}
	if time.Since(started) >= time.Second {
		t.Fatal("fallback exceeded the combined request budget")
	}
}

func TestOverpassAllMirrorsShareOneDeadline(t *testing.T) {
	stalled := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		<-r.Context().Done()
	}))
	defer stalled.Close()
	p := NewOSMProvider(OSMConfig{Endpoint: stalled.URL, FallbackEndpoints: []string{stalled.URL + "/backup", stalled.URL + "/third"}, RequestBudget: 120 * time.Millisecond, AttemptTimeout: 80 * time.Millisecond}, stalled.Client(), cache.Noop{})
	started := time.Now()
	if _, err := p.fetch(context.Background(), "test"); err == nil {
		t.Fatal("stalled mirrors must report missing evidence")
	}
	if time.Since(started) > 500*time.Millisecond {
		t.Fatal("mirror timeouts multiplied instead of sharing a deadline")
	}
}

func TestOverpassRuntimeErrorFallsBackInsteadOfReportingZero(t *testing.T) {
	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"elements":[],"remark":"runtime error: Query timed out"}`))
	}))
	defer primary.Close()
	backup := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"elements":[{"type":"node","id":1}]}`))
	}))
	defer backup.Close()
	p := NewOSMProvider(OSMConfig{Endpoint: primary.URL, FallbackEndpoints: []string{backup.URL}}, primary.Client(), cache.Noop{})
	payload, err := p.fetch(context.Background(), "test")
	if err != nil || !strings.Contains(string(payload), `"id":1`) {
		t.Fatalf("expected usable backup evidence, got %s, %v", payload, err)
	}
}

func TestClassifyOSMChargingStationsIgnoresGooglePlacesCategories(t *testing.T) {
	elements := []osmElement{
		{Type: "way", ID: 1, Tags: map[string]string{"amenity": "hospital"}},
		{Type: "way", ID: 2, Tags: map[string]string{"building": "commercial"}},
		{Type: "way", ID: 3, Tags: map[string]string{"tourism": "hotel"}},
		{Type: "way", ID: 4, Tags: map[string]string{"building": "apartments"}},
		{Type: "way", ID: 5, Tags: map[string]string{"building": "dormitory"}},
		{Type: "way", ID: 6, Tags: map[string]string{"building": "condominium"}},
		{Type: "way", ID: 7, Tags: map[string]string{"tourism": "attraction"}},
		{Type: "way", ID: 8, Tags: map[string]string{"building": "office"}},
		{Type: "node", ID: 9, Tags: map[string]string{"amenity": "charging_station"}},
	}
	chargers := classifyOSMChargingStations(elements)
	if len(chargers) != 1 || chargers[0].Category != "amenity:charging_station" {
		t.Fatalf("expected only the charging station, got %+v", chargers)
	}
}
