package provider

import (
	"context"
	"encoding/json"
	"github.com/rbc/ev-station/apps/api/internal/domain"
	"testing"
)

type recentFloodStub []Observation

func (s recentFloodStub) Collect(context.Context, domain.Site, int) ([]Observation, error) {
	return s, nil
}

func TestCompositeGatewayPreservesDistrictEvidenceAndPromotesPointHistory(t *testing.T) {
	for _, status := range []domain.DataStatus{domain.DataMissing, domain.DataVerified} {
		district := Observation{MetricType: "flood", Status: status, Source: domain.DataSource{Name: "DDPM"}, RawValue: json.RawMessage(`{"assessmentType":"administrative_district_historical_reports","reportedFloodYearCount":3}`)}
		v := gatewayFloodValue{AssessmentType: "gistda_gateway_flood", Historical: gatewayFloodEvidence{Status: "available", Complete: true, YearsObserved: []int{2023}}}
		raw, _ := json.Marshal(v)
		point := Observation{MetricType: "flood_gateway", Status: domain.DataVerified, RawValue: raw, Source: domain.DataSource{Name: "GISTDA GI Service", License: "Open Data Common"}}
		obs, _ := NewCompositeProvider(recentFloodStub{district}, recentFloodStub{point}).Collect(context.Background(), domain.Site{}, 3000)
		for _, o := range obs {
			if o.MetricType == "flood_gateway" {
				t.Fatal("extra scoring metric")
			}
			if o.MetricType == "flood" {
				var v map[string]json.RawMessage
				json.Unmarshal(o.RawValue, &v)
				if o.Status != domain.DataVerified || o.Source.Name != "GISTDA GI Service" || string(v["reportedFloodYearCount"]) != "3" || len(v["gatewayFlood"]) == 0 {
					t.Fatal(o)
				}
			}
		}
	}
}
func TestCompositeRecentFloodPreservesHistoricalStatus(t *testing.T) {
	for _, status := range []domain.DataStatus{domain.DataMissing, domain.DataVerified} {
		historical := Observation{MetricType: "flood", Status: status, Source: domain.DataSource{Name: "district-history", Type: "public_dataset"}, RawValue: json.RawMessage(`{"assessmentType":"administrative_district_historical_reports","reportedFloodYearCount":3}`)}
		recent := Observation{MetricType: "flood_current", Status: domain.DataVerified, RawValue: json.RawMessage(`{"assessmentType":"gistda_recent_flood","windows":[]}`)}
		observations, err := NewCompositeProvider(recentFloodStub{historical}, recentFloodStub{recent}).Collect(context.Background(), domain.Site{}, 3000)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, o := range observations {
			if o.MetricType == "flood_current" {
				t.Fatal("created an additional scoring metric")
			}
			if o.MetricType != "flood" {
				continue
			}
			found = true
			var raw map[string]json.RawMessage
			json.Unmarshal(o.RawValue, &raw)
			if o.Status != status || o.Source.Name != "district-history" || string(raw["reportedFloodYearCount"]) != "3" || len(raw["recentFlood"]) == 0 {
				t.Fatal(o)
			}
		}
		if !found {
			t.Fatal("flood metric missing")
		}
	}
}
