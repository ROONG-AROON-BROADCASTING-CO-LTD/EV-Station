package advisory

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"

	"github.com/rbc/ev-station/apps/api/internal/domain"
)

type StationText struct {
	Reason      string   `json:"reason"`
	Assumptions []string `json:"assumptions"`
	MissingData []string `json:"missingData"`
}

type StationRecommendation struct {
	PowerKW               int         `json:"powerKw"`
	ChargerCount          int         `json:"chargerCount"`
	TotalPowerKW          int         `json:"totalPowerKw"`
	InstallationConfirmed bool        `json:"installationConfirmed"`
	LandAreaSqWah         float64     `json:"landAreaSqWah"`
	PreliminaryCabinetLimit int       `json:"preliminaryCabinetLimit"`
	TH                    StationText `json:"th"`
	EN                    StationText `json:"en"`
}

// Station proposes a planning scenario; public grid proximity never confirms supply.
func (s *Service) Station(ctx context.Context, run domain.AnalysisRun, site domain.Site) (StationRecommendation, error) {
	if strings.TrimSpace(s.config.APIKey) == "" {
		return StationRecommendation{}, ErrNotConfigured
	}
	facts, err := compactRunFacts(run)
	if err != nil {
		return StationRecommendation{}, err
	}
	landAreaSqWah := areaInSquareWah(site)
	preliminaryCabinetLimit := cabinetLimitFromLandArea(landAreaSqWah)
	facts["land"] = map[string]any{"size": site.LandSize, "unit": site.LandSizeUnit, "areaSqWah": landAreaSqWah, "frontageMeters": site.FrontageMeters, "preliminaryCabinetLimit": preliminaryCabinetLimit}
	delete(facts, "financialEstimate")
	data, err := json.Marshal(facts)
	if err != nil {
		return StationRecommendation{}, err
	}
	prompt := `Create a PRELIMINARY EV charging-station planning recommendation from the supplied screening evidence. This is the system's main purpose: help a team and customer screen a site and plan an initial investment before an electrician and the utility inspect the actual site. A charger means one physical DC cabinet, not a connector or site.

Always choose 120, 180, or 240 kW per cabinet and a positive integer cabinet count. Choose conservatively when evidence is limited: prefer 120 kW and one cabinet rather than returning no recommendation. Use traffic, registered EV trend, population, mapped places, competitors, land size, frontage if supplied, and published PEA/MEA electrical-map proximity as combined preliminary evidence. A nearby published high-voltage line or station is a positive planning signal, not confirmation of supply. Do not infer actual simultaneous charging sessions from provincial EV registration or AADT alone. Do not use overall score alone to size chargers. Do not claim installation is confirmed, grid capacity is available, a particular cable route is possible, or a land area alone determines the maximum cabinet count.

The facts include preliminaryCabinetLimit. It is a preliminary upper limit calculated only from the submitted land area using one cabinet per complete 100 square wah, with a minimum of one cabinet. Choose a count from 1 through that limit. State in both languages that frontage, traffic flow, required parking bays, utility capacity and the actual layout can reduce this number after inspection.

Clearly state that the output is a preliminary recommendation. Explain the reason for the selected power and cabinet count, list assumptions used, and list the site layout, utility capacity, connection point, electrical design, and cost items that an electrician/PEA/MEA must confirm before installation. Return identical facts and translated explanations in Thai and English; reason, assumptions, missingData are required in each. No ROI, prices or payback. Return JSON only. FACTS: ` + string(data)
	request := scoringRequest(prompt)
	textSchema := map[string]any{"type": "object", "properties": map[string]any{"reason": map[string]any{"type": "string"}, "assumptions": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}, "missingData": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}}, "required": []string{"reason", "assumptions", "missingData"}, "additionalProperties": false}
	request.GenerationConfig.ResponseJSONSchema = map[string]any{"type": "object", "properties": map[string]any{"powerKw": map[string]any{"type": "integer", "enum": []int{120, 180, 240}}, "chargerCount": map[string]any{"type": "integer", "minimum": 1, "maximum": preliminaryCabinetLimit}, "th": textSchema, "en": textSchema}, "required": []string{"powerKw", "chargerCount", "th", "en"}, "additionalProperties": false}
	request.GenerationConfig.MaxOutputTokens = 2500
	body, err := json.Marshal(request)
	if err != nil {
		return StationRecommendation{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(s.config.BaseURL, "/")+"/models/"+url.PathEscape(s.config.Model)+":generateContent", bytes.NewReader(body))
	if err != nil {
		return StationRecommendation{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", s.config.APIKey)
	response, err := s.client.Do(req)
	if err != nil {
		return StationRecommendation{}, ErrUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return StationRecommendation{}, ErrUnavailable
	}
	var generated geminiResponse
	if json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&generated) != nil || len(generated.Candidates) == 0 || len(generated.Candidates[0].Content.Parts) == 0 {
		return StationRecommendation{}, ErrInvalidOutput
	}
	var result StationRecommendation
	if json.Unmarshal([]byte(generated.Candidates[0].Content.Parts[0].Text), &result) != nil || !validStation(result) {
		return StationRecommendation{}, ErrInvalidOutput
	}
	result.LandAreaSqWah = landAreaSqWah
	result.PreliminaryCabinetLimit = preliminaryCabinetLimit
	if result.ChargerCount > preliminaryCabinetLimit {
		result.ChargerCount = preliminaryCabinetLimit
	}
	result.TotalPowerKW = result.PowerKW * result.ChargerCount
	result.InstallationConfirmed = false
	return result, nil
}

func areaInSquareWah(site domain.Site) float64 {
	switch strings.ToLower(strings.TrimSpace(site.LandSizeUnit)) {
	case "rai":
		return site.LandSize * 400
	case "ngan":
		return site.LandSize * 100
	case "sqm":
		return site.LandSize / 4
	default:
		return site.LandSize
	}
}

func cabinetLimitFromLandArea(areaSqWah float64) int {
	if areaSqWah <= 0 {
		return 1
	}
	return max(1, int(math.Floor(areaSqWah/100)))
}

func validStation(r StationRecommendation) bool {
	return (r.PowerKW == 120 || r.PowerKW == 180 || r.PowerKW == 240) && r.ChargerCount >= 1 && r.ChargerCount <= 100 && strings.TrimSpace(r.TH.Reason) != "" && strings.TrimSpace(r.EN.Reason) != "" && r.TH.Assumptions != nil && r.EN.Assumptions != nil && r.TH.MissingData != nil && r.EN.MissingData != nil
}
