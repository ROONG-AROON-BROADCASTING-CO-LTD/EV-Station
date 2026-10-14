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

type provinceGeocoder struct{ province string }

func (g provinceGeocoder) Reverse(context.Context, float64, float64) (ReverseGeocodingResult, error) {
	return ReverseGeocodingResult{Province: "จังหวัด" + g.province}, nil
}

// Tests every administrative name through the real collection path using
// synthetic source rows; this is not a claim of live geographic coverage.
func TestDLTCollectionAll77Provinces(t *testing.T) {
	if len(dltProvinces) != 77 {
		t.Fatalf("province inventory: %d", len(dltProvinces))
	}
	var csv strings.Builder
	csv.WriteString("ปี,ประเภทรถ,จังหวัด,ชนิดเชื้อเพลิง,จำนวน\n")
	for province := range dltProvinces {
		fmt.Fprintf(&csv, "2569,รถยนต์,จังหวัด%s,ไฟฟ้า,123\n", province)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/previous" {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(csv.String()))
	}))
	defer server.Close()
	for province := range dltProvinces {
		t.Run(province, func(t *testing.T) {
			p := NewDLTEVRegistrationProvider(DLTEVRegistrationConfig{CSVURL: server.URL, PreviousCSVURL: server.URL + "/previous", Geocoder: provinceGeocoder{province}}, server.Client(), cache.Noop{})
			lat, lon := 13.0, 100.0 // Synthetic pin: the mock returns the tested province.
			observations, err := p.Collect(context.Background(), domain.Site{Latitude: &lat, Longitude: &lon}, 3000)
			if err != nil {
				t.Fatal(err)
			}
			for _, o := range observations {
				if o.MetricType == "ev_demand" {
					var v dltEVValue
					if json.Unmarshal(o.RawValue, &v) != nil || v.Province != province || v.RegisteredBEV != 123 || o.Status != domain.DataVerified {
						t.Fatalf("bad provincial evidence: %+v", o)
					}
					if v.ChangePercent != nil {
						t.Fatal("unavailable history must not create trend")
					}
					return
				}
			}
			t.Fatal("missing EV observation")
		})
	}
}

func TestPreviousDLTTimeoutPreservesCurrentEvidence(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/previous" {
			<-r.Context().Done()
			return
		}
		_, _ = w.Write([]byte("ปี,รถ,จังหวัด,เชื้อเพลิง,จำนวน\n2569,รถยนต์,เชียงใหม่,ไฟฟ้า,123\n"))
	}))
	defer server.Close()
	p := NewDLTEVRegistrationProvider(DLTEVRegistrationConfig{CSVURL: server.URL, PreviousCSVURL: server.URL + "/previous", Geocoder: provinceGeocoder{"เชียงใหม่"}}, server.Client(), cache.Noop{})
	lat, lon := 18.0, 99.0
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	obs, err := NewCompositeProvider(p).Collect(ctx, domain.Site{Latitude: &lat, Longitude: &lon}, 3000)
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range obs {
		if o.MetricType == "ev_demand" && o.Status == domain.DataVerified {
			return
		}
	}
	t.Fatal("current evidence lost while waiting for optional history")
}
