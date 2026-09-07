package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rbc/ev-station/apps/api/internal/cache"
	"github.com/rbc/ev-station/apps/api/internal/domain"
)

func TestOSMProviderCollectsPOIAndCompetitionWithoutInventingScores(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", request.Method)
		}
		if request.Header.Get("User-Agent") != "rbc-test" {
			t.Fatalf("unexpected user agent %q", request.Header.Get("User-Agent"))
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
	for _, metricType := range []string{"poi", "competition"} {
		observation := byType[metricType]
		if observation.Status != domain.DataVerified {
			t.Fatalf("expected %s to be verified, got %s", metricType, observation.Status)
		}
		if observation.NormalizedScore != nil {
			t.Fatalf("expected %s score to remain absent", metricType)
		}
		if !strings.Contains(string(observation.RawValue), `"count":1`) {
			t.Fatalf("expected deduplicated count for %s, got %s", metricType, observation.RawValue)
		}
	}
	if byType["road_accessibility"].Status != domain.DataPreliminary || byType["road_accessibility"].NormalizedScore != nil {
		t.Fatalf("road accessibility must be a preliminary road proxy without a score: %+v", byType["road_accessibility"])
	}
	if !strings.Contains(string(byType["road_accessibility"].RawValue), `"mappedMajorRoadCount":1`) {
		t.Fatalf("expected one mapped major road, got %s", byType["road_accessibility"].RawValue)
	}
	poiRaw := string(byType["poi"].RawValue)
	if !strings.Contains(poiRaw, `"categoryCounts":{"amenity:restaurant":1}`) {
		t.Fatalf("expected exact POI category totals, got %s", poiRaw)
	}
	if !strings.Contains(poiRaw, `"excludedChargingStationCount":1`) {
		t.Fatalf("expected charging station to be excluded from POI total, got %s", poiRaw)
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

func TestClassifyOSMElementsSeparatesRequestedBuildingCategories(t *testing.T) {
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
	places, chargers, _ := classifyOSMElements(elements)
	counts := make(map[string]int)
	for _, place := range places {
		counts[place.Category]++
	}
	for _, category := range []string{"amenity:hospital", "building:commercial", "tourism:hotel", "building:apartments", "building:dormitory", "building:condominium", "tourism:attraction", "building:office"} {
		if counts[category] != 1 {
			t.Fatalf("expected one %s, got %d (%+v)", category, counts[category], counts)
		}
	}
	if len(chargers) != 1 || len(places) != 8 {
		t.Fatalf("expected EV station to be separated from 8 POIs: places=%d chargers=%d", len(places), len(chargers))
	}
}
