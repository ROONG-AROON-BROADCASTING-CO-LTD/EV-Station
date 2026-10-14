package provider

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rbc/ev-station/apps/api/internal/cache"
	"github.com/rbc/ev-station/apps/api/internal/domain"
)

// Public Gateway authentication uses api_key. The upstream GI service key
// shown in some example responses is not the user's Gateway credential.
// This credential is intentionally independent of Sphere and Disaster keys.
type GISTDAGatewayFloodConfig struct {
	BaseURL, LatestURL, ArchiveBaseURL, APIKey, UserAgent string
	Geocoder                                              ReverseGeocoder
}
type GISTDAGatewayFloodProvider struct {
	config GISTDAGatewayFloodConfig
	client *http.Client
	cache  cache.Cache
}
type gatewayFloodEvidence struct {
	Status              string     `json:"status"`
	Reason              string     `json:"reason,omitempty"`
	SourceURI           string     `json:"sourceUri"`
	License             string     `json:"license"`
	RetrievedAt         *time.Time `json:"retrievedAt,omitempty"`
	PeriodStartYear     int        `json:"periodStartYear,omitempty"`
	PeriodEndYear       int        `json:"periodEndYear,omitempty"`
	YearsObserved       []int      `json:"yearsObserved,omitempty"`
	Complete            bool       `json:"complete"`
	Result              string     `json:"result,omitempty"`
	ObservationDates    []string   `json:"observationDates,omitempty"`
	DateBasis           string     `json:"dateBasis,omitempty"`
	FeatureCount        *int       `json:"featureCount,omitempty"`
	SiteOverlapCount    *int       `json:"siteOverlapCount,omitempty"`
	SearchRadiusMeters  int        `json:"searchRadiusMeters,omitempty"`
	WindowDays          int        `json:"windowDays,omitempty"`
	NearbyYearsObserved []int      `json:"nearbyYearsObserved,omitempty"`
	GeoScope            string     `json:"geoScope,omitempty"`
}
type gatewayFloodWindow struct {
	Days     int                  `json:"days"`
	Evidence gatewayFloodEvidence `json:"evidence"`
}
type gatewayFloodValue struct {
	AssessmentType string                  `json:"assessmentType"`
	Historical     gatewayFloodEvidence    `json:"historical"`
	Latest         gatewayFloodEvidence    `json:"latest"`
	Windows        []gatewayFloodWindow    `json:"windows,omitempty"`
	MappedHistory  *gatewayFloodEvidence   `json:"mappedHistory,omitempty"`
	Annual         []gatewayAnnualEvidence `json:"annual,omitempty"`
}

func NewGISTDAGatewayFloodProvider(cfg GISTDAGatewayFloodConfig, client *http.Client, c cache.Cache) *GISTDAGatewayFloodProvider {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api-gateway.gistda.or.th/api/2.0/resources/gi-service/v1.1/disasters"
	}
	if cfg.LatestURL == "" && cfg.BaseURL == "https://api-gateway.gistda.or.th/api/2.0/resources/gi-service/v1.1/disasters" {
		cfg.LatestURL = "https://api-gateway.gistda.or.th/api/2.0/resources/features/flood/1day"
	}
	if cfg.ArchiveBaseURL == "" && cfg.LatestURL == "https://api-gateway.gistda.or.th/api/2.0/resources/features/flood/1day" {
		cfg.ArchiveBaseURL = "https://api-gateway.gistda.or.th/api/2.0/resources/stac/flood"
	}
	if client == nil {
		client = &http.Client{Timeout: 18 * time.Second}
	}
	if c == nil {
		c = cache.Noop{}
	}
	return &GISTDAGatewayFloodProvider{cfg, client, c}
}

func (p *GISTDAGatewayFloodProvider) Collect(ctx context.Context, site domain.Site, radius int) ([]Observation, error) {
	if radius <= 0 {
		radius = 3000
	}
	v := gatewayFloodValue{AssessmentType: "gistda_gateway_flood"}
	v.Historical = gatewayFloodEvidence{Status: "missing", SourceURI: "https://data.go.th/dataset/disasters-01", License: "Open Data Common", PeriodStartYear: 2011, PeriodEndYear: 2023}
	v.Latest = gatewayFloodEvidence{Status: "missing", SourceURI: "https://data.go.th/dataset/disasters-03", License: "Open Data Common"}
	if p.config.LatestURL != "" {
		v.Latest.SourceURI = "https://disaster.gistda.or.th/v2/services/open-api"
		v.Latest.License = "Disaster Platform catalogue licence unspecified; situational context only"
		v.Latest.SearchRadiusMeters = radius
		v.Latest.WindowDays = 1
		for _, days := range []int{1, 3, 7, 30} {
			e := v.Latest
			e.WindowDays = days
			v.Windows = append(v.Windows, gatewayFloodWindow{Days: days, Evidence: e})
		}
		e := v.Latest
		e.WindowDays = 0
		v.MappedHistory = &e
	}
	reason := ""
	if site.Latitude == nil || site.Longitude == nil || math.IsNaN(*site.Latitude) || math.IsNaN(*site.Longitude) || math.Abs(*site.Latitude) > 90 || math.Abs(*site.Longitude) > 180 {
		reason = "invalid_coordinates"
	} else if strings.TrimSpace(p.config.APIKey) == "" {
		reason = "not_configured"
	}
	if reason != "" {
		v.Historical.Reason = reason
		v.Latest.Reason = reason
		for i := range v.Windows {
			v.Windows[i].Evidence.Reason = reason
		}
		if v.MappedHistory != nil {
			v.MappedHistory.Reason = reason
		}
	} else {
		var wg sync.WaitGroup
		mappedReady := make(chan []byte, 1)
		if len(v.Windows) == 0 {
			close(mappedReady)
		}
		if p.config.ArchiveBaseURL != "" {
			wg.Add(1)
			go func() {
				defer wg.Done()
				v.Annual = p.collectAnnualWithMapped(ctx, *site.Latitude, *site.Longitude, radius, mappedReady)
			}()
		}
		queries := 2
		if len(v.Windows) > 0 {
			queries = 6
		}
		for i := 0; i < queries; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				var mappedPayload []byte
				if i == 5 {
					defer func() { mappedReady <- mappedPayload; close(mappedReady) }()
				}
				endpoint, ttl := "flood-recurrence", 24*time.Hour
				e := &v.Historical
				if i > 0 {
					endpoint = "flood-extent-1day"
					ttl = time.Hour
					e = &v.Latest
					if i > 1 && i < 5 {
						e = &v.Windows[i-1].Evidence
						endpoint = "flood-extent-" + strconv.Itoa(v.Windows[i-1].Days) + "days"
					}
					if i == 5 {
						e = v.MappedHistory
						endpoint = "flood-mapped-history"
						ttl = 24 * time.Hour
					}
				}
				districtCode := 0
				if i == 5 && p.config.Geocoder != nil {
					lookupCtx, stop := context.WithTimeout(ctx, 5*time.Second)
					location, _ := p.config.Geocoder.Reverse(lookupCtx, *site.Latitude, *site.Longitude)
					stop()
					districtCode = registryDistrictCode(location)
					if districtCode > 0 {
						e.GeoScope = "district_map"
					}
				}
				body, fetched, errReason := p.fetchRadius(ctx, endpoint, *site.Latitude, *site.Longitude, radius, ttl, districtCode)
				if errReason != "" {
					e.Reason = errReason
					return
				}
				if i == 5 {
					mappedPayload = body
				}
				if i == 0 {
					errReason = parseGatewayHistory(body, e)
				} else if i == 5 {
					errReason = parseGatewayMappedHistory(body, e, *site.Latitude, *site.Longitude)
				} else {
					errReason = p.parseLatest(body, e, time.Now().UTC(), *site.Latitude, *site.Longitude)
				}
				if errReason != "" {
					e.Reason = errReason
					return
				}
				e.Status = "available"
				if e.Result == "unknown" {
					e.Status = "missing"
				}
				e.RetrievedAt = &fetched
			}(i)
		}
		wg.Wait()
	}
	if len(v.Windows) > 0 {
		v.Windows[0].Evidence = v.Latest
	}
	raw, _ := json.Marshal(v)
	status := domain.DataMissing
	if v.Historical.Status == "available" || v.Latest.Status == "available" {
		status = domain.DataVerified
	}
	for _, w := range v.Windows {
		if w.Evidence.Status == "available" {
			status = domain.DataVerified
		}
	}
	if v.MappedHistory != nil && v.MappedHistory.Status == "available" {
		status = domain.DataVerified
	}
	retrieved := time.Now().UTC()
	if v.Historical.RetrievedAt != nil {
		retrieved = *v.Historical.RetrievedAt
	} else if v.Latest.RetrievedAt != nil {
		retrieved = *v.Latest.RetrievedAt
	}
	return []Observation{{MetricType: "flood_gateway", RawValue: raw, Status: status, Source: domain.DataSource{Name: "GISTDA flood evidence: point history, mapped observations and annual archives", Type: "open_data_api", Authority: "GISTDA", GeographicScope: "mixed; see individual evidence", ReferenceURI: v.Historical.SourceURI, License: "Point history: Open Data Common; other sections preserve their own source and scope", RetrievedAt: retrieved}, Assumptions: []string{"Historical observations cover 2011–2023 only. Missing or empty records do not prove flood-free conditions.", "The 1/3/7/30-day services are situational evidence, not independent events. Product-name dates are explicitly inferred. Annual archives include administrative summaries, not plot-specific inundation, and are not double-counted in point-history scoring.", "Point results do not confirm frontage-road or access-route flooding."}}}, nil
}

func (p *GISTDAGatewayFloodProvider) fetch(ctx context.Context, path string, lat, lon float64, ttl time.Duration) ([]byte, time.Time, string) {
	return p.fetchRadius(ctx, path, lat, lon, 0, ttl)
}

func (p *GISTDAGatewayFloodProvider) fetchRadius(ctx context.Context, path string, lat, lon float64, radius int, ttl time.Duration, codes ...int) ([]byte, time.Time, string) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	u, err := url.Parse(strings.TrimRight(p.config.BaseURL, "/") + "/" + path)
	featureLatest := (strings.HasPrefix(path, "flood-extent-") || path == "flood-mapped-history") && p.config.LatestURL != ""
	if featureLatest {
		u, err = url.Parse(p.config.LatestURL)
		if err == nil && path == "flood-mapped-history" {
			u.Path = strings.TrimSuffix(u.Path, "/flood/1day") + "/flood-freq"
		} else if err == nil && path != "flood-extent-1day" {
			u.Path = strings.TrimSuffix(u.Path, "/1day") + "/" + strings.TrimPrefix(path, "flood-extent-")
		}
	}
	if err != nil {
		return nil, time.Time{}, "invalid_endpoint"
	}
	q := u.Query()
	q.Set("lat", strconv.FormatFloat(lat, 'f', -1, 64))
	q.Set("lon", strconv.FormatFloat(lon, 'f', -1, 64))
	q.Set("api_key", p.config.APIKey)
	if featureLatest {
		// The service may ignore bbox. Check geometry locally and reject
		// truncated negative results rather than assuming a local absence.
		q.Del("lat")
		q.Del("lon")
		dLat, dLon := 0.001, 0.001
		if radius > 0 {
			dLat = float64(radius) / 111320
			dLon = dLat / math.Max(math.Cos(lat*math.Pi/180), 0.01)
		}
		q.Set("bbox", strconv.FormatFloat(lon-dLon, 'f', 6, 64)+","+strconv.FormatFloat(lat-dLat, 'f', 6, 64)+","+strconv.FormatFloat(lon+dLon, 'f', 6, 64)+","+strconv.FormatFloat(lat+dLat, 'f', 6, 64))
		if path == "flood-mapped-history" && len(codes) > 0 && codes[0] > 0 {
			q.Del("bbox")
			q.Set("ap_idn", strconv.Itoa(codes[0]))
		}
		q.Set("limit", "1000")
		q.Set("offset", "0")
	}
	u.RawQuery = q.Encode()
	h := sha256.Sum256([]byte(u.String()))
	cacheKey := "gistda:gateway:v3:" + hex.EncodeToString(h[:])
	type cachedResponse struct {
		Payload     json.RawMessage
		RetrievedAt time.Time
	}
	if b, found, err := p.cache.Get(ctx, cacheKey); err == nil && found {
		var cached cachedResponse
		if json.Unmarshal(b, &cached) == nil && !cached.RetrievedAt.IsZero() {
			return cached.Payload, cached.RetrievedAt, ""
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, time.Time{}, "invalid_endpoint"
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", p.config.UserAgent)
	resp, err := p.client.Do(req)
	// Never return request errors: Go's URL errors can include the query key.
	if err != nil {
		return nil, time.Time{}, "service_unavailable"
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		reason := "service_unavailable"
		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			reason = "unauthorized"
		}
		if resp.StatusCode == 429 {
			reason = "rate_limited"
		}
		return nil, time.Time{}, reason
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, (4<<20)+1))
	if err != nil || len(b) > 4<<20 || !json.Valid(b) {
		return nil, time.Time{}, "invalid_response"
	}
	// FeatureCollection links can echo the credential. Never persist it.
	b = bytes.ReplaceAll(b, []byte(p.config.APIKey), []byte("[REDACTED]"))
	if featureLatest {
		b = p.collectFeaturePages(ctx, u, b)
	}
	// Validate before caching. A malformed HTTP 200 is not an empty flood result.
	e := gatewayFloodEvidence{}
	reason := ""
	if path == "flood-recurrence" {
		reason = parseGatewayHistory(b, &e)
	} else if path == "flood-mapped-history" {
		reason = parseGatewayMappedHistory(b, &e, lat, lon)
	} else {
		reason = p.parseLatest(b, &e, time.Now().UTC(), lat, lon)
	}
	if reason != "" {
		return nil, time.Time{}, reason
	}
	now := time.Now().UTC()
	saved, _ := json.Marshal(cachedResponse{b, now})
	_ = p.cache.Set(ctx, cacheKey, saved, ttl)
	return b, now, ""
}

func parseGatewayHistory(b []byte, e *gatewayFloodEvidence) string {
	var empty struct {
		Result string `json:"result"`
	}
	if json.Unmarshal(b, &empty) == nil && empty.Result == "not found" {
		e.Reason = "no_record"
		return ""
	}
	var rows []struct {
		Total  *int `json:"total"`
		Detail []struct {
			Year string `json:"year"`
			Freq *int   `json:"freq"`
		} `json:"detail"`
	}
	if json.Unmarshal(b, &rows) != nil || rows == nil {
		return "invalid_response"
	}
	if len(rows) == 0 {
		e.Reason = "no_record"
		return ""
	}
	years := map[int]bool{}
	for _, r := range rows {
		if r.Total == nil || *r.Total < 0 || len(r.Detail) != 13 {
			return "invalid_response"
		}
		seen := map[int]bool{}
		total := 0
		for _, d := range r.Detail {
			year, err := strconv.Atoi(d.Year)
			if err != nil || year < 2011 || year > 2023 || seen[year] || d.Freq == nil || *d.Freq < 0 || *d.Freq > 10000 {
				return "invalid_response"
			}
			seen[year] = true
			total += *d.Freq
			if *d.Freq > 0 {
				years[year] = true
			}
		}
		if total != *r.Total {
			return "invalid_response"
		}
	}
	e.Complete = true
	e.YearsObserved = []int{}
	for y := range years {
		e.YearsObserved = append(e.YearsObserved, y)
	}
	sort.Ints(e.YearsObserved)
	return ""
}

func (p *GISTDAGatewayFloodProvider) parseLatest(b []byte, e *gatewayFloodEvidence, now time.Time, lat, lon float64) string {
	if p.config.LatestURL == "" {
		return parseGatewayLatest(b, e, now)
	}
	count, overlap, complete, err := parseRecentFlood(b, lon, lat)
	if err != nil {
		return "invalid_response"
	}
	e.FeatureCount = &count
	e.SiteOverlapCount = &overlap
	e.Complete = complete
	if count == 0 {
		e.Result = "unknown"
		e.Reason = "no_observations_in_window"
		e.Complete = false
		return ""
	}
	if !complete && overlap == 0 {
		e.Result = "unknown"
		e.Reason = "incomplete_response"
		return ""
	}
	e.Result = "not found"
	if overlap > 0 {
		e.Result = "found"
	}
	// Product-name dates are useful context but explicitly labelled as inferred.
	// Collection timeStamp and record _createdAt are never acquisition dates.
	var collection struct {
		Features []struct {
			Properties struct {
				FileName string `json:"file_name"`
			} `json:"properties"`
		} `json:"features"`
	}
	_ = json.Unmarshal(b, &collection)
	dates := map[string]bool{}
	for _, f := range collection.Features {
		for _, match := range floodProductDatePattern.FindAllStringSubmatch(f.Properties.FileName, -1) {
			date, err := time.Parse("20060102", match[1])
			if err == nil && !date.After(now.Add(24*time.Hour)) && date.Year() >= 2000 {
				dates[date.Format("2006-01-02")] = true
			}
		}
	}
	for date := range dates {
		e.ObservationDates = append(e.ObservationDates, date)
	}
	sort.Strings(e.ObservationDates)
	if len(e.ObservationDates) == 0 {
		e.Reason = "observation_date_unknown"
	} else {
		e.DateBasis = "satellite_product_filename"
		latest, _ := time.Parse("2006-01-02", e.ObservationDates[len(e.ObservationDates)-1])
		if now.Sub(latest) > time.Duration(e.WindowDays+3)*24*time.Hour {
			e.Reason = "stale_data"
		}
	}
	return ""
}

var floodProductDatePattern = regexp.MustCompile(`\b[A-Za-z0-9]+_(\d{8})_\d{4}\b`)

func parseGatewayLatest(b []byte, e *gatewayFloodEvidence, now time.Time) string {
	var r struct {
		Result string `json:"result"`
		Data   []struct {
			Update string `json:"update"`
		} `json:"data"`
	}
	if json.Unmarshal(b, &r) != nil || (r.Result != "found" && r.Result != "not found") {
		return "invalid_response"
	}
	if (r.Result == "found" && len(r.Data) == 0) || (r.Result == "not found" && len(r.Data) > 0) {
		return "invalid_response"
	}
	e.Result = r.Result
	e.Complete = true
	seen := map[string]bool{}
	for _, d := range r.Data {
		date, err := time.Parse("2006-01-02", d.Update)
		if err != nil || date.After(now.Add(24*time.Hour)) {
			return "invalid_response"
		}
		if now.Sub(date) > 72*time.Hour {
			e.Reason = "stale_data"
		}
		seen[d.Update] = true
	}
	for d := range seen {
		e.ObservationDates = append(e.ObservationDates, d)
	}
	sort.Strings(e.ObservationDates)
	if len(e.ObservationDates) == 0 {
		e.Reason = "observation_date_unknown"
	}
	return ""
}

var _ AnalysisProvider = (*GISTDAGatewayFloodProvider)(nil)
