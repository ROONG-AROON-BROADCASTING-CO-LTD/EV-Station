package provider

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/rbc/ev-station/apps/api/internal/config"
	"github.com/rbc/ev-station/apps/api/internal/domain"
	"github.com/rbc/ev-station/apps/api/internal/telemetry"
)

// Explicit opt-in; public endpoints only, no customer writes or AI generation.
func TestPublicProviderReliabilityLive(t *testing.T) {
	if os.Getenv("RBC_RELIABILITY_LIVE") != "1" {
		t.Skip("live audit not requested")
	}
	lat, lon := 13.7712686, 100.5827703
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	for name, p := range map[string]AnalysisProvider{
		"osm":      NewOSMProvider(OSMConfig{Endpoint: cfg.OverpassURL, FallbackEndpoints: cfg.OverpassFallbackURLs, UserAgent: cfg.ExternalUserAgent}, nil, nil),
		"worldpop": NewWorldPopProvider(WorldPopConfig{Year: 2025}, nil, nil),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
			defer cancel()
			ctx = telemetry.WithRecorder(ctx, telemetry.NewSlogRecorder(slog.New(slog.NewJSONHandler(os.Stdout, nil))))
			started := time.Now()
			items, err := p.Collect(ctx, domain.Site{Latitude: &lat, Longitude: &lon}, 3000)
			if err != nil {
				t.Fatal("collection failed")
			}
			for _, item := range items {
				if item.MetricType == "population" && name == "worldpop" || item.MetricType == "road_accessibility" && name == "osm" {
					t.Logf("provider=%s status=%s duration=%s", name, item.Status, time.Since(started))
					if item.Status == domain.DataMissing {
						t.Error("public source unavailable in this live sample")
					}
				}
			}
		})
	}
}
