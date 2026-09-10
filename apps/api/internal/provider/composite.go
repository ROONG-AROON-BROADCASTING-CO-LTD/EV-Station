package provider

import (
	"context"
	"strings"
	"sync"

	"github.com/rbc/ev-station/apps/api/internal/domain"
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

func (p *CompositeProvider) Collect(ctx context.Context, site domain.Site, radius int) ([]Observation, error) {
	type result struct {
		observations []Observation
		err          error
	}
	results := make(chan result, len(p.providers))
	var wait sync.WaitGroup
	for _, dataProvider := range p.providers {
		wait.Add(1)
		go func(current AnalysisProvider) {
			defer wait.Done()
			observations, err := current.Collect(ctx, site, radius)
			results <- result{observations: observations, err: err}
		}(dataProvider)
	}
	wait.Wait()
	close(results)

	merged, positions := unavailableObservations()
	competitionObservations := make([]Observation, 0, len(p.providers))
	supplemental := make(map[string]Observation)
	trafficCandidates := []Observation{}
	for providerResult := range results {
		if providerResult.err != nil {
			continue
		}
		for _, observation := range providerResult.observations {
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
	for _, observation := range supplemental {
		merged = append(merged, observation)
	}
	return merged, nil
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
