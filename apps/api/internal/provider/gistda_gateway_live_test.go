package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/rbc/ev-station/apps/api/internal/domain"
)

type liveFloodTrace struct{ t *testing.T }

func (trace liveFloodTrace) RoundTrip(r *http.Request) (*http.Response, error) {
	start := time.Now()
	response, err := http.DefaultTransport.RoundTrip(r)
	code := 0
	if response != nil {
		code = response.StatusCode
	}
	trace.t.Logf("request path=%s status=%d duration=%s failed=%v", r.URL.Path, code, time.Since(start), err != nil)
	return response, err
}

func TestGatewayLiveAnnual2026(t *testing.T) {
	if os.Getenv("RBC_GISTDA_LIVE_CHECK") != "1" {
		t.Skip("live check not requested")
	}
	p := NewGISTDAGatewayFloodProvider(GISTDAGatewayFloodConfig{APIKey: os.Getenv("GISTDA_GI_SERVICE_API_KEY")}, &http.Client{Timeout: 18 * time.Second, Transport: liveFloodTrace{t}}, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	e := p.collectAnnualYear(ctx, p.config.ArchiveBaseURL+"/catalogs/flood_yearly_2026", 2026, 14.353, 100.5692, 3000, ReverseGeocodingResult{})
	if e.Status != "available" {
		t.Fatalf("2026 annual source unavailable: %s", e.Reason)
	}
	t.Logf("2026 records=%d complete=%v partialYear=%v", e.RecordCount, e.Complete, e.PartialYear)
}

// Explicit opt-in: normal unit tests never use credentials or public APIs.
func TestGatewayLiveOneDayAndHistory(t *testing.T) {
	if os.Getenv("RBC_GISTDA_LIVE_CHECK") != "1" {
		t.Skip("live check not requested")
	}
	key := os.Getenv("GISTDA_GI_SERVICE_API_KEY")
	if key == "" {
		t.Fatal("GI Service key not configured")
	}
	geocoder := NewNominatimGeocoder(NominatimConfig{CountryCodes: "th", UserAgent: "RBC-EV-Station/1.0"}, nil, nil)
	p := NewGISTDAGatewayFloodProvider(GISTDAGatewayFloodConfig{APIKey: key, UserAgent: "RBC-EV-Station/1.0", Geocoder: geocoder}, nil, nil)
	lat, lon := 14.353, 100.5692
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
	defer cancel()
	observations, err := p.Collect(ctx, domain.Site{Latitude: &lat, Longitude: &lon}, 3000)
	if err != nil || len(observations) != 1 {
		t.Fatal("collection failed")
	}
	var result gatewayFloodValue
	if json.Unmarshal(observations[0].RawValue, &result) != nil {
		t.Fatal("invalid evidence")
	}
	if result.Historical.Status != "available" || !result.Historical.Complete {
		t.Fatalf("history unavailable: %s", result.Historical.Reason)
	}
	if len(result.Windows) != 4 {
		t.Fatal("not all recent windows were checked")
	}
	for _, w := range result.Windows {
		if w.Evidence.RetrievedAt == nil {
			t.Fatalf("%d-day source was not checked: %s", w.Days, w.Evidence.Reason)
		}
		t.Logf("%d days: %s; records=%v; dates=%v", w.Days, w.Evidence.Result, w.Evidence.FeatureCount, w.Evidence.ObservationDates)
	}
	if len(result.Annual) < 16 {
		t.Fatalf("annual inventory incomplete: %d years", len(result.Annual))
	}
	for _, a := range result.Annual {
		t.Logf("year %d: %s; records=%d; district=%s; code=%d; reason=%s", a.Year, a.Status, a.RecordCount, a.District, a.DistrictID, a.Reason)
		if a.Status != "available" {
			t.Errorf("annual year %d was not fully checked: %s", a.Year, a.Reason)
		}
	}
}
