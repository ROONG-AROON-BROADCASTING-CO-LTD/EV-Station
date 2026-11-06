package advisory

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/rbc/ev-station/apps/api/internal/domain"
)

func TestSiteSurfaceFactsDiscardUnconfirmedScores(t *testing.T) {
	for _, tc := range []struct {
		name, status, suitability string
		score                     any
	}{
		{"legacy zero", "", "low", 0},
		{"legacy positive", "", "high", 85},
		{"insufficient null", "insufficient_evidence", "not_assessable", nil},
		{"insufficient contradictory score", "insufficient_evidence", "low", 0},
		{"assessed null", "assessed", "low", nil},
		{"assessed invalid grade", "assessed", "unknown", 55},
		{"assessed negative", "assessed", "low", -1},
		{"assessed above maximum", "assessed", "high", 101},
		{"unknown status", "unknown", "low", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw, _ := json.Marshal(map[string]any{
				"assessmentStatus": tc.status, "suitability": tc.suitability, "score": tc.score,
				"summary": "OLD_UNCONFIRMED_CLAIM", "disclaimer": "OLD_UNCONFIRMED_CLAIM",
				"surfaceTypes": []string{"OLD_UNCONFIRMED_CLAIM"}, "observedRisks": []string{"OLD_UNCONFIRMED_CLAIM"},
				"recommendedImprovements": []string{"OLD_UNCONFIRMED_CLAIM"},
				"entranceWidthEstimate":   map[string]any{"minimumMeters": 8, "maximumMeters": 9},
			})
			facts, err := compactRunFacts(domain.AnalysisRun{Metrics: []domain.Metric{{
				Type: "site_readiness", Status: domain.DataPreliminary, RawValue: raw,
				Source:      domain.DataSource{Name: "Gemini", Methodology: "OLD_UNCONFIRMED_CLAIM"},
				Assumptions: []string{"OLD_UNCONFIRMED_CLAIM"},
			}}})
			if err != nil {
				t.Fatal(err)
			}
			metric := facts["metrics"].([]map[string]any)[0]
			if metric["status"] != domain.DataMissing {
				t.Fatalf("unconfirmed surface must be missing in AI facts: %+v", metric)
			}
			observed := metric["observedValues"].(map[string]any)
			wantStatus := "unconfirmed"
			if tc.status == "insufficient_evidence" {
				wantStatus = tc.status
			}
			if observed["assessmentStatus"] != wantStatus {
				t.Fatalf("wrong evidence status supplied to AI: %+v", observed)
			}
			if score, present := observed["score"]; !present || score != nil || observed["suitability"] != "not_assessable" {
				t.Fatalf("unconfirmed surface must not supply a numeric score or low suitability: %+v", observed)
			}
			if _, present := observed["entranceWidthEstimate"]; present {
				t.Fatal("unconfirmed entrance estimate was supplied to AI")
			}
			encoded, _ := json.Marshal(facts)
			if strings.Contains(string(encoded), "OLD_UNCONFIRMED_CLAIM") {
				t.Fatalf("unconfirmed scored prose was supplied to AI: %s", encoded)
			}
		})
	}
}

func TestSiteSurfaceFactsTreatMissingPayloadAsUnassessed(t *testing.T) {
	for _, raw := range []json.RawMessage{nil, json.RawMessage(`null`), json.RawMessage(`{}`), json.RawMessage(`{"assessmentStatus":"assessed",`)} {
		facts, err := compactRunFacts(domain.AnalysisRun{Metrics: []domain.Metric{{Type: "site_readiness", Status: domain.DataPreliminary, RawValue: raw}}})
		if err != nil {
			t.Fatal(err)
		}
		metric := facts["metrics"].([]map[string]any)[0]
		observed := metric["observedValues"].(map[string]any)
		if metric["status"] != domain.DataMissing || observed["assessmentStatus"] != "unconfirmed" || observed["score"] != nil {
			t.Fatalf("missing payload must not become an assessment: %+v", metric)
		}
	}
}

func TestSiteSurfaceFactsPreserveAssessedZeroAndOtherMetrics(t *testing.T) {
	for _, score := range []float64{0, 55} {
		raw, _ := json.Marshal(map[string]any{"assessmentStatus": "assessed", "suitability": "low", "score": score, "summary": "Visible ground", "entranceWidthEstimate": map[string]any{"minimumMeters": 2, "maximumMeters": 3}})
		facts, err := compactRunFacts(domain.AnalysisRun{Metrics: []domain.Metric{
			{Type: "site_readiness", Status: domain.DataPreliminary, RawValue: raw},
			{Type: "population", Status: domain.DataEstimated, RawValue: json.RawMessage(`{"count":100,"score":0}`)},
		}})
		if err != nil {
			t.Fatal(err)
		}
		metrics := facts["metrics"].([]map[string]any)
		surface := metrics[0]["observedValues"].(map[string]any)
		if metrics[0]["status"] != domain.DataPreliminary || surface["score"] != score || surface["suitability"] != "low" || surface["summary"] != "Visible ground" || surface["entranceWidthEstimate"] == nil {
			t.Fatalf("valid assessed score/evidence must survive: %+v", metrics[0])
		}
		population := metrics[1]["observedValues"].(map[string]any)
		if metrics[1]["status"] != domain.DataEstimated || population["score"] != float64(0) || population["count"] != float64(100) {
			t.Fatalf("unrelated metrics must be unchanged: %+v", metrics[1])
		}
	}
}
