package scoring

import (
	"encoding/json"
	"fmt"
	"github.com/rbc/ev-station/apps/api/internal/domain"
	"testing"
)

func TestGatewayFloodHistoryScoresEvenWithoutDistrictAndExcludesCurrent(t *testing.T) {
	base := `{"gatewayFlood":{"assessmentType":"gistda_gateway_flood","historical":{"status":"available","license":"Open Data Common","complete":true,"periodStartYear":2011,"periodEndYear":2023,"yearsObserved":%s},"latest":{"status":"available","result":"found"}}}`
	for _, tc := range []struct {
		years    string
		eligible bool
	}{{`[2011,2023]`, true}, {`[]`, false}, {`[2024]`, false}, {`[2011,2011]`, false}} {
		raw := json.RawMessage(fmt.Sprintf(base, tc.years))
		score, _, ok := preliminaryMetricScore(domain.Metric{Type: "flood", Status: domain.DataMissing, RawValue: raw})
		if ok != tc.eligible || (ok && (score < 84.61 || score > 84.62)) {
			t.Fatal(tc, score, ok)
		}
	}
}
