package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

type annualGeocoder struct{}

func (annualGeocoder) Reverse(context.Context, float64, float64) (ReverseGeocodingResult, error) {
	return ReverseGeocodingResult{Province: "กรุงเทพมหานคร", District: "ห้วยขวาง"}, nil
}

func floodTestFeature(id string, properties map[string]any) map[string]any {
	return map[string]any{"type": "Feature", "id": id, "properties": properties, "geometry": map[string]any{"type": "Polygon", "coordinates": [][][]float64{{{100, 13}, {101, 13}, {101, 14}, {100, 14}, {100, 13}}}}}
}

func TestAnnualArchiveIgnoresStaleFootprintsAndUsesFloodAreaUnits(t *testing.T) {
	var s *httptest.Server
	s = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("api_key") != "private-test" {
			t.Error("missing Gateway authentication")
		}
		var body any
		switch r.URL.Path {
		case "/stac/catalogs/flood_yearly":
			body = map[string]any{"type": "Catalog", "links": []map[string]string{{"rel": "child", "href": s.URL + "/stac/catalogs/flood_yearly_2011"}, {"rel": "child", "href": s.URL + "/stac/catalogs/flood_yearly_2026"}}}
		case "/stac/catalogs/flood_yearly_2011", "/stac/catalogs/flood_yearly_2026":
			year := strings.TrimPrefix(r.URL.Path, "/stac/catalogs/flood_yearly_")
			body = map[string]any{"type": "Catalog", "links": []map[string]string{{"rel": "child", "href": s.URL + "/stac/collections/annual_" + year}}}
		case "/stac/search":
			if r.URL.Query().Has("bbox") {
				t.Error("stale STAC bounds must not suppress actual data")
			}
			year := strings.TrimPrefix(r.URL.Query().Get("collections"), "annual_")
			body = map[string]any{"type": "FeatureCollection", "context": map[string]int{"matched": 1}, "features": []any{map[string]any{"assets": map[string]any{"data": map[string]string{"type": "application/geo+json", "href": s.URL + "/data/" + year + "?api_key=private-test"}}}}}
		case "/data/2011", "/data/2026":
			if r.URL.Query().Get("ap_idn") != "1017" || r.URL.Query().Has("bbox") {
				t.Error("verified district should use the administrative filter")
			}
			p := map[string]any{"pv_tn": "กรุงเทพมหานคร", "ap_tn": "เขตห้วยขวาง", "tb_tn": "สามเสนนอก", "tb_idn": 101701, "area_rai": 9999}
			if r.URL.Path == "/data/2011" {
				p["year"] = "2011"
				p["area_rai"] = 5
			} else {
				p["flood_area"] = 10
				p["ap_idn"] = 1017
				p["pv_idn"] = 10
				p["ap_tn"] = "encoding-corrupted-label"
				p["pv_tn"] = "encoding-corrupted-label"
			}
			body = map[string]any{"type": "FeatureCollection", "numberMatched": 1, "features": []any{floodTestFeature("one", p)}}
		default:
			w.WriteHeader(404)
			return
		}
		_ = json.NewEncoder(w).Encode(body)
	}))
	defer s.Close()
	c := &gatewayTestCache{values: map[string][]byte{}}
	p := NewGISTDAGatewayFloodProvider(GISTDAGatewayFloodConfig{BaseURL: s.URL, ArchiveBaseURL: s.URL + "/stac", APIKey: "private-test", Geocoder: annualGeocoder{}}, s.Client(), c)
	first := p.collectAnnual(context.Background(), 13.7, 100.5, 3000)
	second := p.collectAnnual(context.Background(), 13.7, 100.5, 3000)
	if len(first) != 2 || first[0].Year != 2026 || !first[0].PartialYear {
		t.Fatal(first)
	}
	for i, e := range first {
		want := 10.0
		if e.Year == 2011 {
			want = 5
		}
		if e.Status != "available" || e.RecordCount != 1 || e.ReportedAreaValue == nil || *e.ReportedAreaValue != want || e.Scope != "district_summary" || e.DistrictID != 1017 || !e.Complete {
			t.Fatal("incorrect scope or area", e)
		}
		if e.Year == 2026 && e.ReportedAreaRai != nil {
			t.Fatal("invented area units")
		}
		if !e.RetrievedAt.Equal(*second[i].RetrievedAt) {
			t.Fatal("cache changed retrieval time")
		}
	}
	for _, b := range c.values {
		if strings.Contains(string(b), "private-test") {
			t.Fatal("credential persisted")
		}
	}
	if _, _, reason := p.archiveJSON(context.Background(), "https://example.invalid/steal", time.Hour); reason != "invalid_endpoint" {
		t.Fatal("external asset destination accepted")
	}
}

func TestAllRecentWindowsAndPaginationPreserve2026Evidence(t *testing.T) {
	duplicate := false
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := "one"
		if r.URL.Query().Get("offset") == "1" && !duplicate {
			id = "two"
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"type": "FeatureCollection", "numberMatched": 2, "features": []any{floodTestFeature(id, map[string]any{"file_name": "S1C_20261010_0550", "_createdAt": "2026-10-12T00:00:00Z"})}})
	}))
	defer s.Close()
	p := NewGISTDAGatewayFloodProvider(GISTDAGatewayFloodConfig{BaseURL: s.URL, LatestURL: s.URL + "/flood/1day", APIKey: "private-test"}, s.Client(), nil)
	u, _ := url.Parse(s.URL + "/flood/7days?offset=0&limit=1&api_key=private-test")
	first, _ := json.Marshal(map[string]any{"type": "FeatureCollection", "numberMatched": 2, "features": []any{floodTestFeature("one", map[string]any{"file_name": "S1C_20261010_0550"})}})
	all := p.collectFeaturePages(context.Background(), u, first)
	e := gatewayFloodEvidence{WindowDays: 7}
	now := time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)
	if p.parseLatest(all, &e, now, 13.7, 100.5) != "" || !e.Complete || *e.FeatureCount != 2 || e.DateBasis != "satellite_product_filename" || len(e.ObservationDates) != 1 || e.ObservationDates[0] != "2026-10-10" {
		t.Fatal(e)
	}
	duplicate = true
	partial := p.collectFeaturePages(context.Background(), u, first)
	e = gatewayFloodEvidence{WindowDays: 7}
	if p.parseLatest(partial, &e, now, 15, 102) != "" || e.Result != "unknown" || e.Complete {
		t.Fatal("duplicate page created negative result", e)
	}
	props := map[string]any{}
	for y := 2011; y <= 2024; y++ {
		props[fmt.Sprintf("y_%d", y)] = 0
	}
	props["y_2024"] = 1
	mapped, _ := json.Marshal(map[string]any{"type": "FeatureCollection", "numberMatched": 1, "features": []any{floodTestFeature("archive", props)}})
	e = gatewayFloodEvidence{}
	if parseGatewayMappedHistory(mapped, &e, 13.7, 100.5) != "" || e.PeriodEndYear != 2024 || len(e.YearsObserved) != 1 || e.YearsObserved[0] != 2024 {
		t.Fatal("2024 history lost", e)
	}
	props["y_2024"] = nil
	mapped, _ = json.Marshal(map[string]any{"type": "FeatureCollection", "numberMatched": 1, "features": []any{floodTestFeature("archive", props)}})
	if parseGatewayMappedHistory(mapped, &gatewayFloodEvidence{}, 13.7, 100.5) != "invalid_response" {
		t.Fatal("null annual observation became zero")
	}
}
