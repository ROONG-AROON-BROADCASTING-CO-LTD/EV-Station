package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/rbc/ev-station/apps/api/internal/cache"
	"github.com/rbc/ev-station/apps/api/internal/domain"
)

func TestTrafficRoadsRejectErrorAndFetchAllPages(t *testing.T) {
	for _, mode := range []string{"error", "malformed", "pages"} {
		t.Run(mode, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if mode == "error" {
					_, _ = w.Write([]byte(`{"error":{"code":400,"message":"bad query"}}`))
					return
				}
				if mode == "malformed" {
					_, _ = w.Write([]byte(`{}`))
					return
				}
				if r.URL.Query().Get("resultOffset") == "0" {
					_, _ = w.Write([]byte(`{"features":[{"id":1}],"exceededTransferLimit":true}`))
				} else {
					_, _ = w.Write([]byte(`{"features":[{"id":2}]}`))
				}
			}))
			defer server.Close()
			features, err := fetchTrafficRoads[map[string]int](context.Background(), server.Client(), cache.Noop{}, server.URL, "*", "OBJECTID", "test", time.Hour, 13.7, 100.5, 3000)
			if mode != "pages" && err == nil {
				t.Fatal("an error must not become no matching roads")
			}
			if mode == "pages" && (err != nil || len(features) != 2 || calls != 2) {
				t.Fatalf("incomplete pagination: %v %v %d", features, err, calls)
			}
		})
	}
}

func TestTrafficPriorityAndFailureEvidence(t *testing.T) {
	official := Observation{MetricType: "traffic", Status: domain.DataVerified, RawValue: json.RawMessage(`{"aadt":500,"dataYear":2568,"distanceToRoadMeters":200}`), Source: domain.DataSource{Name: "DRR"}}
	distant := official
	distant.RawValue = json.RawMessage(`{"aadt":9999,"dataYear":2568,"distanceToRoadMeters":2000}`)
	local := Observation{MetricType: "traffic", Status: domain.DataPreliminary, RawValue: json.RawMessage(`{"assessmentType":"local_traffic_count","value":99,"unit":"PCU/hour"}`)}
	estimate := Observation{MetricType: "traffic", Status: domain.DataEstimated, RawValue: json.RawMessage(`{"assessmentType":"osm_road_traffic_potential"}`)}
	if !betterTraffic(official, local) || !betterTraffic(local, estimate) || !betterTraffic(official, distant) || betterTraffic(estimate, official) {
		t.Fatal("incorrect evidence ordering")
	}
	failure := Observation{MetricType: "traffic", Status: domain.DataMissing, Source: domain.DataSource{Name: "DOH", Type: "official_open_data_and_gis"}, Assumptions: []string{"Road layer could not be queried."}}
	result := attachTrafficChecks(estimate, []Observation{failure})
	if !strings.Contains(string(result.RawValue), `"outcome":"unavailable"`) {
		t.Fatal("retrieval failure was hidden")
	}
}

func TestLocalTrafficKeepsUnitsAndRadius(t *testing.T) {
	payload := "station,province,latitude,longitude,value,unit,survey_date,survey_hours,source,reference_url\nTest,Chiang Mai,18.7883,98.9853,500,PCU/hour,2025-01-01,2,Survey,https://example.org/report\n"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(payload)) }))
	defer server.Close()
	p := LocalTrafficProvider{URL: server.URL, Client: server.Client()}
	lat, lon := 18.7883, 98.9853
	result, err := p.Collect(context.Background(), domain.Site{Latitude: &lat, Longitude: &lon}, 3000)
	if err != nil || len(result) != 1 || !strings.Contains(string(result[0].RawValue), `"unit":"PCU/hour"`) {
		t.Fatalf("invalid result %v %v", result, err)
	}
	lat = 18.81
	result, _ = p.Collect(context.Background(), domain.Site{Latitude: &lat, Longitude: &lon}, 3000)
	if result[0].Status != domain.DataMissing {
		t.Fatal("station outside 1 km accepted")
	}
	if _, err = parseLocalTraffic([]byte(strings.Replace(payload, "18.7883", "NaN", 1))); err == nil {
		t.Fatal("NaN accepted")
	}
}

// Opt-in audit makes only public DOH/DRR requests; no Gemini or paid map calls.
func TestTrafficNationalLive(t *testing.T) {
	if os.Getenv("TRAFFIC_LIVE_AUDIT") != "1" {
		t.Skip("set TRAFFIC_LIVE_AUDIT=1 to query public sources")
	}
	points := []struct {
		name     string
		lat, lon float64
	}{{"Bangkok", 13.771293, 100.582707}, {"Ang Thong", 14.588020, 100.310136}, {"Chiang Mai", 18.7883, 98.9853}, {"Khon Kaen", 16.4419, 102.8359}, {"Songkhla", 7.0084, 100.4747}}
	for _, point := range points {
		t.Run(point.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			p := NewCompositeProvider(NewDOHAADTProvider(DOHAADTConfig{}, nil, nil), NewDRRAADTProvider(DRRAADTConfig{}, nil, nil))
			results, err := p.Collect(ctx, domain.Site{Latitude: &point.lat, Longitude: &point.lon}, 3000)
			if err != nil {
				t.Fatal(err)
			}
			for _, item := range results {
				if item.MetricType == "traffic" {
					t.Logf("%s %s %s", point.name, item.Status, item.RawValue)
					if strings.Contains(string(item.RawValue), `"outcome":"unavailable"`) {
						t.Error("upstream retrieval failed; inspect before deployment")
					}
				}
			}
		})
	}
}
