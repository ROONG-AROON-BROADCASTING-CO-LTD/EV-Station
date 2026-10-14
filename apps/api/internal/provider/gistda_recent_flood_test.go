package provider

import (
	"context"
	"encoding/json"
	"github.com/rbc/ev-station/apps/api/internal/domain"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRecentFloodMissingCredential(t *testing.T) {
	p := NewGISTDARecentFloodProvider(GISTDARecentFloodConfig{}, nil, nil)
	lat, lon := 13.7, 100.5
	observations, err := p.Collect(context.Background(), domain.Site{Latitude: &lat, Longitude: &lon}, 3000)
	if err != nil || len(observations) != 1 {
		t.Fatal(observations, err)
	}
	var v recentFloodValue
	json.Unmarshal(observations[0].RawValue, &v)
	if observations[0].Status != domain.DataMissing || len(v.Windows) != 4 || v.ScoringEligible {
		t.Fatal(v)
	}
	for _, w := range v.Windows {
		if w.Reason != "not_configured" || w.FeatureCount != nil {
			t.Fatal(w)
		}
	}
}

func TestRecentFloodWindowFailuresAreNotEmptyResults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("API-Key") != "test-key" || r.URL.Query().Get("api_key") != "" || r.URL.Query().Get("bbox") == "" {
			t.Error("invalid authentication or spatial request")
		}
		switch r.URL.Path {
		case "/features/flood/1day":
			w.Write([]byte(`{"type":"FeatureCollection","features":[]}`))
		case "/features/flood/3days":
			w.WriteHeader(401)
		case "/features/flood/7days":
			w.Write([]byte(`{"error":"upstream failure"}`))
		default:
			w.WriteHeader(429)
		}
	}))
	defer server.Close()
	p := NewGISTDARecentFloodProvider(GISTDARecentFloodConfig{BaseURL: server.URL, APIKey: "test-key"}, server.Client(), nil)
	lat, lon := 13.7, 100.5
	observations, _ := p.Collect(context.Background(), domain.Site{Latitude: &lat, Longitude: &lon}, 3000)
	var v recentFloodValue
	json.Unmarshal(observations[0].RawValue, &v)
	if v.Windows[0].Status != "available" || *v.Windows[0].FeatureCount != 0 || !v.Windows[0].Complete {
		t.Fatal(v)
	}
	for i, reason := range []string{"unauthorized", "invalid_response", "rate_limited"} {
		if v.Windows[i+1].Reason != reason || v.Windows[i+1].FeatureCount != nil {
			t.Fatal(v)
		}
	}
}

func TestRecentFloodGeometryAndPagination(t *testing.T) {
	payload := []byte(`{"type":"FeatureCollection","numberMatched":2,"features":[{"type":"Feature","geometry":{"type":"Polygon","coordinates":[[[100,13],[101,13],[101,14],[100,14],[100,13]],[[100.4,13.4],[100.6,13.4],[100.6,13.6],[100.4,13.6],[100.4,13.4]]]}}]}`)
	count, overlap, complete, err := parseRecentFlood(payload, 100.5, 13.5)
	if err != nil || count != 1 || overlap != 0 || complete {
		t.Fatal(count, overlap, complete, err)
	}
	_, overlap, _, err = parseRecentFlood(payload, 100.2, 13.2)
	if err != nil || overlap != 1 {
		t.Fatal(overlap, err)
	}
	for _, bad := range []string{`{}`, `{"type":"FeatureCollection","features":null}`, `{"type":"FeatureCollection","features":[{"type":"Feature","geometry":{"type":"Point","coordinates":[100,13]}}]}`} {
		if _, _, _, err := parseRecentFlood([]byte(bad), 100, 13); err == nil {
			t.Fatal("accepted malformed collection", bad)
		}
	}
}
