package domain

import "testing"

func TestInvestmentDecisionForScoreBoundary(t *testing.T) {
	below := 59.9
	atThreshold := 60.0
	tests := []struct {
		name  string
		score *float64
		want  string
	}{
		{name: "no score", score: nil, want: InvestmentDecisionNotRecommended},
		{name: "59.9 is not recommended", score: &below, want: InvestmentDecisionNotRecommended},
		{name: "60 is invest", score: &atThreshold, want: InvestmentDecisionInvest},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := InvestmentDecisionForScore(test.score); got != test.want {
				t.Fatalf("InvestmentDecisionForScore(%v) = %q, want %q", test.score, got, test.want)
			}
		})
	}
}
