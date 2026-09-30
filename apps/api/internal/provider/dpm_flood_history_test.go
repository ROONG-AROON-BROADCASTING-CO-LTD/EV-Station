package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rbc/ev-station/apps/api/internal/cache"
	"github.com/rbc/ev-station/apps/api/internal/domain"
)

const dpmFloodFixtureCSV = "PROVINCE_NAME,AMPHUR_NAME ,DISTRICT_CODE,DISTRICT_NAME ,Y2562,Y2563,Y2564,Y2565,Y2566,Y2567\n" +
	"เชียงใหม่,เชียงใหม่,500101,ศรีภูมิ,1,,2,,,,\n" +
	"เชียงใหม่,เชียงใหม่,500102,พระสิงห์,,1,,,,,\n" +
	"เชียงใหม่,แม่ริม,501301,ริมใต้,,,,,,,\n"

const dpmFloodSecondPartFixtureCSV = "PROVINCE_NAME,AMPHUR_NAME ,DISTRICT_CODE,DISTRICT_NAME ,Y2562,Y2563,Y2564,Y2565,Y2566,Y2567\n" +
	"เชียงใหม่,เชียงใหม่,500101,ศรีภูมิ,1,,2,,,,\n" +
	"เชียงใหม่,เชียงใหม่,500103,ช้างม่อย,,,,1,,,\n"

func TestParseDPMFloodHistoryGroupsReportedYearsByDistrict(t *testing.T) {
	areas, err := parseDPMFloodHistoryCSVs([][]byte{[]byte(dpmFloodFixtureCSV), []byte(dpmFloodSecondPartFixtureCSV)})
	if err != nil {
		t.Fatalf("parseDPMFloodHistoryCSV returned error: %v", err)
	}
	area, found := findDPMFloodArea(areas, "จังหวัดเชียงใหม่", "อำเภอเมืองเชียงใหม่")
	if !found {
		t.Fatal("expected the city district to match the DDPM district record")
	}
	if len(area.ReportedYears) != 4 || area.ReportedYears[0] != 2562 || area.ReportedYears[1] != 2563 || area.ReportedYears[2] != 2564 || area.ReportedYears[3] != 2565 {
		t.Fatalf("reported years = %v, want [2562 2563 2564 2565]", area.ReportedYears)
	}
	if area.ReportedVillageIncidents != 5 {
		t.Fatalf("reported village incidents = %d, want 5 after skipping a duplicate row", area.ReportedVillageIncidents)
	}
	if area.AffectedSubdistrictCount != 3 {
		t.Fatalf("affected subdistrict count = %d, want 3", area.AffectedSubdistrictCount)
	}
}

type dpmReverseGeocoderFixture struct {
	result ReverseGeocodingResult
	err    error
}

func (g dpmReverseGeocoderFixture) Reverse(context.Context, float64, float64) (ReverseGeocodingResult, error) {
	return g.result, g.err
}

func TestDPMFloodHistoryProviderReturnsEstimatedDistrictHistory(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/part2" {
			_, _ = w.Write([]byte(dpmFloodSecondPartFixtureCSV))
			return
		}
		_, _ = w.Write([]byte(dpmFloodFixtureCSV))
	}))
	defer server.Close()

	latitude, longitude := 18.79, 98.98
	provider := NewDPMFloodHistoryProvider(DPMFloodHistoryConfig{CSVURLs: []string{server.URL + "/part1", server.URL + "/part2"}, CacheTTL: time.Hour}, server.Client(), cache.Noop{}, dpmReverseGeocoderFixture{result: ReverseGeocodingResult{Province: "เชียงใหม่", District: "อำเภอเมืองเชียงใหม่"}})
	observations, err := provider.Collect(context.Background(), domain.Site{Latitude: &latitude, Longitude: &longitude}, 3000)
	if err != nil {
		t.Fatalf("Collect returned error: %v", err)
	}
	var flood Observation
	for _, observation := range observations {
		if observation.MetricType == "flood" {
			flood = observation
			break
		}
	}
	if flood.Status != domain.DataEstimated {
		t.Fatalf("flood status = %q, want %q", flood.Status, domain.DataEstimated)
	}
	if flood.Source.GeographicScope != "district" || flood.Source.License != "Open Data Common" {
		t.Fatalf("unexpected flood source metadata: %+v", flood.Source)
	}
	var value dpmFloodMetricValue
	if err := json.Unmarshal(flood.RawValue, &value); err != nil {
		t.Fatalf("invalid flood metric JSON: %v", err)
	}
	if value.ReportedFloodYearCount != 4 || value.ReportedVillageIncidents != 5 || value.AffectedSubdistrictCount != 3 || value.PeriodStartYear != 2562 || value.PeriodEndYear != 2567 {
		t.Fatalf("unexpected flood metric: %+v", value)
	}
}

func TestDPMFloodHistoryProviderDoesNotInferFloodFreeFromMissingDistrict(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(dpmFloodFixtureCSV))
	}))
	defer server.Close()

	latitude, longitude := 18.79, 98.98
	provider := NewDPMFloodHistoryProvider(DPMFloodHistoryConfig{CSVURLs: []string{server.URL}}, server.Client(), cache.Noop{}, dpmReverseGeocoderFixture{result: ReverseGeocodingResult{Province: "เชียงใหม่", District: "อำเภอแม่ริม"}})
	observations, err := provider.Collect(context.Background(), domain.Site{Latitude: &latitude, Longitude: &longitude}, 3000)
	if err != nil {
		t.Fatalf("Collect returned error: %v", err)
	}
	for _, observation := range observations {
		if observation.MetricType != "flood" {
			continue
		}
		if observation.Status != domain.DataMissing || !strings.Contains(strings.Join(observation.Assumptions, " "), "not interpreted as no flood risk") {
			t.Fatalf("missing district must remain unknown: %+v", observation)
		}
		return
	}
	t.Fatal("expected a flood observation")
}

func TestDPMFloodHistoryDefaultSnapshotAvoidsDownloadsAndKeepsGeographicScopes(t *testing.T) {
	transport := &dpmNoDownloadTransport{}
	latitude, longitude := 14.35, 100.57
	p := NewDPMFloodHistoryProvider(DPMFloodHistoryConfig{}, &http.Client{Transport: transport}, cache.Noop{}, dpmReverseGeocoderFixture{
		result: ReverseGeocodingResult{Province: "พระนครศรีอยุธยา", District: "พระนครศรีอยุธยา"},
	})
	observations, err := p.Collect(context.Background(), domain.Site{Latitude: &latitude, Longitude: &longitude}, 3000)
	if err != nil {
		t.Fatal(err)
	}
	for _, observation := range observations {
		if observation.MetricType != "flood" {
			continue
		}
		var value dpmFloodMetricValue
		if err := json.Unmarshal(observation.RawValue, &value); err != nil {
			t.Fatal(err)
		}
		if observation.Status != domain.DataEstimated || value.PeriodEndYear != 2567 || value.LatestProvinceReport == nil || value.LatestProvinceReport.Year != 2568 || value.LatestProvinceReport.GeographicScope != "province" || value.LatestProvinceReport.ReportedOccurrences != 11 || value.LatestProvinceReport.AffectedHouseholds != 70999 {
			t.Fatalf("unexpected history or provincial context: %+v %+v", observation, value)
		}
		for _, year := range value.ReportedFloodYears {
			if year > value.PeriodEndYear {
				t.Fatal("provincial year was incorrectly added to district history")
			}
		}
		if transport.requests != 0 || len(p.config.CSVURLs) != 0 || observation.Source.RetrievedAt.IsZero() {
			t.Fatal("default history must use an attributed, local snapshot")
		}
		return
	}
	t.Fatal("no flood observation returned")
}

type dpmNoDownloadTransport struct{ requests int }

func (t *dpmNoDownloadTransport) RoundTrip(_ *http.Request) (*http.Response, error) {
	t.requests++
	return nil, fmt.Errorf("network is disabled for this test")
}

func TestDPMProvinceContextDoesNotInventMissingDistrictHistory(t *testing.T) {
	latitude, longitude := 14.35, 100.57
	p := NewDPMFloodHistoryProvider(DPMFloodHistoryConfig{}, nil, cache.Noop{}, dpmReverseGeocoderFixture{
		result: ReverseGeocodingResult{Province: "จังหวัดพระนครศรีอยุธยา", District: "อำเภอไม่มีข้อมูลทดสอบ"},
	})
	observations, _ := p.Collect(context.Background(), domain.Site{Latitude: &latitude, Longitude: &longitude}, 3000)
	for _, observation := range observations {
		if observation.MetricType != "flood" {
			continue
		}
		if observation.Status != domain.DataMissing || !strings.Contains(string(observation.RawValue), "provincial_year_summary") || strings.Contains(string(observation.RawValue), "reportedFloodYearCount") {
			t.Fatalf("province report must not invent district recurrence: %+v", observation)
		}
		return
	}
	t.Fatal("no flood observation")
}

func TestEmbeddedDPMFloodSnapshotIsCompleteAndAttributable(t *testing.T) {
	snapshot, err := loadDPMFloodSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Districts) != 909 || len(snapshot.ProvinceReports) != 74 || snapshot.ImportedAt.IsZero() {
		t.Fatalf("incomplete snapshot: %d districts; %d provinces", len(snapshot.Districts), len(snapshot.ProvinceReports))
	}
	for _, report := range snapshot.ProvinceReports {
		if report.Year != 2568 || report.GeographicScope != "province" || report.License != "Open Data Common" || !strings.HasPrefix(report.SourceURL, "https://catalog.disaster.go.th/") {
			t.Fatalf("invalid provenance: %+v", report)
		}
	}
}

func TestDPMFloodHistoryRejectsMissingYearColumns(t *testing.T) {
	_, err := parseDPMFloodHistoryCSV([]byte("PROVINCE_NAME,AMPHUR_NAME,DISTRICT_CODE,Y2562\nเชียงใหม่,เชียงใหม่,500101,1\n"))
	if err == nil {
		t.Fatal("partial year coverage must not silently be interpreted as a complete six-year history")
	}
}
