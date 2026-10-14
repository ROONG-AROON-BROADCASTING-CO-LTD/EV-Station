package provider

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// Fetch all bounded local pages when possible. A capped/failed continuation
// keeps numberMatched unchanged, so it cannot become a negative site finding.
func (p *GISTDAGatewayFloodProvider) collectFeaturePages(ctx context.Context, initialURL *url.URL, first []byte) []byte {
	var collection struct {
		Type          string            `json:"type"`
		Features      []json.RawMessage `json:"features"`
		NumberMatched *int              `json:"numberMatched"`
	}
	if json.Unmarshal(first, &collection) != nil || collection.Type != "FeatureCollection" || collection.NumberMatched == nil || len(collection.Features) == 0 || *collection.NumberMatched <= len(collection.Features) {
		return first
	}
	features := collection.Features
	seen := map[string]bool{}
	for _, f := range features {
		id := gatewayFeatureID(f)
		if seen[id] {
			return []byte(`{"error":"duplicate feature"}`)
		}
		seen[id] = true
	}
	bytesUsed := len(first)
	for page := 1; page < 10 && len(features) < *collection.NumberMatched; page++ {
		u := *initialURL
		q := u.Query()
		q.Set("offset", strconv.Itoa(len(features)))
		u.RawQuery = q.Encode()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		if err != nil {
			break
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", p.config.UserAgent)
		r, err := p.client.Do(req)
		if err != nil {
			break
		}
		b, readErr := io.ReadAll(io.LimitReader(r.Body, (4<<20)+1))
		r.Body.Close()
		if readErr != nil || r.StatusCode != 200 || len(b) > 4<<20 {
			break
		}
		bytesUsed += len(b)
		if bytesUsed > 8<<20 {
			break
		}
		b = bytes.ReplaceAll(b, []byte(p.config.APIKey), []byte("[REDACTED]"))
		var next struct {
			Type          string            `json:"type"`
			Features      []json.RawMessage `json:"features"`
			NumberMatched *int              `json:"numberMatched"`
		}
		if json.Unmarshal(b, &next) != nil || next.Type != "FeatureCollection" || next.NumberMatched == nil || *next.NumberMatched != *collection.NumberMatched || len(next.Features) == 0 {
			break
		}
		duplicate := false
		for _, f := range next.Features {
			id := gatewayFeatureID(f)
			if seen[id] {
				duplicate = true
				break
			}
			seen[id] = true
		}
		if duplicate {
			break
		}
		features = append(features, next.Features...)
	}
	// Only persist data fields, never credential-bearing pagination links.
	b, _ := json.Marshal(map[string]any{"type": "FeatureCollection", "features": features, "numberMatched": *collection.NumberMatched, "numberReturned": len(features)})
	return b
}

func gatewayFeatureID(f json.RawMessage) string {
	var row struct {
		ID         json.RawMessage            `json:"id"`
		Properties map[string]json.RawMessage `json:"properties"`
	}
	_ = json.Unmarshal(f, &row)
	if len(row.ID) > 0 && string(row.ID) != "null" {
		return string(row.ID)
	}
	if id := row.Properties["_id"]; len(id) > 0 {
		return string(id)
	}
	var canonical any
	_ = json.Unmarshal(f, &canonical)
	b, _ := json.Marshal(canonical)
	return fmt.Sprintf("%x", sha256.Sum256(b))
}

func parseGatewayMappedHistory(b []byte, e *gatewayFloodEvidence, lat, lon float64) string {
	count, overlap, complete, err := parseRecentFlood(b, lon, lat)
	if err != nil {
		return "invalid_response"
	}
	e.FeatureCount = &count
	e.SiteOverlapCount = &overlap
	e.Complete = complete
	if count == 0 {
		e.Complete = false
		e.Result = "unknown"
		e.Reason = "no_record"
		return ""
	}
	var collection struct {
		Features []json.RawMessage `json:"features"`
	}
	_ = json.Unmarshal(b, &collection)
	pointYears, nearbyYears := map[int]bool{}, map[int]bool{}
	end := 0
	for _, f := range collection.Features {
		var row struct {
			Properties map[string]json.RawMessage `json:"properties"`
		}
		if json.Unmarshal(f, &row) != nil {
			return "invalid_response"
		}
		rowEnd := 0
		for key := range row.Properties {
			if strings.HasPrefix(key, "y_") {
				y, err := strconv.Atoi(strings.TrimPrefix(key, "y_"))
				if err != nil || y < 2011 || y > 2026 {
					return "invalid_response"
				}
				if y > rowEnd {
					rowEnd = y
				}
			}
		}
		if rowEnd < 2023 || (end != 0 && end != rowEnd) {
			return "invalid_response"
		}
		end = rowEnd
		one, _ := json.Marshal(map[string]any{"type": "FeatureCollection", "features": []json.RawMessage{f}, "numberMatched": 1})
		_, inside, _, err := parseRecentFlood(one, lon, lat)
		if err != nil {
			return "invalid_response"
		}
		for y := 2011; y <= end; y++ {
			raw, ok := row.Properties["y_"+strconv.Itoa(y)]
			if !ok {
				return "invalid_response"
			}
			var freq *float64
			if json.Unmarshal(raw, &freq) != nil || freq == nil || math.IsNaN(*freq) || *freq < 0 || *freq > 10000 || *freq != math.Trunc(*freq) {
				return "invalid_response"
			}
			if *freq > 0 {
				nearbyYears[y] = true
				if inside > 0 {
					pointYears[y] = true
				}
			}
		}
	}
	e.PeriodStartYear = 2011
	e.PeriodEndYear = end
	for y := 2011; y <= end; y++ {
		if pointYears[y] {
			e.YearsObserved = append(e.YearsObserved, y)
		}
		if nearbyYears[y] {
			e.NearbyYearsObserved = append(e.NearbyYearsObserved, y)
		}
	}
	e.Result = "not found"
	if overlap > 0 {
		e.Result = "found"
	} else if !complete {
		e.Result = "unknown"
		e.Reason = "incomplete_response"
	}
	return ""
}
