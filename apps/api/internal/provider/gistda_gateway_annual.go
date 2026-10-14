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
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type gatewayAnnualEvidence struct {
	Year              int        `json:"year"`
	Status            string     `json:"status"`
	Reason            string     `json:"reason,omitempty"`
	Scope             string     `json:"scope"`
	Province          string     `json:"province,omitempty"`
	District          string     `json:"district,omitempty"`
	DistrictID        int        `json:"districtId,omitempty"`
	RecordCount       int        `json:"recordCount"`
	ReportedAreaRai   *float64   `json:"reportedAreaRai,omitempty"`
	ReportedAreaValue *float64   `json:"reportedAreaValue,omitempty"`
	AreaUnit          string     `json:"areaUnit,omitempty"`
	Complete          bool       `json:"complete"`
	PartialYear       bool       `json:"partialYear"`
	RetrievedAt       *time.Time `json:"retrievedAt,omitempty"`
	SourceURI         string     `json:"sourceUri"`
}
type gatewaySTACLink struct {
	Rel  string `json:"rel"`
	Href string `json:"href"`
}

func (p *GISTDAGatewayFloodProvider) collectAnnual(ctx context.Context, lat, lon float64, radius int) []gatewayAnnualEvidence {
	return p.collectAnnualWithMapped(ctx, lat, lon, radius, nil)
}

func (p *GISTDAGatewayFloodProvider) collectAnnualWithMapped(ctx context.Context, lat, lon float64, radius int, mapped <-chan []byte) []gatewayAnnualEvidence {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	location := ReverseGeocodingResult{}
	if p.config.Geocoder != nil {
		gc, stop := context.WithTimeout(ctx, 5*time.Second)
		location, _ = p.config.Geocoder.Reverse(gc, lat, lon)
		stop()
	}
	districtID := registryDistrictCode(location)
	if districtID == 0 && location.Province != "" && location.District != "" {
		districtID = p.resolveAnnualDistrict(ctx, lat, lon, radius, location)
	}
	if mapped != nil && districtID == 0 {
		select {
		case b := <-mapped:
			districtID = annualDistrictCode(b, location)
		case <-ctx.Done():
		}
	}
	raw, _, reason := p.archiveJSON(ctx, p.config.ArchiveBaseURL+"/catalogs/flood_yearly", 24*time.Hour)
	var root struct {
		Links []gatewaySTACLink `json:"links"`
	}
	_ = json.Unmarshal(raw, &root)
	links := map[int]string{}
	for _, l := range root.Links {
		if l.Rel != "child" {
			continue
		}
		u, err := url.Parse(l.Href)
		if err != nil {
			continue
		}
		part := u.Path[strings.LastIndex(u.Path, "/")+1:]
		if !strings.HasPrefix(part, "flood_yearly_") {
			continue
		}
		year, err := strconv.Atoi(strings.TrimPrefix(part, "flood_yearly_"))
		if err == nil && year >= 2011 && year <= time.Now().Year() {
			links[year] = l.Href
		}
	}
	if reason != "" || len(links) == 0 {
		return []gatewayAnnualEvidence{{Year: time.Now().Year(), Status: "missing", Reason: "archive_catalog_unavailable", Scope: "search_bbox", PartialYear: true, SourceURI: "https://disaster.gistda.or.th/services/stac/flood/index"}}
	}
	years := []int{}
	for y := range links {
		years = append(years, y)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(years)))
	result := make([]gatewayAnnualEvidence, len(years))
	jobs := make(chan int, len(years))
	for i := range years {
		jobs <- i
	}
	close(jobs)
	var workers sync.WaitGroup
	for worker := 0; worker < 4; worker++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for i := range jobs {
				yearCtx, stop := context.WithTimeout(ctx, 5*time.Second)
				result[i] = p.collectAnnualYear(yearCtx, links[years[i]], years[i], lat, lon, radius, location, districtID)
				stop()
			}
		}()
	}
	workers.Wait()
	return result
}

func (p *GISTDAGatewayFloodProvider) collectAnnualYear(ctx context.Context, catalog string, year int, lat, lon float64, radius int, location ReverseGeocodingResult, codes ...int) gatewayAnnualEvidence {
	e := gatewayAnnualEvidence{Year: year, Status: "missing", Scope: "search_bbox", PartialYear: year == time.Now().Year(), SourceURI: "https://disaster.gistda.or.th/services/stac/flood/index"}
	if location.Province != "" && location.District != "" {
		e.Scope = "district_records_in_search_bbox"
		e.Province = location.Province
		e.District = location.District
	}
	if len(codes) > 0 && codes[0] > 0 {
		e.DistrictID = codes[0]
		e.Scope = "district_summary"
	}
	raw, _, reason := p.archiveJSON(ctx, catalog, 24*time.Hour)
	if reason != "" {
		e.Reason = reason
		return e
	}
	var cat struct {
		Links []gatewaySTACLink `json:"links"`
	}
	if json.Unmarshal(raw, &cat) != nil {
		e.Reason = "invalid_response"
		return e
	}
	collections := []string{}
	for _, l := range cat.Links {
		u, err := url.Parse(l.Href)
		if l.Rel == "child" && err == nil && strings.Contains(u.Path, "/collections/") {
			collections = append(collections, u.Path[strings.LastIndex(u.Path, "/")+1:])
		}
	}
	if len(collections) == 0 {
		e.Reason = "no_archive_collection"
		return e
	}
	search, _ := url.Parse(p.config.ArchiveBaseURL + "/search")
	q := search.Query()
	q.Set("collections", strings.Join(collections, ","))
	q.Set("limit", "100")
	search.RawQuery = q.Encode()
	// STAC item footprints were stale in live checks (2026 index bounds omitted
	// Bangkok although its Features asset contains Bangkok rows). Query item
	// metadata without bbox, then apply bbox to the actual data asset instead.
	raw, _, reason = p.archiveJSON(ctx, search.String(), 24*time.Hour)
	if reason != "" {
		e.Reason = reason
		return e
	}
	var items struct {
		Type    string `json:"type"`
		Context struct {
			Matched *int `json:"matched"`
		} `json:"context"`
		Features []struct {
			Assets map[string]struct {
				Href string `json:"href"`
				Type string `json:"type"`
			} `json:"assets"`
		} `json:"features"`
	}
	if json.Unmarshal(raw, &items) != nil || items.Type != "FeatureCollection" || items.Context.Matched == nil || *items.Context.Matched != len(items.Features) || len(items.Features) == 0 {
		e.Reason = "incomplete_archive_index"
		return e
	}
	dLat := float64(radius) / 111320
	dLon := dLat / math.Max(math.Cos(lat*math.Pi/180), 0.01)
	bbox := strconv.FormatFloat(lon-dLon, 'f', 6, 64) + "," + strconv.FormatFloat(lat-dLat, 'f', 6, 64) + "," + strconv.FormatFloat(lon+dLon, 'f', 6, 64) + "," + strconv.FormatFloat(lat+dLat, 'f', 6, 64)
	totals := map[string]float64{}
	raiUnits := true
	complete := true
	checked := 0
	for _, item := range items.Features {
		asset, ok := item.Assets["data"]
		if !ok || asset.Type != "application/geo+json" {
			e.Reason = "unsupported_archive_asset"
			return e
		}
		u, err := url.Parse(asset.Href)
		if err != nil {
			e.Reason = "invalid_response"
			return e
		}
		q := u.Query()
		q.Set("bbox", bbox)
		if e.DistrictID > 0 {
			q.Del("bbox")
			q.Set("ap_idn", strconv.Itoa(e.DistrictID))
		}
		q.Set("limit", "1000")
		q.Set("offset", "0")
		u.RawQuery = q.Encode()
		body, fetched, reason := p.archiveJSON(ctx, u.String(), time.Hour)
		if reason != "" {
			e.Reason = reason
			return e
		}
		e.RetrievedAt = &fetched
		_, _, all, err := parseRecentFlood(body, lon, lat)
		if err != nil {
			e.Reason = "invalid_response"
			return e
		}
		complete = complete && all
		checked++
		var data struct {
			Features []struct {
				Properties struct {
					Province    string          `json:"pv_tn"`
					District    string          `json:"ap_tn"`
					Subdistrict string          `json:"tb_tn"`
					ID          json.RawMessage `json:"tb_idn"`
					DistrictID  *int            `json:"ap_idn"`
					ProvinceID  *int            `json:"pv_idn"`
					Area        *float64        `json:"flood_area"`
					AreaRai     *float64        `json:"area_rai"`
					Year        string          `json:"year"`
				} `json:"properties"`
			} `json:"features"`
		}
		_ = json.Unmarshal(body, &data)
		for _, f := range data.Features {
			v := f.Properties
			matches := true
			if e.DistrictID > 0 && v.DistrictID != nil && v.ProvinceID != nil {
				// Some 2024/2025 Thai labels are encoding-corrupted at source.
				// Match both numeric administrative codes instead of repairing or
				// guessing those labels; reject contradictory province/district IDs.
				matches = *v.DistrictID == e.DistrictID && *v.ProvinceID == e.DistrictID/100
			} else if e.District != "" {
				matches = annualAreaName(v.District) == annualAreaName(e.District) && annualAreaName(v.Province) == annualAreaName(e.Province)
			}
			if !matches {
				if e.DistrictID > 0 {
					complete = false
				}
				continue
			}
			if v.Area == nil && v.Year == strconv.Itoa(year) {
				v.Area = v.AreaRai
			} else if v.Area != nil {
				raiUnits = false
			}
			if v.Area == nil || *v.Area < 0 || math.IsNaN(*v.Area) || math.IsInf(*v.Area, 0) {
				e.Reason = "unsupported_archive_schema"
				return e
			}
			key := string(v.ID)
			if key == "" || key == "null" {
				key = v.Province + "|" + v.District + "|" + v.Subdistrict
			}
			if *v.Area > totals[key] {
				totals[key] = *v.Area
			}
		}
	}
	if checked == 0 {
		e.Reason = "no_archive_collection"
		return e
	}
	area := 0.0
	for _, v := range totals {
		if v > 0 {
			e.RecordCount++
			area += v
		}
	}
	e.ReportedAreaValue = &area
	e.AreaUnit = "source_unspecified"
	if raiUnits {
		e.ReportedAreaRai = &area
		e.AreaUnit = "rai"
	}
	e.Complete = complete
	e.Status = "available"
	if !complete {
		e.Reason = "partial_results"
	} else if e.RecordCount == 0 {
		e.Reason = "no_published_record_in_search_area"
	}
	return e
}

func annualAreaName(s string) string {
	return normalizeDPMAreaName(strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(s), "อ."), "จ."))
}

func annualDistrictCode(b []byte, location ReverseGeocodingResult) int {
	if location.Province == "" || location.District == "" {
		return 0
	}
	var collection struct {
		Features []struct {
			Properties struct {
				Province string `json:"pv_tn"`
				District string `json:"ap_tn"`
				Code     *int   `json:"ap_idn"`
			} `json:"properties"`
		} `json:"features"`
	}
	if json.Unmarshal(b, &collection) != nil {
		return 0
	}
	codes := map[int]bool{}
	for _, f := range collection.Features {
		v := f.Properties
		if v.Code != nil && *v.Code > 0 && annualAreaName(v.Province) == annualAreaName(location.Province) && annualAreaName(v.District) == annualAreaName(location.District) {
			codes[*v.Code] = true
		}
	}
	if len(codes) != 1 {
		return 0
	}
	for code := range codes {
		return code
	}
	return 0
}

func (p *GISTDAGatewayFloodProvider) resolveAnnualDistrict(ctx context.Context, lat, lon float64, radius int, location ReverseGeocodingResult) int {
	if p.config.LatestURL == "" {
		return 0
	}
	ctx, stop := context.WithTimeout(ctx, 4*time.Second)
	defer stop()
	u, err := url.Parse(p.config.LatestURL)
	if err != nil {
		return 0
	}
	u.Path = strings.TrimSuffix(u.Path, "/flood/1day") + "/flood-freq"
	dLat := float64(radius) / 111320
	dLon := dLat / math.Max(math.Cos(lat*math.Pi/180), 0.01)
	q := u.Query()
	q.Set("bbox", strconv.FormatFloat(lon-dLon, 'f', 6, 64)+","+strconv.FormatFloat(lat-dLat, 'f', 6, 64)+","+strconv.FormatFloat(lon+dLon, 'f', 6, 64)+","+strconv.FormatFloat(lat+dLat, 'f', 6, 64))
	q.Set("limit", "1")
	// Administrative IDs can be read from a small validated name match, without
	// downloading all large recurrence polygons just to resolve an amphoe code.
	for offset := 0; offset < 8; offset++ {
		q.Set("offset", strconv.Itoa(offset))
		u.RawQuery = q.Encode()
		b, _, reason := p.archiveJSON(ctx, u.String(), 24*time.Hour, false)
		if reason != "" {
			return 0
		}
		if code := annualDistrictCode(b, location); code > 0 {
			return code
		}
		var rows struct {
			Features []json.RawMessage `json:"features"`
		}
		if json.Unmarshal(b, &rows) != nil || len(rows.Features) == 0 {
			return 0
		}
	}
	return 0
}

func (p *GISTDAGatewayFloodProvider) archiveJSON(ctx context.Context, target string, ttl time.Duration, paginate ...bool) ([]byte, time.Time, string) {
	base, err := url.Parse(p.config.ArchiveBaseURL)
	if err != nil {
		return nil, time.Time{}, "invalid_endpoint"
	}
	u, err := url.Parse(target)
	if err != nil || u.Scheme != base.Scheme || u.Host != base.Host || u.User != nil {
		return nil, time.Time{}, "invalid_endpoint"
	}
	q := u.Query()
	q.Del("token")
	q.Del("key")
	q.Set("api_key", p.config.APIKey)
	u.RawQuery = q.Encode()
	fingerprint := sha256.Sum256([]byte(u.String()))
	key := "gistda:annual:v1:" + hex.EncodeToString(fingerprint[:])
	type saved struct {
		Payload     json.RawMessage `json:"payload"`
		RetrievedAt time.Time       `json:"retrievedAt"`
	}
	if b, found, err := p.cache.Get(ctx, key); err == nil && found {
		var v saved
		if json.Unmarshal(b, &v) == nil && !v.RetrievedAt.IsZero() {
			return v.Payload, v.RetrievedAt, ""
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, time.Time{}, "invalid_endpoint"
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", p.config.UserAgent)
	r, err := p.client.Do(req)
	if err != nil {
		return nil, time.Time{}, "service_unavailable"
	}
	defer r.Body.Close()
	if r.StatusCode != 200 {
		reason := "service_unavailable"
		if r.StatusCode == 401 || r.StatusCode == 403 || r.StatusCode == 407 {
			reason = "unauthorized"
		}
		return nil, time.Time{}, reason
	}
	b, err := io.ReadAll(io.LimitReader(r.Body, (4<<20)+1))
	if err != nil || len(b) > 4<<20 || !json.Valid(b) {
		return nil, time.Time{}, "invalid_response"
	}
	b = bytes.ReplaceAll(b, []byte(p.config.APIKey), []byte("[REDACTED]"))
	var marker struct {
		Type string `json:"type"`
	}
	if json.Unmarshal(b, &marker) != nil || (marker.Type != "Catalog" && marker.Type != "FeatureCollection" && marker.Type != "Collection") {
		return nil, time.Time{}, "invalid_response"
	}
	if (len(paginate) == 0 || paginate[0]) && (strings.Contains(q.Get("bbox"), ",") || q.Get("ap_idn") != "") {
		b = p.collectFeaturePages(ctx, u, b)
	}
	now := time.Now().UTC()
	out, _ := json.Marshal(saved{Payload: b, RetrievedAt: now})
	_ = p.cache.Set(ctx, key, out, ttl)
	return b, now, ""
}
