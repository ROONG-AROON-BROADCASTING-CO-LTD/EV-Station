package scoring

import (
	"encoding/json"
	"github.com/rbc/ev-station/apps/api/internal/domain"
	"testing"
)

func TestMissingEvidenceCannotCreateScores(t *testing.T) {
	score := 99.0
	for _, metric := range []domain.Metric{
		{Type: "population", Status: domain.DataMissing, NormalizedScore: &score},
		{Type: "population", Status: domain.DataEstimated, RawValue: json.RawMessage(`{}`)},
		{Type: "ev_demand", Status: domain.DataVerified, RawValue: json.RawMessage(`{}`)},
	} {
		if _, _, ok := preliminaryMetricScore(metric); ok {
			t.Fatalf("missing evidence scored: %+v", metric)
		}
	}
}

func TestDuplicateMetricsDoNotInflateCoverage(t *testing.T) {
	e, _ := New(DefaultWeights)
	score := 80.0
	m := domain.Metric{Type: "population", Status: domain.DataEstimated, NormalizedScore: &score}
	result := e.EvaluatePreliminary([]domain.Metric{m, m, m, m, m, m})
	if result.Overall != nil || result.Summary.CoveragePercentage != 12.5 || result.Summary.ScoredMetricCount != 1 {
		t.Fatalf("inflated coverage: %+v", result)
	}
}
