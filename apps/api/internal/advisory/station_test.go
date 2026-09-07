package advisory

import (
	"testing"

	"github.com/rbc/ev-station/apps/api/internal/domain"
)

func TestStationRejectsUnsupportedAndContradictorySizes(t *testing.T) {
	text := StationText{Reason: "Reason", Assumptions: []string{}, MissingData: []string{}}
	for _, tc := range []struct {
		power, count int
		valid        bool
	}{{120, 1, true}, {180, 2, true}, {240, 3, true}, {0, 0, false}, {121, 1, false}, {120, 0, false}, {0, 1, false}, {240, -1, false}, {240, 101, false}} {
		r := StationRecommendation{PowerKW: tc.power, ChargerCount: tc.count, TH: text, EN: text}
		if validStation(r) != tc.valid {
			t.Errorf("power %d count %d", tc.power, tc.count)
		}
	}
}

func TestPreliminaryCabinetLimitUsesSubmittedLandArea(t *testing.T) {
	tests := []struct {
		name string
		site domain.Site
		want int
	}{
		{name: "small plot retains one cabinet", site: domain.Site{LandSize: 50, LandSizeUnit: "sqwah"}, want: 1},
		{name: "two hundred square wah allows two cabinets", site: domain.Site{LandSize: 200, LandSizeUnit: "sqwah"}, want: 2},
		{name: "rai is converted to square wah", site: domain.Site{LandSize: 5, LandSizeUnit: "rai"}, want: 20},
		{name: "square metres are converted to square wah", site: domain.Site{LandSize: 800, LandSizeUnit: "sqm"}, want: 2},
	}
	for _, tc := range tests {
		if got := cabinetLimitFromLandArea(areaInSquareWah(tc.site)); got != tc.want {
			t.Errorf("%s: got %d, want %d", tc.name, got, tc.want)
		}
	}
}
