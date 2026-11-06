package provider

import (
	"context"
	"errors"
	"github.com/rbc/ev-station/apps/api/internal/domain"
	"github.com/rbc/ev-station/apps/api/internal/roads"
	"github.com/rbc/ev-station/apps/api/internal/telemetry"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
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

func TestCompetitionQueryUsesOneKilometreAndSharesCacheAcrossRoadRadii(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if err := r.ParseForm(); err != nil {
			t.Error(err)
		}
		if got := r.Form.Get("data"); got != buildOverpassContextQuery(13.7, 100.5, 1000) {
			t.Errorf("competition query must match the one-kilometre scoring radius: %s", got)
		}
		_, _ = w.Write([]byte(`{"elements":[]}`))
	}))
	defer server.Close()
	lat, lon := 13.7, 100.5
	provider := NewOSMProvider(OSMConfig{Endpoint: server.URL, LocalRoads: localRoadFixture{}}, server.Client(), &gatewayTestCache{values: map[string][]byte{}})
	for _, radius := range []int{1000, 2000, 3000} {
		if _, err := provider.Collect(context.Background(), domain.Site{Latitude: &lat, Longitude: &lon}, radius); err != nil {
			t.Fatal(err)
		}
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("expected one shared competitor request across three road radii, got %d", got)
	}
}

type deadlineRoadLookup struct {
	deadline chan time.Time
}

func (lookup deadlineRoadLookup) Nearby(ctx context.Context, lat, lon float64, radius int) (roads.Result, error) {
	deadline, _ := ctx.Deadline()
	lookup.deadline <- deadline
	return localRoadFixture{}.Nearby(ctx, lat, lon, radius)
}

func TestLocalLookupSharesCollectionBudget(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"elements":[]}`))
	}))
	defer server.Close()
	deadlines := make(chan time.Time, 1)
	budget := time.Second
	provider := NewOSMProvider(OSMConfig{Endpoint: server.URL, LocalRoads: deadlineRoadLookup{deadlines}, RequestBudget: budget}, server.Client(), nil)
	lat, lon := 13.7, 100.5
	started := time.Now()
	if _, err := provider.Collect(context.Background(), domain.Site{Latitude: &lat, Longitude: &lon}, 3000); err != nil {
		t.Fatal(err)
	}
	deadline := <-deadlines
	if deadline.IsZero() || deadline.Before(started) || deadline.After(started.Add(budget+100*time.Millisecond)) {
		t.Fatalf("local lookup must participate in the collection budget, got deadline %v", deadline)
	}
}

type localRoadError struct{ err error }

func (lookup localRoadError) Nearby(context.Context, float64, float64, int) (roads.Result, error) {
	return roads.Result{}, lookup.err
}

type roadLookupRecorder struct {
	mu     sync.Mutex
	events []telemetry.Event
}

func (r *roadLookupRecorder) Record(event telemetry.Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, event)
}

func (r *roadLookupRecorder) localEvents() []telemetry.Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	var events []telemetry.Event
	for _, event := range r.events {
		if event.Operation == "local_road_lookup" {
			events = append(events, event)
		}
	}
	return events
}

func TestLocalRoadLookupTelemetryExplainsRemoteFallback(t *testing.T) {
	for _, test := range []struct {
		name      string
		lookup    roads.Lookup
		status    string
		errorType string
		reason    string
		roadCalls int32
	}{
		{name: "available", lookup: localRoadFixture{}, status: "success"},
		{name: "snapshot unavailable", lookup: localRoadFixture{true}, status: "missing", errorType: "unavailable", reason: "local_snapshot_unavailable", roadCalls: 1},
		{name: "lookup timeout", lookup: localRoadError{context.DeadlineExceeded}, status: "error", errorType: "timeout", reason: "local_lookup_timeout", roadCalls: 1},
		{name: "database error", lookup: localRoadError{errors.New("synthetic database error")}, status: "error", errorType: "error", reason: "local_lookup_error", roadCalls: 1},
		{name: "not configured", status: "missing", reason: "local_roads_not_configured", roadCalls: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			recorder := &roadLookupRecorder{}
			var roadCalls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if err := r.ParseForm(); err != nil {
					t.Error(err)
				}
				if r.Form.Get("data") == buildOverpassRoadQuery(13.7, 100.5, 3000) {
					roadCalls.Add(1)
					if len(recorder.localEvents()) != 1 {
						t.Error("local lookup outcome must be recorded before remote fallback starts")
					}
				}
				_, _ = w.Write([]byte(`{"elements":[]}`))
			}))
			defer server.Close()
			provider := NewOSMProvider(OSMConfig{Endpoint: server.URL, LocalRoads: test.lookup}, server.Client(), nil)
			lat, lon := 13.7, 100.5
			ctx := telemetry.WithRecorder(telemetry.WithAnalysisID(context.Background(), "local-road-fixture"), recorder)
			if _, err := provider.Collect(ctx, domain.Site{Latitude: &lat, Longitude: &lon}, 3000); err != nil {
				t.Fatal(err)
			}
			if got := roadCalls.Load(); got != test.roadCalls {
				t.Errorf("remote road requests = %d, want %d", got, test.roadCalls)
			}
			events := recorder.localEvents()
			if len(events) != 1 {
				t.Fatalf("expected one local lookup event, got %d", len(events))
			}
			event := events[0]
			if event.Status != test.status || event.ErrorType != test.errorType || event.FallbackReason != test.reason || event.FallbackTriggered != (test.roadCalls > 0) {
				t.Fatalf("incorrect lookup/fallback telemetry: %+v", event)
			}
			if event.AnalysisID != "local-road-fixture" || event.Provider != "osm" || event.DurationMS < 0 || event.StartedAt.IsZero() || event.FinishedAt.Before(event.StartedAt) {
				t.Fatalf("missing lookup identity or timing: %+v", event)
			}
		})
	}
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
