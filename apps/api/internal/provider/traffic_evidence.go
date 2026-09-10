package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/rbc/ev-station/apps/api/internal/cache"
	"github.com/rbc/ev-station/apps/api/internal/domain"
)

type trafficCheck struct {
	Source  string `json:"source"`
	Outcome string `json:"outcome"`
	Reason  string `json:"reason,omitempty"`
}

func betterTraffic(candidate, current Observation) bool {
	read := func(item Observation) (int, float64, int) {
		if item.Status == domain.DataMissing {
			return 0, math.Inf(1), 0
		}
		var value struct {
			AADT           *float64 `json:"aadt"`
			AssessmentType string   `json:"assessmentType"`
			Distance       *float64 `json:"distanceToSectionMeters"`
			RoadDistance   *float64 `json:"distanceToRoadMeters"`
			Year           int      `json:"dataYear"`
		}
		_ = json.Unmarshal(item.RawValue, &value)
		distance := math.Inf(1)
		if value.Distance != nil {
			distance = *value.Distance
		} else if value.RoadDistance != nil {
			distance = *value.RoadDistance
		}
		if value.AADT != nil && item.Status == domain.DataVerified {
			return 4, distance, value.Year
		}
		if value.AssessmentType == "local_traffic_count" {
			return 3, distance, value.Year
		}
		return dataStatusStrength(item.Status), distance, value.Year
	}
	a, ad, ay := read(candidate)
	b, bd, by := read(current)
	if a != b {
		return a > b
	}
	if ay != by {
		return ay > by
	}
	if ad != bd {
		return ad < bd
	}
	return candidate.Source.Name < current.Source.Name
}

// Preserve each source outcome, including failures, alongside the selected value.
func attachTrafficChecks(selected Observation, candidates []Observation) Observation {
	checks := []trafficCheck{}
	for _, item := range candidates {
		if item.Source.Type == "unavailable" {
			continue
		}
		outcome := "available"
		reason := ""
		if item.Status == domain.DataMissing {
			outcome = "no_match"
			if len(item.Assumptions) > 0 {
				reason = item.Assumptions[0]
			}
			if strings.Contains(reason, "could not") || strings.Contains(reason, "failed") {
				outcome = "unavailable"
			}
		}
		checks = append(checks, trafficCheck{item.Source.Name, outcome, reason})
	}
	sort.Slice(checks, func(i, j int) bool { return checks[i].Source < checks[j].Source })
	if len(checks) == 0 {
		return selected
	}
	value := map[string]any{}
	_ = json.Unmarshal(selected.RawValue, &value)
	if value == nil {
		value = map[string]any{}
	}
	value["sourceChecks"] = checks
	selected.RawValue, _ = json.Marshal(value)
	if selected.Status == domain.DataMissing {
		selected.Assumptions = []string{"No usable traffic evidence was returned. Review the individual source results."}
	}
	return selected
}

// ArcGIS can return HTTP 200 with an error or a truncated feature collection.
// Only cache validated, complete responses; legacy cached errors are bypassed.
func fetchTrafficRoads[T any](ctx context.Context, client *http.Client, externalCache cache.Cache, endpoint, fields, idField, agent string, ttl time.Duration, lat, lon float64, radius int) ([]T, error) {
	params := url.Values{"f": {"json"}, "where": {"1=1"}, "geometry": {fmt.Sprintf("%.7f,%.7f", lon, lat)}, "geometryType": {"esriGeometryPoint"}, "inSR": {"4326"}, "spatialRel": {"esriSpatialRelIntersects"}, "distance": {strconv.Itoa(radius)}, "units": {"esriSRUnit_Meter"}, "outFields": {fields}, "returnGeometry": {"true"}, "outSR": {"4326"}, "orderByFields": {idField + " ASC"}, "resultRecordCount": {"1000"}}
	key := "traffic:roads:v2:" + hashText(endpoint+params.Encode())
	if payload, found, err := externalCache.Get(ctx, key); err == nil && found {
		var features []T
		if json.Unmarshal(payload, &features) == nil && features != nil {
			return features, nil
		}
	}
	features := []T{}
	for page := 0; page < 50; page++ {
		params.Set("resultOffset", strconv.Itoa(len(features)))
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(endpoint, "?")+"?"+params.Encode(), nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", agent)
		res, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		payload, readErr := io.ReadAll(io.LimitReader(res.Body, 16<<20))
		res.Body.Close()
		if readErr != nil {
			return nil, readErr
		}
		if res.StatusCode != 200 {
			return nil, fmt.Errorf("road layer HTTP %d", res.StatusCode)
		}
		var result struct {
			Features []T `json:"features"`
			Error    *struct {
				Code int `json:"code"`
			} `json:"error"`
			More bool `json:"exceededTransferLimit"`
		}
		if err = json.Unmarshal(payload, &result); err != nil {
			return nil, err
		}
		if result.Error != nil {
			return nil, fmt.Errorf("ArcGIS error %d", result.Error.Code)
		}
		if result.Features == nil {
			return nil, fmt.Errorf("road layer omitted features")
		}
		features = append(features, result.Features...)
		if !result.More {
			payload, _ = json.Marshal(features)
			_ = externalCache.Set(ctx, key, payload, ttl)
			return features, nil
		}
		if len(result.Features) == 0 {
			return nil, fmt.Errorf("road pagination made no progress")
		}
	}
	return nil, fmt.Errorf("road pagination limit exceeded")
}
