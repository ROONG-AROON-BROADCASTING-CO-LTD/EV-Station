package advisory

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rbc/ev-station/apps/api/internal/domain"
)

func TestSiteSurfaceEvidenceContract(t *testing.T) {
	tests := []struct {
		name       string
		output     string
		wantError  bool
		wantStatus string
		wantScore  any
	}{
		{
			name:       "insufficient evidence has no numeric score",
			output:     `{"assessmentStatus":"insufficient_evidence","summary":"The evidence contains only a logo and test text, with no visible ground conditions.","suitability":"not_assessable","score":null,"surfaceTypes":[],"observedRisks":[],"recommendedImprovements":["Provide clear photos of the ground and entrance."],"disclaimer":"Evidence is insufficient for preliminary assessment."}`,
			wantStatus: "insufficient_evidence",
		},
		{
			name:       "genuinely assessed zero remains a score",
			output:     `{"assessmentStatus":"assessed","summary":"Visible ground is impassable due to severe surface collapse.","suitability":"low","score":0,"surfaceTypes":["Collapsed pavement"],"observedRisks":["Large visible holes"],"recommendedImprovements":["Arrange a site survey."],"disclaimer":"Visible evidence only; field verification is required."}`,
			wantStatus: "assessed", wantScore: float64(0),
		},
		{
			name:      "missing evidence status cannot default to assessed",
			output:    `{"summary":"Unable to assess the ground.","suitability":"low","score":0,"disclaimer":"Insufficient evidence."}`,
			wantError: true,
		},
		{
			name:      "missing score cannot default to zero",
			output:    `{"assessmentStatus":"assessed","summary":"Visible ground.","suitability":"low","disclaimer":"Preliminary."}`,
			wantError: true,
		},
		{
			name:      "assessed result cannot omit a numeric score",
			output:    `{"assessmentStatus":"assessed","summary":"Visible ground.","suitability":"low","score":null,"disclaimer":"Preliminary."}`,
			wantError: true,
		},
		{
			name:      "unknown evidence status is rejected",
			output:    `{"assessmentStatus":"unknown","summary":"Visible ground.","suitability":"low","score":0,"disclaimer":"Preliminary."}`,
			wantError: true,
		},
		{
			name:      "insufficient evidence cannot be labeled low suitability",
			output:    `{"assessmentStatus":"insufficient_evidence","summary":"No usable evidence.","suitability":"low","score":null,"disclaimer":"Insufficient evidence."}`,
			wantError: true,
		},
		{
			name:      "insufficient evidence cannot have numeric score",
			output:    `{"assessmentStatus":"insufficient_evidence","summary":"No usable evidence.","suitability":"not_assessable","score":0,"disclaimer":"Insufficient evidence."}`,
			wantError: true,
		},
		{
			name:      "insufficient evidence cannot have entrance estimate",
			output:    `{"assessmentStatus":"insufficient_evidence","summary":"No usable evidence.","suitability":"not_assessable","score":null,"disclaimer":"Insufficient evidence.","entranceWidthEstimate":{"minimumMeters":5,"maximumMeters":6,"confidence":"high","visualEvidence":"Invented gate.","obstructions":[]}}`,
			wantError: true,
		},
		{
			name:      "out of range score is rejected",
			output:    `{"assessmentStatus":"assessed","summary":"Visible ground.","suitability":"high","score":101,"disclaimer":"Preliminary."}`,
			wantError: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{"candidates": []any{map[string]any{"content": map[string]any{"parts": []any{map[string]any{"text": tc.output}}}}}})
			}))
			defer server.Close()
			service := NewGeminiService(GeminiConfig{APIKey: "test-key", Model: "test-model", BaseURL: server.URL}, server.Client())
			result, err := service.AnalyzeSiteSurface(context.Background(), []domain.SiteImage{{MIMEType: "image/png", Data: []byte("test-logo")}}, "en")
			if tc.wantError {
				if !errors.Is(err, ErrInvalidOutput) {
					t.Fatalf("want invalid model output to be rejected, got result=%+v err=%v", result, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := json.Marshal(result)
			if err != nil {
				t.Fatal(err)
			}
			var publicResult map[string]any
			if err := json.Unmarshal(encoded, &publicResult); err != nil {
				t.Fatal(err)
			}
			if publicResult["assessmentStatus"] != tc.wantStatus || publicResult["score"] != tc.wantScore {
				t.Fatalf("unexpected public evidence status or score: %s", encoded)
			}
			if _, present := publicResult["score"]; !present {
				t.Fatalf("score must explicitly be null when unassessable: %s", encoded)
			}
		})
	}
}

func TestSiteSurfaceRejectsNonFiniteScores(t *testing.T) {
	for _, score := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		if validSiteSurface(generatedSiteSurface{AssessmentStatus: "assessed", Summary: "Visible ground.", Suitability: "low", Score: &score, Disclaimer: "Preliminary."}) {
			t.Fatalf("accepted non-finite surface score %v", score)
		}
	}
}
