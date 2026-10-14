package provider

import (
	"context"
	"github.com/rbc/ev-station/apps/api/internal/domain"
	"github.com/rbc/ev-station/apps/api/internal/roads"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type localRoadFixture struct{ unavailable bool }

func (l localRoadFixture) Nearby(context.Context, float64, float64, int) (roads.Result, error) {
	if l.unavailable {
		return roads.Result{}, roads.ErrUnavailable
	}
	distance := 12.5
	return roads.Result{Counts: map[string]int{"primary": 1}, Count: 1, Distance: &distance, Class: "primary", Name: "Test road", SourceTime: time.Now(), ImportedAt: time.Now()}, nil
}
func TestLocalRoadsAvoidsRemoteRoadQueryAndKeepsPreliminaryStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		if r.Form.Get("data") == buildOverpassRoadQuery(13.7, 100.5, 3000) {
			t.Error("remote roads requested despite local evidence")
		}
		w.Write([]byte(`{"elements":[]}`))
	}))
	defer server.Close()
	lat, lon := 13.7, 100.5
	p := NewOSMProvider(OSMConfig{Endpoint: server.URL, LocalRoads: localRoadFixture{}}, server.Client(), nil)
	items, err := p.Collect(context.Background(), domain.Site{Latitude: &lat, Longitude: &lon}, 3000)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range items {
		if item.MetricType == "road_accessibility" && (item.Source.Type != "local_open_data_snapshot" || item.Status != domain.DataPreliminary) {
			t.Fatal("local provenance or evidence status lost")
		}
	}
}

func TestMissingLocalRoadsFallsBackToRemote(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"elements":[{"type":"way","id":1,"tags":{"highway":"primary"},"geometry":[{"lat":13.7,"lon":100.5}]}]}`))
	}))
	defer server.Close()
	p := NewOSMProvider(OSMConfig{Endpoint: server.URL, LocalRoads: localRoadFixture{true}}, server.Client(), nil)
	lat, lon := 13.7, 100.5
	items, err := p.Collect(context.Background(), domain.Site{Latitude: &lat, Longitude: &lon}, 3000)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range items {
		if item.MetricType == "road_accessibility" && item.Source.Type != "open_data_api" {
			t.Fatal("remote fallback not used")
		}
	}
}
