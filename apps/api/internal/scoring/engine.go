package scoring

import (
	"errors"
	"math"
)

var (
	ErrInvalidWeights = errors.New("scoring weights must total 1.0")
	ErrMissingMetric  = errors.New("required metric score is missing")
)

var DefaultWeights = map[string]float64{
	// Customer-supplied ground-surface assessment is deliberately not a score
	// component. Electrical readiness uses only public-grid proximity evidence,
	// never published capacity or a claim of confirmed connection availability.
	"traffic":            4.0 / 24.0,
	"road_accessibility": 1.0 / 24.0,
	"ev_demand":          4.0 / 24.0,
	"population":         3.0 / 24.0,
	"poi":                3.0 / 24.0,
	"competition":        3.0 / 24.0,
	"flood":              1.0 / 24.0,
	"electrical":         3.0 / 24.0,
	"site_requirements":  2.0 / 24.0,
}

type Engine struct{ Weights map[string]float64 }

func New(weights map[string]float64) (*Engine, error) {
	total := 0.0
	for _, weight := range weights {
		if weight < 0 {
			return nil, ErrInvalidWeights
		}
		total += weight
	}
	if math.Abs(total-1) > 0.000001 {
		return nil, ErrInvalidWeights
	}
	return &Engine{Weights: weights}, nil
}

func (e *Engine) Calculate(scores map[string]*float64) (float64, error) {
	total := 0.0
	for metric, weight := range e.Weights {
		score, ok := scores[metric]
		if !ok || score == nil {
			return 0, ErrMissingMetric
		}
		if *score < 0 || *score > 100 {
			return 0, errors.New("metric score must be between 0 and 100")
		}
		total += *score * weight
	}
	return math.Round(total*100) / 100, nil
}
