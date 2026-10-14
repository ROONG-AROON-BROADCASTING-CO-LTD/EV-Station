package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rbc/ev-station/apps/api/internal/cache"
	"github.com/rbc/ev-station/apps/api/internal/domain"
)

func TestNominatimSearchPreservesProvenance(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") != "rbc-test" {
			t.Fatalf("unexpected user agent: %q", r.Header.Get("User-Agent"))
		}
		if r.URL.Query().Get("countrycodes") != "th" || r.URL.Query().Get("q") != "Bang Na" {
			t.Fatalf("unexpected query: %s", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"display_name":"Bang Na, Bangkok, Thailand","lat":"13.6682","lon":"100.6047","category":"place","type":"suburb"}]`))
	}))
	defer server.Close()

	geocoder := NewNominatimGeocoder(NominatimConfig{Endpoint: server.URL, UserAgent: "rbc-test", CountryCodes: "th", CacheTTL: time.Hour}, server.Client(), cache.Noop{})
	results, err := geocoder.Search(context.Background(), "Bang Na", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Latitude != 13.6682 || results[0].Longitude != 100.6047 {
		t.Fatalf("unexpected results: %#v", results)
	}
	if results[0].Status != "preliminary" || results[0].Source.Name != "OpenStreetMap Nominatim" || len(results[0].Assumptions) == 0 {
		t.Fatalf("provenance is incomplete: %#v", results[0])
	}
}

func TestNominatimRejectsShortQuery(t *testing.T) {
	t.Parallel()
	geocoder := NewNominatimGeocoder(NominatimConfig{}, nil, cache.Noop{})
	if _, err := geocoder.Search(context.Background(), "ab", 5); err != ErrInvalidGeocodingQuery {
		t.Fatalf("expected ErrInvalidGeocodingQuery, got %v", err)
	}
}

func TestNominatimReverseReturnsNearestRoadAndArea(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/reverse" || r.URL.Query().Get("lat") != "18.4171102" || r.URL.Query().Get("lon") != "103.4999855" {
			t.Fatalf("unexpected reverse request: %s", r.URL.String())
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"address":{"road":"ทางหลวงแผ่นดินหมายเลข 212","city_district":"เมืองบึงกาฬ","state":"บึงกาฬ"}}`))
	}))
	defer server.Close()

	geocoder := NewNominatimGeocoder(NominatimConfig{Endpoint: server.URL + "/search", UserAgent: "rbc-test", CountryCodes: "th", CacheTTL: time.Hour}, server.Client(), cache.Noop{})
	result, err := geocoder.Reverse(context.Background(), 18.4171102, 103.4999855)
	if err != nil {
		t.Fatal(err)
	}
	if result.Road != "ทางหลวงแผ่นดินหมายเลข 212" || result.District != "เมืองบึงกาฬ" || result.Province != "บึงกาฬ" {
		t.Fatalf("unexpected reverse result: %#v", result)
	}
}

func TestNominatimReverseUsesCountyForFloodDistrictMatching(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"address":{"road":"ถนนศรีธรรมไตรปิฎก","city_district":"ตำบลในเมือง","city":"เทศบาลนครพิษณุโลก","municipality":"อรัญญิก","county":"อำเภอเมืองพิษณุโลก","province":"จังหวัดพิษณุโลก"}}`))
	}))
	defer server.Close()
	geocoder := NewNominatimGeocoder(NominatimConfig{Endpoint: server.URL, CountryCodes: "th"}, server.Client(), cache.Noop{})
	result, err := geocoder.Reverse(context.Background(), 16.798026, 100.264773)
	if err != nil || result.District != "อำเภอเมืองพิษณุโลก" {
		t.Fatalf("must select administrative district rather than tambon: %+v, %v", result, err)
	}
	snapshot, err := loadDPMFloodSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	areas := make(map[string]dpmFloodAreaSummary)
	for _, area := range snapshot.Districts {
		areas[dpmFloodAreaKey(area.Province, area.District)] = area
	}
	area, found := findDPMFloodArea(areas, result.Province, result.District)
	if !found || len(area.ReportedYears) == 0 {
		t.Fatalf("real Phitsanulok address must match embedded DDPM history: %+v", result)
	}
}

func TestNominatimBangkokAddressMatchesEmbeddedFloodHistory(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"address":{"road":"ซอยลาดพร้าว 138","quarter":"แขวงคลองจั่น","suburb":"เขตบางกะปิ","city":"กรุงเทพมหานคร","country_code":"th"}}`))
	}))
	defer server.Close()
	geocoder := NewNominatimGeocoder(NominatimConfig{Endpoint: server.URL, CountryCodes: "th"}, server.Client(), cache.Noop{})
	lat, lon := 13.7682202, 100.6377021
	address, err := geocoder.Reverse(context.Background(), lat, lon)
	if err != nil || address.Province != "กรุงเทพมหานคร" || address.District != "เขตบางกะปิ" {
		t.Fatalf("Bangkok city/suburb address must resolve to province/khet: %+v, %v", address, err)
	}
	flood := NewDPMFloodHistoryProvider(DPMFloodHistoryConfig{}, server.Client(), cache.Noop{}, geocoder)
	observations, err := flood.Collect(context.Background(), domain.Site{Latitude: &lat, Longitude: &lon}, 3000)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range observations {
		if item.MetricType != "flood" {
			continue
		}
		var value dpmFloodMetricValue
		if err := json.Unmarshal(item.RawValue, &value); err != nil {
			t.Fatal(err)
		}
		if item.Status != domain.DataEstimated || value.District != "บางกะปิ" || len(value.ReportedFloodYears) != 1 || value.ReportedFloodYears[0] != 2566 {
			t.Fatalf("must return actual district history, not parcel evidence: %+v, %s", item, item.RawValue)
		}
		return
	}
	t.Fatal("flood observation missing")
}
