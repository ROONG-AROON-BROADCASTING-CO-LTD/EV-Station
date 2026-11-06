package analysis

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rbc/ev-station/apps/api/internal/advisory"
	"github.com/rbc/ev-station/apps/api/internal/domain"
	"github.com/rbc/ev-station/apps/api/internal/provider"
	"github.com/rbc/ev-station/apps/api/internal/repository"
	"github.com/rbc/ev-station/apps/api/internal/scoring"
)

func TestSiteSurfaceEvidencePersistsWithoutInventingScore(t *testing.T) {
	var locationScore *float64
	for _, tc := range []struct {
		name, assessmentStatus, suitability string
		score                               *float64
		wantStatus                          domain.DataStatus
	}{
		{"insufficient evidence", "insufficient_evidence", "not_assessable", nil, domain.DataMissing},
		{"assessed zero", "assessed", "low", floatPointer(0), domain.DataPreliminary},
		{"assessed positive", "assessed", "moderate", floatPointer(55), domain.DataPreliminary},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			repo := repository.NewMemory()
			now := time.Now().UTC()
			site := domain.Site{ID: uuid.New(), Name: "Synthetic evidence QA", LandSize: 100, LandSizeUnit: "sqm", CreatedAt: now, UpdatedAt: now}
			if _, err := repo.CreateSite(ctx, site); err != nil {
				t.Fatal(err)
			}
			if err := repo.AddSiteImages(ctx, site.ID, []domain.SiteImage{
				{ID: uuid.New(), SiteID: site.ID, MIMEType: "image/png", Data: []byte("synthetic image"), CreatedAt: now},
				{ID: uuid.New(), SiteID: site.ID, MIMEType: "application/pdf", Data: []byte("synthetic document"), CreatedAt: now},
			}); err != nil {
				t.Fatal(err)
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assessment, _ := json.Marshal(map[string]any{
					"assessmentStatus": tc.assessmentStatus, "suitability": tc.suitability, "score": tc.score,
					"summary": "Synthetic response for evidence handling", "surfaceTypes": []string{},
					"observedRisks": []string{}, "recommendedImprovements": []string{"Supply clear ground photos"},
					"disclaimer": "Not an engineering assessment",
				})
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{"candidates": []any{map[string]any{"content": map[string]any{"parts": []any{map[string]any{"text": string(assessment)}}}}}})
			}))
			defer server.Close()
			engine, _ := scoring.New(scoring.DefaultWeights)
			gemini := advisory.NewGeminiService(advisory.GeminiConfig{APIKey: "test-key", Model: "test-model", BaseURL: server.URL, Timeout: time.Second}, server.Client())
			service := NewService(repo, provider.FixtureProvider{}, engine, gemini)
			run, err := service.Run(ctx, site.ID, 3000)
			if err != nil {
				t.Fatal(err)
			}
			stored, err := service.Get(ctx, run.ID)
			if err != nil {
				t.Fatal(err)
			}
			if stored.Status != "completed" || stored.OverallScore == nil {
				t.Fatalf("missing surface evidence must not prevent location analysis: %+v", stored)
			}
			if locationScore == nil {
				locationScore = stored.OverallScore
			} else if *stored.OverallScore != *locationScore {
				t.Fatal("surface assessment must not change the independent location score")
			}
			for _, metric := range stored.Metrics {
				if metric.Type != "site_readiness" {
					continue
				}
				if metric.Status != tc.wantStatus || metric.NormalizedScore != nil {
					t.Fatalf("surface metric status/score mismatch: status=%s normalized=%v", metric.Status, metric.NormalizedScore)
				}
				var raw map[string]any
				if err := json.Unmarshal(metric.RawValue, &raw); err != nil {
					t.Fatal(err)
				}
				if raw["assessmentStatus"] != tc.assessmentStatus || raw["suitability"] != tc.suitability {
					t.Fatalf("evidence status was not persisted: %s", metric.RawValue)
				}
				score, present := raw["score"]
				if !present || (tc.score == nil && score != nil) || (tc.score != nil && score != *tc.score) {
					t.Fatalf("expected null for missing evidence, and preserved real numeric score: %s", metric.RawValue)
				}
				if raw["imageCount"] != float64(1) || raw["documentCount"] != float64(1) || raw["summary"] == "" {
					t.Fatalf("evidence details were lost: %s", metric.RawValue)
				}
				return
			}
			t.Fatal("surface metric was not persisted")
		})
	}
}
