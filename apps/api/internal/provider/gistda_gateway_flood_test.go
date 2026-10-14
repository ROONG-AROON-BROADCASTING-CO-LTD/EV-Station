package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rbc/ev-station/apps/api/internal/cache"
	"github.com/rbc/ev-station/apps/api/internal/domain"
)

func historyFixture(positive bool) []byte {
	detail := []map[string]any{}
	for year := 2011; year <= 2023; year++ {
		freq := 0
		if positive && year == 2011 {
			freq = 1
		}
		detail = append(detail, map[string]any{"year": fmt.Sprint(year), "freq": freq})
	}
	total := 0
	if positive {
		total = 1
	}
	b, _ := json.Marshal([]map[string]any{{"total": total, "detail": detail}})
	return b
}
func TestGatewayHistoryContractAndMissingRecords(t *testing.T) {
	for _, b := range [][]byte{[]byte(strings.Replace(string(historyFixture(true)), `"total":1`, `"total":9`, 1)), []byte(strings.Replace(string(historyFixture(true)), `"year":"2023"`, `"year":"2011"`, 1))} {
		if parseGatewayHistory(b, &gatewayFloodEvidence{}) == "" {
			t.Fatal("accepted inconsistent annual data")
		}
	}
	for _, b := range [][]byte{[]byte(`null`), []byte(`{"error":"denied"}`), []byte(`[{"total":1,"detail":[{"year":"2023","freq":1}]}]`)} {
		if parseGatewayHistory(b, &gatewayFloodEvidence{}) == "" {
			t.Fatal("accepted incomplete schema", string(b))
		}
	}
	for _, positive := range []bool{true, false} {
		e := gatewayFloodEvidence{}
		if parseGatewayHistory(historyFixture(positive), &e) != "" || !e.Complete || (positive && len(e.YearsObserved) != 1) || (!positive && len(e.YearsObserved) != 0) {
			t.Fatal(e)
		}
	}
	e := gatewayFloodEvidence{}
	if parseGatewayHistory([]byte(`[]`), &e) != "" || e.Complete || e.Reason != "no_record" {
		t.Fatal(e)
	}
	e = gatewayFloodEvidence{}
	if parseGatewayHistory([]byte(`{"result":"not found"}`), &e) != "" || e.Complete || e.Reason != "no_record" {
		t.Fatal("live no-record response must not establish complete history", e)
	}
}

func TestGatewayFeatureLatestGeometryCompletenessAndSecretCache(t *testing.T) {
	fixture := `{"type":"FeatureCollection","features":[{"type":"Feature","geometry":{"type":"Polygon","coordinates":[[[100,13],[101,13],[101,14],[100,14],[100,13]]]}}],"numberMatched":1,"links":[{"rel":"self","href":"https://example.test?api_key=private-test"}],"timeStamp":"2026-10-12T00:00:00Z"}`
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("api_key") != "private-test" || r.URL.Query().Get("bbox") == "" || r.URL.Query().Get("limit") != "1000" {
			t.Error("wrong feature query")
		}
		w.Write([]byte(fixture))
	}))
	defer s.Close()
	c := &gatewayTestCache{values: map[string][]byte{}}
	p := NewGISTDAGatewayFloodProvider(GISTDAGatewayFloodConfig{BaseURL: s.URL, LatestURL: s.URL, APIKey: "private-test"}, s.Client(), c)
	b, _, reason := p.fetch(context.Background(), "flood-extent-1day", 13.7, 100.5, time.Hour)
	if reason != "" || bytes.Contains(b, []byte("private-test")) {
		t.Fatal("failed or exposed key", reason)
	}
	for _, v := range c.values {
		if bytes.Contains(v, []byte("private-test")) {
			t.Fatal("cached credential")
		}
	}
	e := gatewayFloodEvidence{}
	if p.parseLatest(b, &e, time.Now(), 13.7, 100.5) != "" || e.Result != "found" || !e.Complete || e.Reason != "observation_date_unknown" || len(e.ObservationDates) != 0 {
		t.Fatal(e)
	}
	e = gatewayFloodEvidence{}
	if p.parseLatest(b, &e, time.Now(), 15, 102) != "" || e.Result != "not found" {
		t.Fatal("outside polygon", e)
	}
	partial := bytes.ReplaceAll(b, []byte(`"numberMatched":1`), []byte(`"numberMatched":2`))
	e = gatewayFloodEvidence{}
	if p.parseLatest(partial, &e, time.Now(), 15, 102) != "" || e.Reason != "incomplete_response" || e.Result != "unknown" {
		t.Fatal("truncated results became negative")
	}
	if p.parseLatest([]byte(`{"error":"unavailable"}`), &gatewayFloodEvidence{}, time.Now(), 13.7, 100.5) != "invalid_response" {
		t.Fatal("error became no flood")
	}
	e = gatewayFloodEvidence{}
	if p.parseLatest([]byte(`{"type":"FeatureCollection","features":[],"numberMatched":0}`), &e, time.Now(), 13.7, 100.5) != "" || e.Result != "unknown" || e.Reason != "no_observations_in_window" {
		t.Fatal("valid empty response rejected")
	}
}

type gatewayTestCache struct {
	cache.Noop
	mu     sync.Mutex
	values map[string][]byte
}

func (c *gatewayTestCache) Get(_ context.Context, k string) ([]byte, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	b, ok := c.values[k]
	return b, ok, nil
}
func (c *gatewayTestCache) Set(_ context.Context, k string, b []byte, _ time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.values[k] = b
	return nil
}
func TestGatewayCachePreservesOriginalRetrieval(t *testing.T) {
	var calls atomic.Int32
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if strings.HasSuffix(r.URL.Path, "flood-recurrence") {
			w.Write(historyFixture(true))
		} else {
			w.Write([]byte(`{"result":"not found","data":[]}`))
		}
	}))
	defer s.Close()
	c := &gatewayTestCache{values: map[string][]byte{}}
	p := NewGISTDAGatewayFloodProvider(GISTDAGatewayFloodConfig{BaseURL: s.URL, APIKey: "private-test"}, s.Client(), c)
	lat, lon := 13.7, 100.5
	site := domain.Site{Latitude: &lat, Longitude: &lon}
	first, _ := p.Collect(context.Background(), site, 3000)
	second, _ := p.Collect(context.Background(), site, 3000)
	var a, b gatewayFloodValue
	json.Unmarshal(first[0].RawValue, &a)
	json.Unmarshal(second[0].RawValue, &b)
	if calls.Load() != 2 || !a.Historical.RetrievedAt.Equal(*b.Historical.RetrievedAt) || !first[0].Source.RetrievedAt.Equal(second[0].Source.RetrievedAt) {
		t.Fatal("cache refreshed source time or repeated queries")
	}
	for key := range c.values {
		if strings.Contains(key, "private-test") {
			t.Fatal("secret in cache key")
		}
	}
}
func TestGatewayLatestPreservesDatesAndRejectsErrors(t *testing.T) {
	now := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	e := gatewayFloodEvidence{}
	if parseGatewayLatest([]byte(`{"result":"found","data":[{"update":"2024-12-18"}]}`), &e, now) != "" || e.Reason != "stale_data" {
		t.Fatal(e)
	}
	e = gatewayFloodEvidence{}
	if parseGatewayLatest([]byte(`{"result":"not found","data":[]}`), &e, now) != "" || e.Reason != "observation_date_unknown" {
		t.Fatal(e)
	}
	for _, body := range []string{`{}`, `{"result":"found","data":[]}`, `{"result":"not found","data":[{"update":"2026-10-06"}]}`, `{"result":"found","data":[{"update":"2027-01-01"}]}`} {
		if parseGatewayLatest([]byte(body), &gatewayFloodEvidence{}, now) == "" {
			t.Fatal(body)
		}
	}
}
func TestGatewayPointQueriesAndIndependentFailure(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("api_key") != "private-test" || r.URL.Query().Has("key") || r.URL.Query().Get("lat") != "13.7" || r.URL.Query().Get("lon") != "100.5" {
			t.Error("wrong contract")
		}
		if strings.HasSuffix(r.URL.Path, "flood-recurrence") {
			w.Write(historyFixture(true))
		} else {
			w.WriteHeader(403)
		}
	}))
	defer s.Close()
	lat, lon := 13.7, 100.5
	p := NewGISTDAGatewayFloodProvider(GISTDAGatewayFloodConfig{BaseURL: s.URL, APIKey: "private-test"}, s.Client(), nil)
	obs, err := p.Collect(context.Background(), domain.Site{Latitude: &lat, Longitude: &lon}, 3000)
	if err != nil || len(obs) != 1 || strings.Contains(string(obs[0].RawValue), "private-test") {
		t.Fatal("invalid or leaked evidence")
	}
	var v gatewayFloodValue
	json.Unmarshal(obs[0].RawValue, &v)
	if v.Historical.Status != "available" || v.Latest.Status != "missing" || v.Latest.Reason != "unauthorized" {
		t.Fatal(v)
	}
	p = NewGISTDAGatewayFloodProvider(GISTDAGatewayFloodConfig{}, nil, nil)
	obs, _ = p.Collect(context.Background(), domain.Site{Latitude: &lat, Longitude: &lon}, 3000)
	json.Unmarshal(obs[0].RawValue, &v)
	if v.Historical.Reason != "not_configured" || v.Latest.Reason != "not_configured" {
		t.Fatal(v)
	}
}
