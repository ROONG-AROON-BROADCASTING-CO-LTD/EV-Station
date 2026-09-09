package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rbc/ev-station/apps/api/internal/cache"
	"github.com/rbc/ev-station/apps/api/internal/domain"
)

func TestOpenChargeMapProviderReturnsNearbyCompetitionEvidence(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Query().Get("key") != "ocm-key" || request.URL.Query().Get("distanceunit") != "KM" {
			t.Fatalf("expected authenticated Open Charge Map request: %s", request.URL.RawQuery)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`[{"ID":42,"AddressInfo":{"Title":"RBC Charger","Latitude":13.7,"Longitude":100.5},"OperatorInfo":{"Title":"RBC"}}]`))
	}))
	defer server.Close()
	latitude, longitude := 13.7, 100.5
	provider := NewOpenChargeMapProvider(OpenChargeMapConfig{APIKey: "ocm-key", Endpoint: server.URL, CacheTTL: time.Hour}, server.Client(), cache.Noop{})
	observations, err := provider.Collect(context.Background(), domain.Site{Latitude: &latitude, Longitude: &longitude}, 3000)
	if err != nil {
		t.Fatal(err)
	}
	competition := observationByType(observations, "competition")
	if competition.Status != domain.DataPreliminary || len(competition.RawValue) == 0 {
		t.Fatalf("expected supplemental competition evidence, got %+v", competition)
	}
}

func TestOpenChargeMapProviderWithoutKeyDoesNotInventCompetition(t *testing.T) {
	latitude, longitude := 13.7, 100.5
	provider := NewOpenChargeMapProvider(OpenChargeMapConfig{}, nil, cache.Noop{})
	observations, err := provider.Collect(context.Background(), domain.Site{Latitude: &latitude, Longitude: &longitude}, 3000)
	if err != nil {
		t.Fatal(err)
	}
	if competition := observationByType(observations, "competition"); competition.Status != domain.DataMissing {
		t.Fatalf("missing key must not invent competition evidence: %+v", competition)
	}
}
