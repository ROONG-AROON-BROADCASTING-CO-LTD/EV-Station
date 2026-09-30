package config

import (
	"testing"
)

func TestLoadUsesDevelopmentSecretOutsideProduction(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("JWT_SECRET", "")

	config, err := Load()
	if err != nil {
		t.Fatalf("Load() returned an unexpected error: %v", err)
	}
	if config.JWTSecret != "development-only-change-this-secret" {
		t.Fatalf("JWTSecret = %q, want development fallback", config.JWTSecret)
	}
}

func TestLoadPreservesExplicitDPMFloodHistoryOverride(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("DPM_FLOOD_HISTORY_CSV_URLS", "https://example.test/first.csv,https://example.test/second.csv")
	config, err := Load()
	if err != nil || len(config.DPMFloodHistoryCSVURLs) != 2 {
		t.Fatalf("explicit CSV override was not preserved: %v %v", config.DPMFloodHistoryCSVURLs, err)
	}
}

func TestLoadRequiresJWTSecretInProduction(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("JWT_SECRET", "")

	if _, err := Load(); err == nil {
		t.Fatal("Load() succeeded without JWT_SECRET in production")
	}
}

func TestLoadAcceptsJWTSecretInProduction(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("JWT_SECRET", "production-secret")

	config, err := Load()
	if err != nil {
		t.Fatalf("Load() returned an unexpected error: %v", err)
	}
	if config.JWTSecret != "production-secret" {
		t.Fatalf("JWTSecret = %q, want configured secret", config.JWTSecret)
	}
}

func TestLoadUsesEmbeddedDPMFloodHistoryByDefault(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("DPM_FLOOD_HISTORY_CSV_URLS", "")
	t.Setenv("DPM_FLOOD_HISTORY_CSV_URL", "")

	config, err := Load()
	if err != nil {
		t.Fatalf("Load() returned an unexpected error: %v", err)
	}
	if len(config.DPMFloodHistoryCSVURLs) != 0 {
		t.Fatalf("DPMFloodHistoryCSVURLs = %v, want embedded snapshot", config.DPMFloodHistoryCSVURLs)
	}
}
