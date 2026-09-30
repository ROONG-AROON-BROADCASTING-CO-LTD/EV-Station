package provider

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/rbc/ev-station/apps/api/internal/cache"
	"github.com/rbc/ev-station/apps/api/internal/domain"
	"github.com/rbc/ev-station/apps/api/internal/telemetry"
)

// LocalTrafficProvider reads an operator-curated collection of survey stations
// from any province. A point is nearby context, not the plot's entrance count.
type LocalTrafficProvider struct {
	URL    string
	Client *http.Client
	Cache  cache.Cache
}
type localTrafficCount struct {
	AssessmentType string  `json:"assessmentType"`
	Station        string  `json:"station"`
	Province       string  `json:"province"`
	Latitude       float64 `json:"latitude"`
	Longitude      float64 `json:"longitude"`
	Value          float64 `json:"value"`
	Unit           string  `json:"unit"`
	SurveyDate     string  `json:"surveyDate"`
	SurveyHours    float64 `json:"surveyHours"`
	Source         string  `json:"source"`
	ReferenceURI   string  `json:"referenceUri"`
	DistanceMeters float64 `json:"distanceMeters"`
}

func (p *LocalTrafficProvider) Collect(ctx context.Context, site domain.Site, radius int) ([]Observation, error) {
	ctx = telemetry.WithOperation(ctx, "local_traffic_download")
	source := domain.DataSource{Name: "Provincial / local traffic surveys", Type: "local_traffic_survey", RetrievedAt: time.Now().UTC()}
	missing := func(reason string) ([]Observation, error) {
		return []Observation{{MetricType: "traffic", Status: domain.DataMissing, Source: source, Assumptions: []string{reason}}}, nil
	}
	if p.URL == "" {
		return nil, nil
	}
	if site.Latitude == nil || site.Longitude == nil {
		return missing("Valid coordinates are required for local traffic surveys.")
	}
	if radius <= 0 || radius > 1000 {
		radius = 1000
	}
	client := p.Client
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	c := p.Cache
	if c == nil {
		c = cache.Noop{}
	}
	key := "traffic:local:v1:" + hashText(p.URL)
	payload, found, _ := c.Get(ctx, key)
	if !found {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.URL, nil)
		if err != nil {
			return missing("Local traffic survey data could not be retrieved.")
		}
		res, err := client.Do(req)
		if err != nil {
			return missing("Local traffic survey data could not be retrieved.")
		}
		payload, err = io.ReadAll(io.LimitReader(res.Body, 16<<20))
		res.Body.Close()
		if err != nil || res.StatusCode != 200 {
			return missing("Local traffic survey data could not be retrieved.")
		}
	}
	records, err := parseLocalTraffic(payload)
	if err != nil {
		return missing("Local traffic survey data could not be validated.")
	}
	if !found {
		_ = c.Set(ctx, key, payload, 24*time.Hour)
	}
	var selected *localTrafficCount
	for _, record := range records {
		record.DistanceMeters = haversineMeters(*site.Latitude, *site.Longitude, record.Latitude, record.Longitude)
		if record.DistanceMeters > float64(radius) {
			continue
		}
		if selected == nil || record.DistanceMeters < selected.DistanceMeters || (record.DistanceMeters == selected.DistanceMeters && record.SurveyDate > selected.SurveyDate) {
			copy := record
			selected = &copy
		}
	}
	if selected == nil {
		return missing("No local traffic survey station was found within 1 km or the selected radius.")
	}
	raw, _ := json.Marshal(selected)
	source.Name = selected.Source
	source.ReferenceURI = selected.ReferenceURI
	source.DatasetVersion = selected.SurveyDate
	return []Observation{{MetricType: "traffic", RawValue: raw, Status: domain.DataPreliminary, Source: source, Assumptions: []string{"Nearby survey-station context only. Road connectivity, direction and access to the plot require review.", "Original survey units and duration are preserved. This result is excluded from the AADT scoring formula."}}}, nil
}

func parseLocalTraffic(payload []byte) ([]localTrafficCount, error) {
	reader := csv.NewReader(strings.NewReader(strings.TrimPrefix(string(payload), "\ufeff")))
	rows, err := reader.ReadAll()
	if err != nil || len(rows) < 2 {
		return nil, fmt.Errorf("invalid survey CSV")
	}
	header := map[string]int{}
	for i, v := range rows[0] {
		header[strings.TrimSpace(v)] = i
	}
	required := []string{"station", "province", "latitude", "longitude", "value", "unit", "survey_date", "survey_hours", "source", "reference_url"}
	for _, key := range required {
		if _, ok := header[key]; !ok {
			return nil, fmt.Errorf("missing column %s", key)
		}
	}
	records := []localTrafficCount{}
	for _, row := range rows[1:] {
		get := func(k string) string { return strings.TrimSpace(row[header[k]]) }
		num := func(k string) float64 {
			n, e := strconv.ParseFloat(get(k), 64)
			if e != nil {
				return math.NaN()
			}
			return n
		}
		lat, lon, value, hours := num("latitude"), num("longitude"), num("value"), num("survey_hours")
		for _, n := range []float64{lat, lon, value, hours} {
			if math.IsNaN(n) || math.IsInf(n, 0) {
				return nil, fmt.Errorf("non-finite survey value")
			}
		}
		date, e := time.Parse("2006-01-02", get("survey_date"))
		if e != nil || date.After(time.Now().Add(24*time.Hour)) {
			return nil, fmt.Errorf("invalid survey date")
		}
		ref, e := url.Parse(get("reference_url"))
		if e != nil || ref.Host == "" || (ref.Scheme != "https" && ref.Scheme != "http") {
			return nil, fmt.Errorf("invalid survey reference")
		}
		unit := get("unit")
		if lat < 5 || lat > 21 || lon < 97 || lon > 106 || value < 0 || hours <= 0 || get("station") == "" || get("source") == "" || (unit != "vehicles/day" && unit != "vehicles/survey" && unit != "PCU/day" && unit != "PCU/hour") {
			return nil, fmt.Errorf("invalid survey record")
		}
		records = append(records, localTrafficCount{AssessmentType: "local_traffic_count", Station: get("station"), Province: get("province"), Latitude: lat, Longitude: lon, Value: value, Unit: unit, SurveyDate: get("survey_date"), SurveyHours: hours, Source: get("source"), ReferenceURI: ref.String()})
	}
	return records, nil
}
