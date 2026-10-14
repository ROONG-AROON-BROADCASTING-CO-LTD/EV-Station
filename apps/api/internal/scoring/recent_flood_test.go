package scoring

import (
	"encoding/json"
	"github.com/rbc/ev-station/apps/api/internal/domain"
	"testing"
)

func TestRecentFloodDoesNotSubstituteHistoricalEvidence(t *testing.T) {
	raw := json.RawMessage(`{"recentFlood":{"assessmentType":"gistda_recent_flood","windows":[{"days":1,"status":"available","featureCount":0}]}}`)
	stale := 100.0
	metric := domain.Metric{Type: "flood", Status: domain.DataVerified, NormalizedScore: &stale, RawValue: raw}
	if _, _, eligible := preliminaryMetricScore(metric); eligible {
		t.Fatal("recent evidence alone must not retain a historical score")
	}
	metric.RawValue = json.RawMessage(`{"assessmentType":"administrative_district_historical_reports","reportedFloodYearCount":3,"periodStartYear":2562,"periodEndYear":2567,"recentFlood":{"assessmentType":"gistda_recent_flood","windows":[]}}`)
	if score, _, eligible := preliminaryMetricScore(metric); !eligible || score != 70 {
		t.Fatal("recent context changed district scoring", score, eligible)
	}
}
func TestReuseLicenseRejectsRestrictedVariants(t *testing.T) {
	for _, license := range []string{"", "unknown", "CC-BY-NC-4.0", "CC-BY-ND-4.0", "may be ODbL"} {
		if hasOpenReuseLicense(license) {
			t.Fatal("accepted undeclared or restricted licence", license)
		}
	}
	if !hasOpenReuseLicense("Open Data Common") {
		t.Fatal("rejected explicit open licence")
	}
}
