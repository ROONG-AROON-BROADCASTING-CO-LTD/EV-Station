package advisory

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/rbc/ev-station/apps/api/internal/domain"
)

func TestStationUsesCombinedEvidenceBeforeUtilityCapacityIsConfirmed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		request := string(body)
		for _, fact := range []string{"traffic", "ev_demand", "population", "competition", "layoutCabinetLimit"} {
			if !strings.Contains(request, fact) {
				t.Fatalf("AI request did not include %q", fact)
			}
		}
		if !strings.Contains(request, "Do not describe or label the recommendation as preliminary or เบื้องต้น") {
			t.Fatal("AI prompt must present the result as a recommendation without calling it preliminary")
		}
		result, err := json.Marshal(map[string]any{"powerKw": 120, "chargerCount": 1, "th": map[string]any{"reason": "ข้อมูลทำเลสนับสนุนให้เริ่มหนึ่งตู้", "assumptions": []string{}, "missingData": []string{}}, "en": map[string]any{"reason": "Site evidence supports starting with one cabinet.", "assumptions": []string{}, "missingData": []string{}}})
		if err != nil {
			t.Fatal(err)
		}
		response := map[string]any{"candidates": []any{map[string]any{"content": map[string]any{"parts": []any{map[string]any{"text": string(result)}}}}}}
		responseJSON, err := json.Marshal(response)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write(responseJSON)
	}))
	defer server.Close()

	service := NewGeminiService(GeminiConfig{APIKey: "test", BaseURL: server.URL, Model: "test"}, server.Client())
	frontage := 16.0
	run := domain.AnalysisRun{Metrics: []domain.Metric{
		{Type: "traffic", RawValue: []byte(`{"aadt":4000}`), Status: domain.DataVerified},
		{Type: "ev_demand", RawValue: []byte(`{"registeredEV":20}`), Status: domain.DataVerified},
		{Type: "population", RawValue: []byte(`{"population":10000}`), Status: domain.DataVerified},
		{Type: "competition", RawValue: []byte(`{"competitorCount":4}`), Status: domain.DataVerified},
	}}
	result, err := service.Station(context.Background(), run, domain.Site{LandSize: 400, LandSizeUnit: "sqwah", FrontageMeters: &frontage})
	if err != nil {
		t.Fatal(err)
	}
	if !result.GeneratedByAI || result.CapacityConfirmed || result.RecommendationStage != "screening_evidence" || result.ChargerCount != 1 || result.PowerKW != 120 {
		t.Fatalf("unexpected recommendation: %+v", result)
	}
}

func TestStationRejectsUnsupportedAndContradictorySizes(t *testing.T) {
	text := StationText{Reason: "Reason", Assumptions: []string{}, MissingData: []string{}}
	for _, tc := range []struct {
		power, count int
		valid        bool
	}{{120, 1, true}, {180, 2, true}, {240, 3, true}, {0, 0, false}, {121, 1, false}, {120, 0, false}, {0, 1, false}, {240, -1, false}, {240, 101, false}} {
		r := StationRecommendation{RecommendationAvailable: true, PowerKW: tc.power, ChargerCount: tc.count, TH: text, EN: text}
		if validStation(r) != tc.valid {
			t.Errorf("power %d count %d", tc.power, tc.count)
		}
	}
}

func TestStationKeepsLayoutQuantityWhenGridEvidenceIsUnconfirmed(t *testing.T) {
	result := initialPhaseStationRecommendation(7_600, franchiseLayoutPlan{Code: "L", RecommendedAreaSqWah: 400, CabinetLimit: 4}, 1, 4)
	if !result.RecommendationAvailable || result.CapacityConfirmed || result.PowerKW != 120 || result.ChargerCount != 4 || result.TotalPowerKW != 480 || !validStation(result) {
		t.Fatalf("expected a valid L-layout recommendation, got %+v", result)
	}
	if strings.Contains(result.TH.Reason, "เบื้องต้น") || strings.Contains(result.EN.Reason, "preliminary") {
		t.Fatalf("recommendation text should not be labelled preliminary: %+v", result)
	}
	confirmed := domain.AnalysisRun{Metrics: []domain.Metric{{Type: "electrical", Status: domain.DataVerified, Source: domain.DataSource{SiteVerification: "utility_capacity_confirmed"}}}}
	if !hasConfirmedElectricalCapacity(confirmed) {
		t.Fatal("expected utility-confirmed electrical metric to enable sizing")
	}
	if hasConfirmedElectricalCapacity(domain.AnalysisRun{Metrics: []domain.Metric{{Type: "electrical", Status: domain.DataPreliminary, Source: domain.DataSource{SiteVerification: "preliminary_map_lookup"}}}}) {
		t.Fatal("published proximity evidence must not enable sizing")
	}
}

func TestStationBlocksUndergroundButNotNarrowEntrance(t *testing.T) {
	narrow := 6.9
	if got := StationBlocker(domain.Site{ElectricalSupplyType: "underground"}); got != "underground_electricity" {
		t.Fatalf("StationBlocker() = %q, want underground_electricity", got)
	}
	if got := StationBlocker(domain.Site{FrontageMeters: &narrow}); got != "" {
		t.Fatalf("StationBlocker() = %q, want empty for an informational entrance recommendation", got)
	}
	result := blockedStationRecommendation(400, franchiseLayoutPlan{Code: "L", RecommendedAreaSqWah: 400, CabinetLimit: 4}, "underground_electricity")
	if result.RecommendationAvailable || result.Blocker != "underground_electricity" || !validStation(result) {
		t.Fatalf("expected a valid blocked recommendation, got %+v", result)
	}
}

func TestCabinetLimitUsesLandAreaRegardlessOfFrontage(t *testing.T) {
	frontage5 := 5.0
	frontage10 := 10.0
	frontage12 := 12.0
	if got := cabinetLimitFromSite(domain.Site{FrontageMeters: &frontage5}, 800); got != 4 {
		t.Fatalf("5m frontage got %d, want 4", got)
	}
	if got := cabinetLimitFromSite(domain.Site{FrontageMeters: &frontage10}, 800); got != 4 {
		t.Fatalf("10m frontage got %d, want 4", got)
	}
	if got := cabinetLimitFromSite(domain.Site{FrontageMeters: &frontage12}, 800); got != 4 {
		t.Fatalf("12m frontage got %d, want 4 for the L format", got)
	}
	near22 := 500.0
	medium22 := 3_000.0
	far22 := 6_000.0
	for _, tc := range []struct {
		name     string
		distance *float64
		want     int
	}{{"near 22kv", &near22, 4}, {"medium 22kv", &medium22, 2}, {"far 22kv", &far22, 1}} {
		run := domain.AnalysisRun{Metrics: []domain.Metric{{Type: "electrical", Status: domain.DataPreliminary, RawValue: []byte(`{"voltageCode":"22 kV","nearestHighVoltageLineMeters":` + strconv.FormatFloat(*tc.distance, 'f', 0, 64) + `}`)}}}
		if got := electricalPlanningCabinetLimit(run); got != tc.want {
			t.Errorf("%s got %d, want %d", tc.name, got, tc.want)
		}
	}
}

func TestFranchiseLayoutUsesPublishedTotalAreaIncludingCafeAndCharging(t *testing.T) {
	for _, tc := range []struct {
		area         float64
		code         string
		wantArea     float64
		wantCabinets int
	}{{100, "S", 100, 1}, {200, "M", 200, 2}, {400, "L", 400, 4}, {7_600, "L", 400, 4}} {
		plan := franchiseLayoutForArea(tc.area)
		if plan.Code != tc.code || plan.RecommendedAreaSqWah != tc.wantArea || plan.CabinetLimit != tc.wantCabinets {
			t.Errorf("area %.0f got %+v", tc.area, plan)
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
		{name: "large rai plot is capped at the L project format", site: domain.Site{LandSize: 5, LandSizeUnit: "rai"}, want: 4},
		{name: "square metres are converted to square wah", site: domain.Site{LandSize: 800, LandSizeUnit: "sqm"}, want: 2},
	}
	for _, tc := range tests {
		if got := cabinetLimitFromLandArea(areaInSquareWah(tc.site)); got != tc.want {
			t.Errorf("%s: got %d, want %d", tc.name, got, tc.want)
		}
	}
}
