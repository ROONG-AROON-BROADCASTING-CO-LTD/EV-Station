package telemetry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/rbc/ev-station/apps/api/internal/cache"
)

type capturedRecorder struct {
	mu     sync.Mutex
	events []Event
}

func (r *capturedRecorder) Record(event Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, event)
}

func (r *capturedRecorder) Events() []Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Event(nil), r.events...)
}

func TestHTTPClientRecordsSafeRequestMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	recorder := &capturedRecorder{}
	client := NewHTTPClient("google_places", time.Second, recorder)
	ctx := WithCategory(WithOperation(WithAnalysisID(context.Background(), "analysis-123"), "google_places_search"), "restaurant")
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/nearby?key=secret&lat=13.7", nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	_ = response.Body.Close()

	events := recorder.Events()
	if len(events) != 1 {
		t.Fatalf("event count = %d, want 1", len(events))
	}
	event := events[0]
	if event.AnalysisID != "analysis-123" || event.Provider != "google_places" || event.Category != "restaurant" {
		t.Fatalf("unexpected telemetry event: %#v", event)
	}
	if event.Endpoint != request.URL.Host || event.HTTPStatus != http.StatusOK || event.Status != "success" {
		t.Fatalf("unexpected HTTP metadata: %#v", event)
	}
	if event.DurationMS < 0 {
		t.Fatalf("duration must be non-negative: %#v", event)
	}
}

func TestWrappedCacheRecordsMissWithoutLoggingKey(t *testing.T) {
	recorder := &capturedRecorder{}
	wrapped := WrapCache(cache.Noop{}, "osm", recorder)
	ctx := WithOperation(WithAnalysisID(context.Background(), "analysis-456"), "overpass_roads")
	if _, hit, err := wrapped.Get(ctx, "sensitive-cache-key"); err != nil || hit {
		t.Fatalf("cache result = hit:%t error:%v", hit, err)
	}

	events := recorder.Events()
	if len(events) != 1 {
		t.Fatalf("event count = %d, want 1", len(events))
	}
	event := events[0]
	if event.Provider != "osm" || event.Operation != "cache_get" || event.CacheHit == nil || *event.CacheHit {
		t.Fatalf("unexpected cache telemetry: %#v", event)
	}
	if event.CacheKeyFingerprint == "" || event.CacheMissReason != "not_found" {
		t.Fatalf("cache miss must expose only a safe fingerprint and reason: %#v", event)
	}

	if err := wrapped.Set(ctx, "sensitive-cache-key", []byte("private payload"), time.Hour); err != nil {
		t.Fatal(err)
	}
	events = recorder.Events()
	set := events[len(events)-1]
	if set.Operation != "cache_set" || set.Status != "success" || set.CacheTTLSeconds != 3600 || set.CacheKeyFingerprint == "sensitive-cache-key" {
		t.Fatalf("unexpected cache-set telemetry: %#v", set)
	}
}
