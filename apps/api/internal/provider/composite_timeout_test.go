package provider

import (
	"context"
	"testing"
	"time"

	"github.com/rbc/ev-station/apps/api/internal/domain"
)

type delayedProvider struct{}

func (delayedProvider) Collect(ctx context.Context, _ domain.Site, _ int) ([]Observation, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

type immediateProvider struct{}

func (immediateProvider) Collect(_ context.Context, _ domain.Site, _ int) ([]Observation, error) {
	return []Observation{{MetricType: "population", Status: domain.DataPreliminary, Source: domain.DataSource{Name: "available source", Type: "fixture"}}}, nil
}

func TestCompositeReturnsAvailableEvidenceWhenAnotherProviderTimesOut(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	observations, err := NewCompositeProvider(immediateProvider{}, delayedProvider{}).Collect(ctx, domain.Site{}, 3000)
	if err != nil {
		t.Fatal(err)
	}
	statuses := map[string]domain.DataStatus{}
	for _, observation := range observations {
		statuses[observation.MetricType] = observation.Status
	}
	if statuses["population"] != domain.DataPreliminary || statuses["traffic"] != domain.DataMissing {
		t.Fatalf("expected available population and missing traffic after deadline: %+v", statuses)
	}
}
