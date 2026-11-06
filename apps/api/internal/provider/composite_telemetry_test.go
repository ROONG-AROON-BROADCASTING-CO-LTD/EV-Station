package provider

import (
	"context"
	"errors"
	"testing"

	"github.com/rbc/ev-station/apps/api/internal/domain"
	"github.com/rbc/ev-station/apps/api/internal/telemetry"
)

type telemetryResultProvider struct {
	observations []Observation
	err          error
}

func (p telemetryResultProvider) Collect(context.Context, domain.Site, int) ([]Observation, error) {
	return p.observations, p.err
}

type collectionRecorder struct{ events []telemetry.Event }

func (r *collectionRecorder) Record(e telemetry.Event) { r.events = append(r.events, e) }

func TestInstrumentRecordsEvidenceAvailability(t *testing.T) {
	for _, test := range []struct {
		name         string
		observations []Observation
		err          error
		status       string
	}{
		{name: "empty", status: "missing"},
		{name: "all missing without provider error", observations: []Observation{{MetricType: "road", Status: domain.DataMissing}}, status: "missing"},
		{name: "partial", observations: []Observation{{MetricType: "road", Status: domain.DataEstimated}, {MetricType: "competition", Status: domain.DataMissing}}, status: "partial"},
		{name: "available preliminary", observations: []Observation{{MetricType: "road", Status: domain.DataPreliminary}}, status: "success"},
		{name: "available verified", observations: []Observation{{MetricType: "road", Status: domain.DataVerified}}, status: "success"},
		{name: "ignores out of scope placeholders", observations: []Observation{{MetricType: "population", Status: domain.DataEstimated}, {MetricType: "road", Status: domain.DataMissing, Source: domain.DataSource{Type: "unavailable"}}}, status: "success"},
		{name: "only placeholders", observations: []Observation{{MetricType: "road", Status: domain.DataMissing, Source: domain.DataSource{Type: "unavailable"}}}, status: "missing"},
		{name: "error", err: errors.New("fixture failure"), status: "error"},
		{name: "error with partial observations", observations: []Observation{{MetricType: "road", Status: domain.DataEstimated}}, err: context.DeadlineExceeded, status: "error"},
	} {
		t.Run(test.name, func(t *testing.T) {
			recorder := &collectionRecorder{}
			wrapped := Instrument("fixture", telemetryResultProvider{test.observations, test.err}, recorder)
			ctx := telemetry.WithAnalysisID(context.Background(), "analysis-fixture")
			observations, err := wrapped.Collect(ctx, domain.Site{}, 3000)
			if err != test.err || len(observations) != len(test.observations) {
				t.Fatal("instrumentation changed the provider result")
			}
			if len(recorder.events) != 1 {
				t.Fatalf("expected one collection event, got %d", len(recorder.events))
			}
			event := recorder.events[0]
			if event.Status != test.status || event.Provider != "fixture" || event.Operation != "provider_collect" || event.AnalysisID != "analysis-fixture" {
				t.Fatalf("incorrect collection outcome: %+v, expected status=%s", event, test.status)
			}
			if event.ErrorType != telemetry.ErrorType(test.err) {
				t.Fatalf("error classification changed: %+v", event)
			}
		})
	}
}
