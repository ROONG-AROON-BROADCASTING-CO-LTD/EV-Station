package scoring

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"

	"github.com/rbc/ev-station/apps/api/internal/domain"
)

const (
	PreliminaryVersion        = "preliminary-v2"
	MinimumCoveragePercentage = 60.0
)

type PreliminaryResult struct {
	Overall      *float64
	MetricScores map[string]float64
	Rules        map[string]string
	Summary      domain.ScoringSummary
}

// EvaluatePreliminary converts provider evidence into transparent screening
// scores. It never changes evidence status. Customer-supplied ground-surface
// assessment is not included because it is a separate visual site review.
func (e *Engine) EvaluatePreliminary(metrics []domain.Metric) PreliminaryResult {
	result := PreliminaryResult{
		MetricScores: make(map[string]float64),
		Rules:        make(map[string]string),
		Summary: domain.ScoringSummary{
			Version: PreliminaryVersion, RequiredMetricCount: len(e.Weights),
			MinimumCoveragePercent: MinimumCoveragePercentage,
			Limitations: []string{
				"This score is a deterministic preliminary screening indicator, not an investment approval.",
				"Missing metrics are excluded and the available weights are renormalized; review coverage before comparing sites.",
				"Electrical readiness uses only published-grid proximity or service-area evidence; it does not confirm capacity, a connection point, or three-phase supply for the plot.",
			},
		},
	}

	availableWeight := 0.0
	weightedTotal := 0.0
	seen := make(map[string]bool, len(metrics))
	for _, metric := range metrics {
		weight, required := e.Weights[metric.Type]
		if !required {
			continue
		}
		seen[metric.Type] = true
		score, rule, ok := preliminaryMetricScore(metric)
		if !ok {
			result.Summary.ExcludedMetrics = append(result.Summary.ExcludedMetrics, metric.Type)
			continue
		}
		score = roundTwo(clamp(score, 0, 100))
		result.MetricScores[metric.Type] = score
		result.Rules[metric.Type] = rule
		availableWeight += weight
		weightedTotal += score * weight
	}
	for metricType := range e.Weights {
		if !seen[metricType] {
			result.Summary.ExcludedMetrics = append(result.Summary.ExcludedMetrics, metricType)
		}
	}
	sort.Strings(result.Summary.ExcludedMetrics)
	result.Summary.ScoredMetricCount = len(result.MetricScores)
	result.Summary.CoveragePercentage = roundTwo(availableWeight * 100)
	if result.Summary.CoveragePercentage >= MinimumCoveragePercentage && availableWeight > 0 {
		overall := roundTwo(weightedTotal / availableWeight)
		result.Overall = &overall
	}
	return result
}

func preliminaryMetricScore(metric domain.Metric) (float64, string, bool) {
	// Older reports can already have a normalized POI score persisted.  Do not
	// let that stale score survive recalculation when the map response was empty
	// or did not declare complete coverage.
	if metric.Type == "poi" && incompleteZeroPOI(metric.RawValue) {
		return 0, "", false
	}
	if metric.NormalizedScore != nil {
		return *metric.NormalizedScore, "Provider-supplied deterministic normalized score.", true
	}
	if metric.Status == domain.DataMissing || len(metric.RawValue) == 0 {
		return 0, "", false
	}
	switch metric.Type {
	case "traffic":
		var value struct {
			AADT           *float64 `json:"aadt"`
			AssessmentType string   `json:"assessmentType"`
			PotentialScore *float64 `json:"potentialScore"`
		}
		if json.Unmarshal(metric.RawValue, &value) == nil {
			if value.AADT != nil && *value.AADT >= 0 {
				return 20 + 80*(*value.AADT/80000), "AADT screening rule: 20 points at zero, increasing linearly to 100 at 80,000 vehicles/day.", true
			}
			if value.AssessmentType == "osm_road_traffic_potential" && value.PotentialScore != nil {
				return *value.PotentialScore, "Road-based traffic-potential rule: mapped road class, proximity and network connectivity are used only when an official AADT match is unavailable; it is not a vehicle count.", true
			}
		}
	case "road_accessibility":
		var road struct {
			MappedMajorRoadCount int      `json:"mappedMajorRoadCount"`
			NearestMeters        *float64 `json:"nearestMajorRoadMeters"`
		}
		if json.Unmarshal(metric.RawValue, &road) == nil && road.NearestMeters != nil {
			proximity := 100 - (*road.NearestMeters / 20)
			count := math.Min(float64(road.MappedMajorRoadCount)*2, 100)
			return proximity*0.7 + count*0.3, "Road-accessibility rule: 70% nearest-major-road proximity and 30% mapped major-road count; this does not verify the actual entrance, turning movements, road width or on-site obstructions.", true
		}
	case "ev_demand":
		var value struct {
			RegisteredBEV float64 `json:"registeredBev"`
		}
		if json.Unmarshal(metric.RawValue, &value) == nil && value.RegisteredBEV >= 0 {
			return 20 + 80*(value.RegisteredBEV/300000), "Provincial BEV screening rule: 20 points at zero, increasing linearly to 100 at 300,000 registered BEVs; not local demand.", true
		}
	case "population":
		var value struct {
			Density float64 `json:"populationDensityPerKm2"`
		}
		if json.Unmarshal(metric.RawValue, &value) == nil && value.Density >= 0 {
			return 10 + 90*(value.Density/6000), "Population rule: modelled density increases linearly from 10 to 100 points at 6,000 people/km².", true
		}
	case "poi":
		var value struct {
			Count            float64 `json:"count"`
			Radius           float64 `json:"radiusMeters"`
			CoverageComplete bool    `json:"coverageComplete"`
		}
		if json.Unmarshal(metric.RawValue, &value) == nil && value.CoverageComplete && value.Count >= 0 && value.Radius > 0 {
			areaKM2 := math.Pi * math.Pow(value.Radius/1000, 2)
			density := value.Count / areaKM2
			return 10 + 90*(density/8), "POI rule: mapped POI density increases linearly from 10 to 100 points at 8 POIs/km².", true
		}
	case "competition":
		var value struct {
			Count           float64 `json:"count"`
			CoverageMatched bool    `json:"coverageMatched"`
		}
		if json.Unmarshal(metric.RawValue, &value) == nil && value.Count >= 0 && value.CoverageMatched {
			return 80 - value.Count*8, "Competition rule: starts at 80 with no mapped competitors and subtracts 8 points per deduplicated station, with a 0–100 clamp.", true
		}
	case "flood":
		var value struct {
			Count float64 `json:"mappedFloodRiskAreaCount"`
		}
		if json.Unmarshal(metric.RawValue, &value) == nil && value.Count >= 0 {
			if value.Count == 0 {
				return 80, "Flood-layer rule: 80 when no published risk polygon overlaps the radius; this does not mean zero flood risk.", true
			}
			return 40 - (value.Count-1)*5, "Flood-layer rule: 40 for one overlapping published risk polygon, minus 5 for each additional polygon.", true
		}
	case "electrical":
		var value struct {
			AssessmentType               string   `json:"assessmentType"`
			MatchingMethod               string   `json:"matchingMethod"`
			DistanceToAreaMeters         *float64 `json:"distanceToAreaMeters"`
			NearestHighVoltageLineMeters *float64 `json:"nearestHighVoltageLineMeters"`
			NearestStationMeters         *float64 `json:"nearestStationMeters"`
			DataAvailable                bool     `json:"dataAvailable"`
		}
		if json.Unmarshal(metric.RawValue, &value) != nil {
			return 0, "", false
		}
		nearest := nearestElectricalDistance(value.NearestHighVoltageLineMeters, value.NearestStationMeters, value.DistanceToAreaMeters)
		if nearest != nil {
			return 100 - *nearest/100, "Electrical-proximity rule: score decreases linearly from 100 at a published high-voltage line, station, or MEA station-area boundary to 0 at 10 km. This is public-map proximity only and does not confirm capacity, a connection point, or three-phase supply for the plot.", true
		}
		if value.AssessmentType == "published_station_area_guideline" && value.MatchingMethod == "point_in_published_area" {
			return 65, "Electrical-area rule: 65 when the plot falls within a published MEA station area. This is an area reference only and does not confirm capacity or connection feasibility for the plot.", true
		}
		if value.AssessmentType == "official_public_map_guideline" && value.DataAvailable {
			return 50, "Electrical-service-area rule: 50 when the PEA public planning map is available for the area but no plot-level grid distance is published. This does not confirm capacity or connection feasibility for the plot.", true
		}
	case "site_requirements":
		var value struct {
			InternetAvailable     *bool    `json:"internetAvailable"`
			InternetSupports24GHz *bool    `json:"internetSupports24GHz"`
			LandLevelingRequired  *bool    `json:"landLevelingRequired"`
			FrontageMeters        *float64 `json:"frontageMeters"`
			ElectricalExtensionKM *float64 `json:"electricalExtensionKm"`
		}
		if json.Unmarshal(metric.RawValue, &value) != nil || value.InternetAvailable == nil || value.InternetSupports24GHz == nil || value.LandLevelingRequired == nil || value.FrontageMeters == nil || value.ElectricalExtensionKM == nil {
			return 0, "", false
		}
		score := 0.0
		if *value.InternetAvailable {
			score += 25
		}
		if *value.InternetAvailable && *value.InternetSupports24GHz {
			score += 15
		}
		if !*value.LandLevelingRequired {
			score += 15
		} else {
			score += 5
		}
		if *value.FrontageMeters >= 7 {
			score += 25
		} else if *value.FrontageMeters >= 6 {
			score += 15
		}
		if *value.ElectricalExtensionKM <= 1 {
			score += 20
		} else if *value.ElectricalExtensionKM <= 3 {
			score += 10
		}
		return score, "Site-requirements rule: internet availability 25 points, 2.4 GHz support 15 points, no land levelling required 15 points (5 when required), frontage 25 points at 7 metres or more (15 at 6–6.99 metres), and estimated electrical extension 20 points up to 1 km, 10 points for 1–3 km, or 0 above 3 km. These are customer/field-survey inputs and require verification.", true
	}
	return 0, "", false
}

func incompleteZeroPOI(raw json.RawMessage) bool {
	var value struct {
		Count            float64 `json:"count"`
		CoverageComplete *bool   `json:"coverageComplete"`
	}
	if json.Unmarshal(raw, &value) != nil || value.Count != 0 {
		return false
	}
	return value.CoverageComplete == nil || !*value.CoverageComplete
}

func nearestElectricalDistance(distances ...*float64) *float64 {
	var nearest *float64
	for _, distance := range distances {
		if distance == nil || *distance < 0 {
			continue
		}
		if nearest == nil || *distance < *nearest {
			value := *distance
			nearest = &value
		}
	}
	return nearest
}

func ScoringRuleAssumption(rule string) string {
	return fmt.Sprintf("Deterministic %s scoring rule: %s", PreliminaryVersion, rule)
}

func clamp(value, minimum, maximum float64) float64 {
	return math.Max(minimum, math.Min(maximum, value))
}

func roundTwo(value float64) float64 { return math.Round(value*100) / 100 }
