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

func TestGISTDAElevationProviderReturnsTerrainEvidenceWithoutScore(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Query().Get("key") != "test-key" || !strings.Contains(request.URL.Query().Get("geo"), "LineString") {
			t.Fatalf("expected keyed GeoJSON request, got %s", request.URL.RawQuery)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"data":[{"location":0,"elevation":12.5},{"location":0.5,"elevation":14.0},{"location":1,"elevation":13.0}]}`))
	}))
	defer server.Close()
	latitude, longitude := 13.7, 100.5
	provider := NewGISTDAElevationProvider(GISTDAElevationConfig{Endpoint: server.URL, APIKey: "test-key", CacheTTL: time.Hour, UserAgent: "rbc-test"}, server.Client(), cache.Noop{})
	observations, err := provider.Collect(context.Background(), domain.Site{Latitude: &latitude, Longitude: &longitude}, 3000)
	if err != nil {
		t.Fatal(err)
	}
	if len(observations) != 1 || observations[0].MetricType != "site_requirements" || observations[0].Status != domain.DataEstimated || observations[0].NormalizedScore != nil {
		t.Fatalf("expected unscored terrain observation, got %+v", observations)
	}
	if !strings.Contains(string(observations[0].RawValue), `"elevationRangeMeters":1.5`) {
		t.Fatalf("expected elevation range in raw data, got %s", observations[0].RawValue)
	}
}

func TestGISTDAElevationProviderDoesNotInventTerrainWithoutKey(t *testing.T) {
	latitude, longitude := 13.7, 100.5
	provider := NewGISTDAElevationProvider(GISTDAElevationConfig{}, nil, cache.Noop{})
	observations, err := provider.Collect(context.Background(), domain.Site{Latitude: &latitude, Longitude: &longitude}, 3000)
	if err != nil {
		t.Fatal(err)
	}
	if len(observations) != 1 || observations[0].Status != domain.DataMissing || observations[0].NormalizedScore != nil {
		t.Fatalf("missing API key must not fabricate terrain data: %+v", observations)
	}
}
