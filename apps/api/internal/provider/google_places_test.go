package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rbc/ev-station/apps/api/internal/cache"
	"github.com/rbc/ev-station/apps/api/internal/domain"
)

func TestGooglePlacesProviderUsesServerKeyAndMarksCappedResultsIncomplete(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests++
		if request.Method != http.MethodPost || request.Header.Get("X-Goog-Api-Key") != "server-key" {
			t.Fatalf("expected authenticated Google Places POST")
		}
		if request.Header.Get("X-Goog-FieldMask") != "places.id,places.displayName,places.primaryType,places.location" {
			t.Fatalf("unexpected field mask: %s", request.Header.Get("X-Goog-FieldMask"))
		}
		body := `{"places":[{"id":"place-` + strings.Repeat("x", requests) + `","displayName":{"text":"Place"},"primaryType":"restaurant","location":{"latitude":13.7,"longitude":100.5}}]}`
		if requests == 1 {
			places := make([]string, 0, googlePlacesMaxResults)
			for index := 0; index < googlePlacesMaxResults; index++ {
				places = append(places, `{"id":"capped-`+string(rune('a'+index))+`","displayName":{"text":"Place"},"primaryType":"restaurant","location":{"latitude":13.7,"longitude":100.5}}`)
			}
			body = `{"places":[` + strings.Join(places, ",") + `]}`
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(body))
	}))
	defer server.Close()
	latitude, longitude := 13.7, 100.5
	provider := NewGooglePlacesProvider(GooglePlacesConfig{APIKey: "server-key", Endpoint: server.URL, CacheTTL: time.Hour}, server.Client(), cache.Noop{})
	observations, err := provider.Collect(context.Background(), domain.Site{Latitude: &latitude, Longitude: &longitude}, 3000)
	if err != nil {
		t.Fatal(err)
	}
	poi := observationByType(observations, "poi")
	if poi.Status != domain.DataVerified {
		t.Fatalf("expected Google Places evidence, got %+v", poi)
	}
	var raw struct {
		CoverageComplete bool `json:"coverageComplete"`
	}
	if err := json.Unmarshal(poi.RawValue, &raw); err != nil || raw.CoverageComplete {
		t.Fatalf("capped Nearby Search must be excluded from scoring: raw=%s err=%v", poi.RawValue, err)
	}
}

func TestGooglePlacesProviderWithoutKeyDoesNotInventPOI(t *testing.T) {
	latitude, longitude := 13.7, 100.5
	provider := NewGooglePlacesProvider(GooglePlacesConfig{}, nil, cache.Noop{})
	observations, err := provider.Collect(context.Background(), domain.Site{Latitude: &latitude, Longitude: &longitude}, 3000)
	if err != nil {
		t.Fatal(err)
	}
	if poi := observationByType(observations, "poi"); poi.Status != domain.DataMissing {
		t.Fatalf("missing key must not invent POI evidence: %+v", poi)
	}
}

func observationByType(observations []Observation, metricType string) Observation {
	for _, observation := range observations {
		if observation.MetricType == metricType {
			return observation
		}
	}
	return Observation{}
}
