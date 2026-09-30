package scoring

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/rbc/ev-station/apps/api/internal/domain"
)

func TestPreliminaryScoreUsesAvailableEvidenceAndReportsCoverage(t *testing.T) {
	engine, _ := New(DefaultWeights)
	metrics := []domain.Metric{
		{Type: "traffic", Status: domain.DataVerified, RawValue: json.RawMessage(`{"aadt":80000}`)},
		{Type: "road_accessibility", Status: domain.DataPreliminary, RawValue: json.RawMessage(`{"mappedMajorRoadCount":4,"nearestMajorRoadMeters":100}`)},
		{Type: "ev_demand", Status: domain.DataEstimated, RawValue: json.RawMessage(`{"registeredBev":150000}`)},
		{Type: "population", Status: domain.DataEstimated, RawValue: json.RawMessage(`{"populationDensityPerKm2":3000}`)},
		{Type: "poi", Status: domain.DataVerified, RawValue: json.RawMessage(`{"count":100,"radiusMeters":3000,"coverageComplete":true}`)},
		{Type: "competition", Status: domain.DataVerified, RawValue: json.RawMessage(`{"count":2,"coverageMatched":true}`)},
		{Type: "flood", Status: domain.DataVerified, RawValue: json.RawMessage(`{"mappedFloodRiskAreaCount":0}`)},
		{Type: "electrical", Status: domain.DataPreliminary, RawValue: json.RawMessage(`{"assessmentType":"pea_public_grid_evidence","nearestHighVoltageLineMeters":3674}`)},
		{Type: "site_requirements", Status: domain.DataPreliminary, RawValue: json.RawMessage(`{"internetAvailable":true,"internetSupports24GHz":true,"landLevelingRequired":false,"frontageMeters":7,"electricalExtensionKm":1}`)},
	}
	result := engine.EvaluatePreliminary(metrics)
	if result.Overall == nil {
		t.Fatal("expected a preliminary score")
	}
	if result.Summary.CoveragePercentage != 100 || result.Summary.ScoredMetricCount != 9 {
		t.Fatalf("unexpected coverage: %+v", result.Summary)
	}
	if result.MetricScores["electrical"] != 63.26 {
		t.Fatalf("expected public-grid proximity score, got %v", result.MetricScores["electrical"])
	}
}

func TestPreliminaryFloodScoreUsesDistrictReportedYearsAsAnAdvisory(t *testing.T) {
	engine, _ := New(DefaultWeights)
	result := engine.EvaluatePreliminary([]domain.Metric{{
		Type: "flood", Status: domain.DataEstimated,
		RawValue: json.RawMessage(`{"assessmentType":"administrative_district_historical_reports","reportedFloodYearCount":3}`),
	}})
	if result.MetricScores["flood"] != 70 {
		t.Fatalf("three reported years should produce the transparent 70-point screening score, got %+v", result)
	}
	if !strings.Contains(result.Rules["flood"], "not a parcel-flood probability") {
		t.Fatalf("flood rule must state its geographic and calibration limits: %q", result.Rules["flood"])
	}
}

func TestPreliminaryFloodScoreKeepsMissingHistoryOutOfScore(t *testing.T) {
	engine, _ := New(DefaultWeights)
	result := engine.EvaluatePreliminary([]domain.Metric{{Type: "flood", Status: domain.DataMissing}})
	if _, found := result.MetricScores["flood"]; found {
		t.Fatal("missing flood-history records must not be scored as low risk")
	}
}

func TestPreliminaryFloodScoreExcludesProvincialContextEvenWithStaleScore(t *testing.T) {
	engine, _ := New(DefaultWeights)
	staleScore := 100.0
	for _, status := range []domain.DataStatus{domain.DataMissing, domain.DataEstimated} {
		result := engine.EvaluatePreliminary([]domain.Metric{{Type: "flood", Status: status, NormalizedScore: &staleScore,
			RawValue: json.RawMessage(`{"assessmentType":"provincial_year_summary","latestProvinceReport":{"year":2568,"reportedOccurrences":11}}`),
		}})
		if _, found := result.MetricScores["flood"]; found {
			t.Fatal("province context must not receive or retain a district flood score")
		}
	}
}

func TestPreliminaryFloodScoreValidatesStatedHistoryPeriod(t *testing.T) {
	engine, _ := New(DefaultWeights)
	for _, test := range []struct {
		raw    string
		scored bool
		score  float64
	}{
		{`{"assessmentType":"administrative_district_historical_reports","periodStartYear":2562,"periodEndYear":2567,"reportedFloodYearCount":7}`, false, 0},
		{`{"assessmentType":"administrative_district_historical_reports","periodStartYear":2562,"periodEndYear":2568,"reportedFloodYearCount":7}`, true, 30},
		{`{"assessmentType":"administrative_district_historical_reports","periodStartYear":2568,"periodEndYear":2562,"reportedFloodYearCount":1}`, false, 0},
		{`{"assessmentType":"administrative_district_historical_reports","reportedFloodYearCount":0}`, false, 0},
		{`{"latestProvinceReport":{"year":2568}}`, false, 0},
	} {
		result := engine.EvaluatePreliminary([]domain.Metric{{Type: "flood", Status: domain.DataEstimated, RawValue: json.RawMessage(test.raw)}})
		score, scored := result.MetricScores["flood"]
		if scored != test.scored || score != test.score {
			t.Fatalf("period validation failed for %s: %v", test.raw, result.MetricScores)
		}
	}
}

func TestPreliminaryScoreExcludesSiteSurfaceAssessment(t *testing.T) {
	engine, _ := New(DefaultWeights)
	result := engine.EvaluatePreliminary([]domain.Metric{{Type: "site_readiness", Status: domain.DataPreliminary, RawValue: json.RawMessage(`{"analysisScope":"ground_surface_only","score":95}`)}})
	if len(result.MetricScores) != 0 || result.Summary.RequiredMetricCount != 9 {
		t.Fatalf("ground-surface assessment must remain outside the location score: %+v", result)
	}
}

func TestPreliminaryScoreExcludesCompetitionWithoutLocalCoverage(t *testing.T) {
	engine, _ := New(DefaultWeights)
	result := engine.EvaluatePreliminary([]domain.Metric{{Type: "competition", Status: domain.DataPreliminary, RawValue: json.RawMessage(`{"count":0,"coverageMatched":false}`)}})
	if _, found := result.MetricScores["competition"]; found {
		t.Fatal("competition without local source coverage must not receive a score")
	}
}

func TestPreliminaryScoreExcludesIncompleteZeroPOIAndRenormalizesWeights(t *testing.T) {
	engine, _ := New(DefaultWeights)
	fullScore := 100.0
	stalePOIScore := 10.0
	metrics := []domain.Metric{
		{Type: "traffic", Status: domain.DataVerified, NormalizedScore: &fullScore},
		{Type: "road_accessibility", Status: domain.DataPreliminary, NormalizedScore: &fullScore},
		{Type: "ev_demand", Status: domain.DataEstimated, NormalizedScore: &fullScore},
		{Type: "population", Status: domain.DataEstimated, NormalizedScore: &fullScore},
		// This reproduces an old report: zero POIs plus a score saved before
		// coverage safeguards were added.
		{Type: "poi", Status: domain.DataVerified, NormalizedScore: &stalePOIScore, RawValue: json.RawMessage(`{"count":0,"radiusMeters":3000}`)},
		{Type: "competition", Status: domain.DataVerified, NormalizedScore: &fullScore},
		{Type: "flood", Status: domain.DataVerified, NormalizedScore: &fullScore, RawValue: json.RawMessage(`{"mappedFloodRiskAreaCount":0}`)},
		{Type: "electrical", Status: domain.DataPreliminary, NormalizedScore: &fullScore},
	}

	result := engine.EvaluatePreliminary(metrics)
	if _, found := result.MetricScores["poi"]; found {
		t.Fatal("incomplete zero-POI evidence must not retain a stale score")
	}
	if result.Summary.CoveragePercentage != 79.17 {
		t.Fatalf("expected POI weight to be excluded and remaining weights renormalized, got %+v", result.Summary)
	}
	if result.Overall == nil || *result.Overall != 100 {
		t.Fatalf("expected renormalized score of 100, got %+v", result.Overall)
	}
}

func TestPreliminaryFloodScoreDropsStaleScoreWithoutFloodEvidence(t *testing.T) {
	engine, _ := New(DefaultWeights)
	staleScore := 80.0
	result := engine.EvaluatePreliminary([]domain.Metric{{Type: "flood", Status: domain.DataEstimated, NormalizedScore: &staleScore, RawValue: json.RawMessage(`{}`)}})
	if _, found := result.MetricScores["flood"]; found {
		t.Fatal("a stored score with empty flood evidence must be excluded")
	}
}

func TestPreliminaryScoreRequiresMinimumCoverage(t *testing.T) {
	engine, _ := New(DefaultWeights)
	result := engine.EvaluatePreliminary([]domain.Metric{{Type: "traffic", Status: domain.DataVerified, RawValue: json.RawMessage(`{"aadt":20000}`)}})
	if result.Overall != nil || result.Summary.CoveragePercentage != 16.67 {
		t.Fatalf("low-coverage evidence must not produce an overall score: %+v", result)
	}
}

func TestPreliminaryScoreSeparatesRoadAccessibilityFromTrafficVolume(t *testing.T) {
	engine, _ := New(DefaultWeights)
	result := engine.EvaluatePreliminary([]domain.Metric{{Type: "road_accessibility", Status: domain.DataPreliminary, RawValue: json.RawMessage(`{"mappedMajorRoadCount":3,"nearestMajorRoadMeters":80}`)}})
	if _, found := result.MetricScores["road_accessibility"]; !found {
		t.Fatal("expected road accessibility evidence to receive its own screening score")
	}
	if _, found := result.MetricScores["traffic"]; found {
		t.Fatal("road accessibility must not be recorded as traffic volume")
	}
}

func TestPreliminaryScoreUsesExplicitRoadTrafficPotentialOnlyWhenLabelled(t *testing.T) {
	engine, _ := New(DefaultWeights)
	result := engine.EvaluatePreliminary([]domain.Metric{{Type: "traffic", Status: domain.DataEstimated, RawValue: json.RawMessage(`{"assessmentType":"osm_road_traffic_potential","potentialScore":87}`)}})
	if result.MetricScores["traffic"] != 87 {
		t.Fatalf("expected explicit road traffic potential score, got %+v", result.MetricScores)
	}
}
