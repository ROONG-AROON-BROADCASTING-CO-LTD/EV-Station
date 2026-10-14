package provider

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/rbc/ev-station/apps/api/internal/domain"
	"github.com/rbc/ev-station/apps/api/internal/telemetry"
)

// CompositeProvider runs independent providers concurrently and keeps the
// strongest factual observation for each metric. Missing observations never
// overwrite an available observation.
type CompositeProvider struct {
	providers []AnalysisProvider
}

func NewCompositeProvider(providers ...AnalysisProvider) *CompositeProvider {
	return &CompositeProvider{providers: providers}
}

// Instrument wraps a provider at the composite boundary. It records its total
// duration without changing provider selection, concurrency, or failures.
func Instrument(name string, current AnalysisProvider, recorder telemetry.Recorder) AnalysisProvider {
	if recorder == nil {
		recorder = telemetry.Noop()
	}
	return instrumentedProvider{name: name, current: current, recorder: recorder}
}

type instrumentedProvider struct {
	name     string
	current  AnalysisProvider
	recorder telemetry.Recorder
}

func (p instrumentedProvider) Collect(ctx context.Context, site domain.Site, radius int) ([]Observation, error) {
	ctx = telemetry.WithRecorder(ctx, p.recorder)
	started := time.Now().UTC()
	observations, err := p.current.Collect(ctx, site, radius)
	finished := time.Now().UTC()
	status := "success"
	if err != nil {
		status = "error"
	}
	p.recorder.Record(telemetry.Event{AnalysisID: telemetry.AnalysisID(ctx), Provider: p.name, Operation: "provider_collect", StartedAt: started, FinishedAt: finished, DurationMS: finished.Sub(started).Milliseconds(), Status: status, ErrorType: telemetry.ErrorType(err)})
	return observations, err
}

func (p *CompositeProvider) Collect(ctx context.Context, site domain.Site, radius int) ([]Observation, error) {
	type result struct {
		observations []Observation
		err          error
	}
	results := make(chan result, len(p.providers))
	for _, dataProvider := range p.providers {
		go func(current AnalysisProvider) {
			observations, err := current.Collect(ctx, site, radius)
			results <- result{observations: observations, err: err}
		}(dataProvider)
	}
	collected := make([]result, 0, len(p.providers))
collect:
	for len(collected) < len(p.providers) {
		select {
		case item := <-results:
			collected = append(collected, item)
		case <-ctx.Done():
			for {
				select {
				case item := <-results:
					collected = append(collected, item)
				default:
					break collect
				}
			}
		}
	}

	merged, positions := unavailableObservations()
	competitionObservations := make([]Observation, 0, len(p.providers))
	floodObservations := make([]Observation, 0, len(p.providers))
	supplemental := make(map[string]Observation)
	trafficCandidates := []Observation{}
	for _, providerResult := range collected {
		if providerResult.err != nil {
			continue
		}
		for _, observation := range providerResult.observations {
			if observation.MetricType == "flood_current" || observation.MetricType == "flood_gateway" {
				supplemental[observation.MetricType] = observation
				continue
			}
			if observation.MetricType == "flood" {
				floodObservations = append(floodObservations, observation)
			}
			if observation.MetricType == "traffic" {
				trafficCandidates = append(trafficCandidates, observation)
			}
			if observation.MetricType == "site_requirements" {
				current, found := supplemental[observation.MetricType]
				if !found || dataStatusStrength(observation.Status) > dataStatusStrength(current.Status) {
					supplemental[observation.MetricType] = observation
				}
				continue
			}
			position, exists := positions[observation.MetricType]
			if !exists {
				continue
			}
			if observation.MetricType == "competition" && observation.Status != domain.DataMissing {
				competitionObservations = append(competitionObservations, observation)
				continue
			}
			if observation.Status == domain.DataMissing {
				if merged[position].Status == domain.DataMissing && merged[position].Source.Type == "unavailable" {
					merged[position] = observation
				}
				continue
			}
			if observation.MetricType == "traffic" {
				if betterTraffic(observation, merged[position]) {
					merged[position] = observation
				}
				continue
			}
			if dataStatusStrength(observation.Status) > dataStatusStrength(merged[position].Status) ||
				(dataStatusStrength(observation.Status) == dataStatusStrength(merged[position].Status) && observationPriority(observation) > observationPriority(merged[position])) {
				merged[position] = observation
			}
		}
	}
	if len(competitionObservations) > 0 {
		merged[positions["competition"]] = mergeCompetitionObservations(competitionObservations, radius)
	}
	trafficPosition := positions["traffic"]
	merged[trafficPosition] = attachTrafficChecks(merged[trafficPosition], trafficCandidates)
	floodPosition := positions["flood"]
	merged[floodPosition] = attachFloodEvidence(merged[floodPosition], floodObservations)
	if gateway, found := supplemental["flood_gateway"]; found {
		value := make(map[string]json.RawMessage)
		_ = json.Unmarshal(merged[floodPosition].RawValue, &value)
		if value == nil {
			value = make(map[string]json.RawMessage)
		}
		value["gatewayFlood"] = gateway.RawValue
		merged[floodPosition].RawValue, _ = json.Marshal(value)
		var point gatewayFloodValue
		if json.Unmarshal(gateway.RawValue, &point) == nil && point.Historical.Status == "available" && point.Historical.Complete && (len(point.Historical.YearsObserved) > 0 || merged[floodPosition].Status == domain.DataMissing) {
			merged[floodPosition].Status = domain.DataVerified
			merged[floodPosition].Source = gateway.Source
			merged[floodPosition].Assumptions = gateway.Assumptions
		}
		delete(supplemental, "flood_gateway")
	}
	if recent, found := supplemental["flood_current"]; found {
		value := make(map[string]any)
		if len(merged[floodPosition].RawValue) > 0 {
			_ = json.Unmarshal(merged[floodPosition].RawValue, &value)
		}
		if value == nil {
			value = make(map[string]any)
		}
		value["recentFlood"] = json.RawMessage(recent.RawValue)
		if raw, err := json.Marshal(value); err == nil {
			merged[floodPosition].RawValue = raw
		}
		delete(supplemental, "flood_current")
	}
	for _, observation := range supplemental {
		merged = append(merged, observation)
	}
	if ctx.Err() != nil {
		for index := range merged {
			if merged[index].Status == domain.DataMissing && merged[index].Source.Type == "unavailable" {
				merged[index].Assumptions = []string{"The provider collection deadline ended before evidence for this metric was available."}
			}
		}
	}
	return merged, nil
}

// Keep independently useful flood evidence together (district reports and
// spatial layers measure different things and must not overwrite each other).
func attachFloodEvidence(selected Observation, candidates []Observation) Observation {
	type evidence struct {
		Name        string            `json:"name"`
		Status      domain.DataStatus `json:"status"`
		SourceURI   string            `json:"sourceUri,omitempty"`
		License     string            `json:"license,omitempty"`
		RawValue    json.RawMessage   `json:"rawValue,omitempty"`
		Assumptions []string          `json:"assumptions,omitempty"`
	}
	items := make([]evidence, 0, len(candidates))
	for _, item := range candidates {
		if item.Status == domain.DataMissing || len(item.RawValue) == 0 {
			continue
		}
		items = append(items, evidence{Name: item.Source.Name, Status: item.Status, SourceURI: item.Source.ReferenceURI, License: item.Source.License, RawValue: item.RawValue, Assumptions: item.Assumptions})
	}
	if len(items) < 2 || len(selected.RawValue) == 0 {
		return selected
	}
	var value map[string]any
	if json.Unmarshal(selected.RawValue, &value) != nil || value == nil {
		return selected
	}
	value["floodEvidence"] = items
	if raw, err := json.Marshal(value); err == nil {
		selected.RawValue = raw
	}
	return selected
}

func observationPriority(observation Observation) int {
	switch observation.MetricType {
	case "traffic":
		if strings.Contains(observation.Source.Name, "Department of Highways") {
			return 2
		}
		if strings.Contains(observation.Source.Name, "Department of Rural Roads") {
			return 1
		}
	case "poi":
		if observation.Source.Type == "commercial_places_api" {
			return 2
		}
		if observation.Source.Type == "open_data_api" {
			return 1
		}
	}
	return 0
}

func dataStatusStrength(status domain.DataStatus) int {
	switch status {
	case domain.DataVerified:
		return 3
	case domain.DataEstimated:
		return 2
	case domain.DataPreliminary:
		return 1
	default:
		return 0
	}
}
