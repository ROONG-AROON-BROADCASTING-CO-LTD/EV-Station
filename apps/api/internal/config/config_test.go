package config

import (
	"testing"
)

func TestDisasterKeyDoesNotReuseSphereCredential(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("GISTDA_API_KEY", "sphere-test")
	t.Setenv("GISTDA_DISASTER_API_KEY", "")
	t.Setenv("GISTDA_FLOOD_REFERENCE_ENABLED", "")
	t.Setenv("GISTDA_GI_SERVICE_API_KEY", "")
	t.Setenv("GISTDA_DISASTER_ENABLED", "")
	cfg, err := Load()
	if err != nil || cfg.GISTDADisasterAPIKey != "" || cfg.GISTDAFloodReferenceEnabled || cfg.GISTDAGIServiceAPIKey != "" || cfg.GISTDADisasterEnabled {
		t.Fatal("Disaster key must be configured separately", err)
	}
}

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
	t.Setenv("JWT_SECRET", "test-production-secret-with-32-bytes-minimum")
	t.Setenv("AUTH_REQUIRED", "true")
	t.Setenv("DATABASE_URL", "postgres://test/test")

	config, err := Load()
	if err != nil {
		t.Fatalf("Load() returned an unexpected error: %v", err)
	}
	if config.JWTSecret != "test-production-secret-with-32-bytes-minimum" {
		t.Fatalf("JWTSecret = %q, want configured secret", config.JWTSecret)
	}
}

func TestProductionRejectsUnsafeConfiguration(t *testing.T) {
	for _, setting := range []struct{ key, value string }{
		{"JWT_SECRET", "change-this-before-production"},
		{"JWT_SECRET", "development-only-change-this-secret"},
		{"AUTH_REQUIRED", "false"},
		{"DATABASE_URL", ""},
	} {
		t.Run(setting.key+setting.value, func(t *testing.T) {
			t.Setenv("APP_ENV", "production")
			t.Setenv("JWT_SECRET", "test-production-secret-with-32-bytes-minimum")
			t.Setenv("AUTH_REQUIRED", "true")
			t.Setenv("DATABASE_URL", "postgres://test/test")
			t.Setenv(setting.key, setting.value)
			if _, err := Load(); err == nil {
				t.Fatal("unsafe production configuration accepted")
			}
		})
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
