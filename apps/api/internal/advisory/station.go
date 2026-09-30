package advisory

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/rbc/ev-station/apps/api/internal/domain"
)

type StationText struct {
	Reason      string   `json:"reason"`
	Assumptions []string `json:"assumptions"`
	MissingData []string `json:"missingData"`
}

type StationRecommendation struct {
	RecommendationAvailable bool        `json:"recommendationAvailable"`
	Blocker                 string      `json:"blocker,omitempty"`
	CapacityConfirmed       bool        `json:"capacityConfirmed"`
	RecommendationStage     string      `json:"recommendationStage"`
	PowerKW                 int         `json:"powerKw"`
	ChargerCount            int         `json:"chargerCount"`
	TotalPowerKW            int         `json:"totalPowerKw"`
	InstallationConfirmed   bool        `json:"installationConfirmed"`
	LandAreaSqWah           float64     `json:"landAreaSqWah"`
	FranchisePackage        string      `json:"franchisePackage"`
	RecommendedAreaSqWah    float64     `json:"recommendedAreaSqWah"`
	LayoutCabinetLimit      int         `json:"layoutCabinetLimit"`
	ElectricalCabinetLimit  int         `json:"electricalCabinetLimit"`
	PreliminaryCabinetLimit int         `json:"preliminaryCabinetLimit"`
	TH                      StationText `json:"th"`
	EN                      StationText `json:"en"`
	GeneratedByAI           bool        `json:"-"`
}

// Station proposes a planning scenario; public grid proximity never confirms supply.
func (s *Service) Station(ctx context.Context, run domain.AnalysisRun, site domain.Site) (StationRecommendation, error) {
	landAreaSqWah := areaInSquareWah(site)
	franchisePlan := franchiseLayoutForSite(site, landAreaSqWah)
	if blocker := StationBlocker(site); blocker != "" {
		return blockedStationRecommendation(landAreaSqWah, franchisePlan, blocker), nil
	}
	layoutCabinetLimit := franchisePlan.CabinetLimit
	electricalCabinetLimit := electricalPlanningCabinetLimit(run)
	// S/M/L is a site-layout recommendation. Published map voltage and distance
	// are useful risk evidence, but cannot reduce the advertised package count:
	// they do not reveal remaining capacity or the feasible connection design.
	cabinetLimit := layoutCabinetLimit
	capacityConfirmed := hasConfirmedElectricalCapacity(run)
	if strings.TrimSpace(s.config.APIKey) == "" {
		if !capacityConfirmed {
			// Preserve a useful planning recommendation if AI is not
			// configured. It follows the package layout but does not claim that
			// traffic, demand, or utility capacity has been assessed.
			return initialPhaseStationRecommendation(landAreaSqWah, franchisePlan, electricalCabinetLimit, cabinetLimit), nil
		}
		return StationRecommendation{}, ErrNotConfigured
	}
	facts, err := compactRunFacts(run)
	if err != nil {
		return StationRecommendation{}, err
	}
	facts["land"] = map[string]any{"size": site.LandSize, "unit": site.LandSizeUnit, "areaSqWah": landAreaSqWah, "frontageMeters": site.FrontageMeters, "franchisePackage": franchisePlan.Code, "recommendedAreaSqWah": franchisePlan.RecommendedAreaSqWah, "layoutCabinetLimit": layoutCabinetLimit, "electricalCabinetLimit": electricalCabinetLimit}
	delete(facts, "financialEstimate")
	data, err := json.Marshal(facts)
	if err != nil {
		return StationRecommendation{}, err
	}
	powerOptions := []int{120, 180, 240}
	utilityContext := "The utility has already confirmed capacity and a feasible connection point for this site; final electrical design is still required."
	if !capacityConfirmed {
		powerOptions = []int{120}
		utilityContext = "The utility has NOT confirmed capacity or a feasible connection point. You may use published electrical-map evidence only as risk context. Recommend 120 kW per cabinet only and never claim that supply, a cable route, or installation is available."
	}
	prompt := `Create an EV charging-station recommendation from the supplied site evidence. ` + utilityContext + ` Recommend a site configuration that helps the team and customer decide what station setup to pursue. The recommendation is planning guidance and is not confirmation of a utility connection or installation approval. A charger means one physical DC cabinet, not a connector or site.

Use all available evidence together: traffic, registered-EV trend, population, mapped places, competitors, land size, frontage if supplied, and published PEA/MEA electrical-map proximity. The S/M/L package and layoutCabinetLimit are physical layout ceilings, not the answer by themselves. Choose a conservative cabinet count from 1 through that ceiling: lower it when demand, traffic, competition, or evidence quality does not support the full package; use the full ceiling only when the combined evidence supports it. Treat a 7 m entrance/exit only as a customer-facing layout recommendation; do not block or reduce the package because its width is missing or below 7 m. A nearby published high-voltage line or station is a positive planning signal, not confirmation of supply. Do not infer actual simultaneous charging sessions from provincial EV registration or AADT alone. Do not use overall score alone to size chargers. Do not claim installation is confirmed, grid capacity is available, a particular cable route is possible, or that land area alone determines the recommended cabinet count.

The facts include layoutCabinetLimit. It is the S/M/L layout ceiling (1, 2, or 4 cabinets), based on the supplied total-site area, not an electrical-capacity calculation. Published grid voltage/distance is risk context only and must not be treated as remaining capacity. State in both languages which combined evidence supports the count and that utility capacity, traffic flow, required parking bays, and the actual layout must be verified before installation.

Do not describe or label the recommendation as preliminary or เบื้องต้น. Explain the recommendation for power and cabinet count, list assumptions used, and list the site layout, utility capacity, connection point, transformer sizing, electrical design, and cost items that an engineer and PEA/MEA must confirm before installation. Do not assume that a particular transformer size or utility capacity is available. Return identical facts and translated explanations in Thai and English; reason, assumptions, missingData are required in each. No ROI, prices or payback. Return JSON only. FACTS: ` + string(data)
	prompt += " Flood-related access risks must be discussed even when the overall location score is strong. District-level flood reports do not confirm flooding on the frontage road. Include verification of frontage-road and approach-route flood history among required checks when road-specific evidence is absent. RBC's 120 kW project approach uses a charging-station transformer separate from the cafe supply; actual sizing and connection remain subject to engineering and utility review."
	request := scoringRequest(prompt)
	textSchema := map[string]any{"type": "object", "properties": map[string]any{"reason": map[string]any{"type": "string"}, "assumptions": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}, "missingData": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}}, "required": []string{"reason", "assumptions", "missingData"}, "additionalProperties": false}
	request.GenerationConfig.ResponseJSONSchema = map[string]any{"type": "object", "properties": map[string]any{"powerKw": map[string]any{"type": "integer", "enum": powerOptions}, "chargerCount": map[string]any{"type": "integer", "minimum": 1, "maximum": cabinetLimit}, "th": textSchema, "en": textSchema}, "required": []string{"powerKw", "chargerCount", "th", "en"}, "additionalProperties": false}
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
	if json.Unmarshal([]byte(generated.Candidates[0].Content.Parts[0].Text), &result) != nil {
		return StationRecommendation{}, ErrInvalidOutput
	}
	// Gemini returns only the fields in the response schema. Mark this as an
	// available recommendation before validating its power, cabinet count, and
	// bilingual explanation.
	result.RecommendationAvailable = true
	if !validStation(result) {
		return StationRecommendation{}, ErrInvalidOutput
	}
	result.LandAreaSqWah = landAreaSqWah
	result.FranchisePackage = franchisePlan.Code
	result.RecommendedAreaSqWah = franchisePlan.RecommendedAreaSqWah
	result.LayoutCabinetLimit = layoutCabinetLimit
	result.ElectricalCabinetLimit = electricalCabinetLimit
	result.PreliminaryCabinetLimit = cabinetLimit
	result.RecommendationAvailable = true
	result.CapacityConfirmed = capacityConfirmed
	if capacityConfirmed {
		result.RecommendationStage = "utility_confirmed"
	} else {
		result.RecommendationStage = "screening_evidence"
	}
	result.GeneratedByAI = true
	if result.ChargerCount > cabinetLimit {
		result.ChargerCount = cabinetLimit
	}
	result.TotalPowerKW = result.PowerKW * result.ChargerCount
	result.InstallationConfirmed = false
	return result, nil
}

// StationBlocker contains commercial conditions that stop a recommendation.
// A 7 m entrance is a layout recommendation for the customer, not a blocker.
// Unknown electricity is deliberately not treated as underground; it requires
// evidence from the site or PEA/MEA rather than a province-level guess.
func StationBlocker(site domain.Site) string {
	if site.ElectricalSupplyType == "underground" {
		return "underground_electricity"
	}
	return ""
}

func blockedStationRecommendation(landAreaSqWah float64, franchisePlan franchiseLayoutPlan, blocker string) StationRecommendation {
	thReason, enReason := "", ""
	if blocker == "underground_electricity" {
		thReason = "จุดเชื่อมต่อระบุว่าเป็นสายไฟฟ้าใต้ดิน ซึ่งนโยบายธุรกิจปัจจุบันยังไม่ลงทุนเพราะต้นทุนงานโยธาและการเชื่อมต่อสูง"
		enReason = "The connection point is recorded as underground electricity. The current business policy does not invest because civil-work and connection costs are high."
	}
	return StationRecommendation{
		RecommendationAvailable: false, Blocker: blocker, RecommendationStage: "blocked",
		LandAreaSqWah: landAreaSqWah, FranchisePackage: franchisePlan.Code, RecommendedAreaSqWah: franchisePlan.RecommendedAreaSqWah,
		LayoutCabinetLimit: franchisePlan.CabinetLimit,
		TH:                 StationText{Reason: thReason, Assumptions: []string{"ไฟฟ้าใต้ดินต้องยืนยันจากหน้างานหรือ PEA/MEA ไม่อนุมานจากจังหวัด"}, MissingData: []string{"หากต้องการทบทวน: หลักฐานรูปแบบระบบไฟฟ้าจาก PEA/MEA"}},
		EN:                 StationText{Reason: enReason, Assumptions: []string{"Underground electricity must be confirmed at the site or by PEA/MEA; it is never inferred from province."}, MissingData: []string{"To reconsider: PEA/MEA evidence of the supply type."}},
	}
}

// hasConfirmedElectricalCapacity accepts only a utility-confirmed capacity
// record. Published grid layers and proximity evidence are intentionally not
// sufficient to size chargers.
func hasConfirmedElectricalCapacity(run domain.AnalysisRun) bool {
	for _, metric := range run.Metrics {
		if metric.Type == "electrical" && metric.Status == domain.DataVerified && metric.Source.SiteVerification == "utility_capacity_confirmed" {
			return true
		}
	}
	return false
}

// initialPhaseStationRecommendation uses the supplied S/M/L layout. Published
// utility-map voltage and distance remain risk evidence; they do not prove
// capacity and must not reduce the layout's cabinet quantity.
func initialPhaseStationRecommendation(landAreaSqWah float64, franchisePlan franchiseLayoutPlan, electricalCabinetLimit, preliminaryCabinetLimit int) StationRecommendation {
	return StationRecommendation{
		RecommendationAvailable: true,
		CapacityConfirmed:       false,
		RecommendationStage:     "initial_phase",
		PowerKW:                 120,
		ChargerCount:            preliminaryCabinetLimit,
		TotalPowerKW:            120 * preliminaryCabinetLimit,
		InstallationConfirmed:   false,
		LandAreaSqWah:           landAreaSqWah,
		FranchisePackage:        franchisePlan.Code,
		RecommendedAreaSqWah:    franchisePlan.RecommendedAreaSqWah,
		LayoutCabinetLimit:      franchisePlan.CabinetLimit,
		ElectricalCabinetLimit:  electricalCabinetLimit,
		PreliminaryCabinetLimit: preliminaryCabinetLimit,
		TH: StationText{
			Reason:      "เสนอรูปแบบ " + franchisePlan.Code + " ตามมาตรฐานพื้นที่แนะนำ " + formatSqWah(franchisePlan.RecommendedAreaSqWah) + " ตร.วา สำหรับ " + strconv.Itoa(franchisePlan.CabinetLimit) + " สถานีชาร์จ และแนะนำ 120 kW จำนวน " + strconv.Itoa(preliminaryCabinetLimit) + " ตู้",
			Assumptions: []string{"พื้นที่แนะนำ 100 / 200 / 400 ตร.วา ของรูปแบบ S / M / L เป็นพื้นที่รวมร้านกาแฟ พื้นที่บริการ ทางเข้า และสถานีชาร์จแล้ว จึงไม่หักพื้นที่ร้านกาแฟซ้ำ", "จำนวนตู้ตามรูปแบบแฟรนไชส์จากขนาดพื้นที่; ทางเข้า–ออกอย่างน้อย 7 ม. เป็นคำแนะนำการวางผังเท่านั้น", "แรงดันและระยะจากชั้นข้อมูลสาธารณะเป็นข้อมูลประกอบการวางแผน ไม่ใช่กำลังไฟคงเหลือ ระยะเดินสายจริง หรือการอนุมัติจุดเชื่อมต่อ"},
			MissingData: []string{"หนังสือหรือผลสำรวจจาก PEA/MEA ที่ยืนยันกำลังไฟและจุดเชื่อมต่อ", "แบบระบบไฟฟ้าและขนาดหม้อแปลงที่วิศวกรตรวจสอบ รวมถึงแนวเดินสายและค่าใช้จ่ายติดตั้ง", "ผังตำแหน่งตู้และช่องจอดจริง"},
		},
		EN: StationText{
			Reason:      "Recommend the " + franchisePlan.Code + " format with its recommended " + formatSqWah(franchisePlan.RecommendedAreaSqWah) + " sq wah total site area and " + strconv.Itoa(franchisePlan.CabinetLimit) + " charging stations; the recommended configuration is " + strconv.Itoa(preliminaryCabinetLimit) + " 120 kW cabinets.",
			Assumptions: []string{"The S / M / L recommended areas of 100 / 200 / 400 sq wah already include the café, service area, access, and charging stations, so café space is not deducted again.", "Cabinet quantity follows the franchise layout from land size; a 7 m entrance/exit is a layout recommendation only.", "Published voltage and distance are planning evidence only, not remaining capacity, an actual cable route, or a connection approval."},
			MissingData: []string{"PEA/MEA survey or written confirmation of capacity and connection point", "Engineer-reviewed electrical design and transformer sizing, cable route, and installation cost", "Actual cabinet and parking-bay layout"},
		},
	}
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
	if areaSqWah < 200 {
		return 1
	}
	if areaSqWah < 400 {
		return 2
	}
	return 4
}

func cabinetLimitFromSite(site domain.Site, areaSqWah float64) int {
	return franchiseLayoutForSite(site, areaSqWah).CabinetLimit
}

type franchiseLayoutPlan struct {
	Code                 string
	RecommendedAreaSqWah float64
	CabinetLimit         int
}

func franchiseLayoutForSite(_ domain.Site, areaSqWah float64) franchiseLayoutPlan {
	return franchiseLayoutForArea(areaSqWah)
}

func franchiseLayoutForArea(areaSqWah float64) franchiseLayoutPlan {
	switch {
	case areaSqWah < 200:
		return franchiseLayoutPlan{Code: "S", RecommendedAreaSqWah: 100, CabinetLimit: 1}
	case areaSqWah < 400:
		return franchiseLayoutPlan{Code: "M", RecommendedAreaSqWah: 200, CabinetLimit: 2}
	default:
		return franchiseLayoutPlan{Code: "L", RecommendedAreaSqWah: 400, CabinetLimit: 4}
	}
}

func formatSqWah(value float64) string {
	return strconv.FormatFloat(value, 'f', 0, 64)
}

var voltageNumberPattern = regexp.MustCompile(`(?i)(\d+(?:\.\d+)?)\s*(?:k\s*v|kv)?`)

// electricalPlanningCabinetLimit interprets only published voltage and the
// distance to the same published feature. It intentionally returns a planning
// ceiling, never a capacity estimate. A distant 22 kV line therefore cannot
// inflate a recommendation just because it has a high voltage label.
func electricalPlanningCabinetLimit(run domain.AnalysisRun) int {
	bestLimit := 1
	for _, metric := range run.Metrics {
		if metric.Type != "electrical" || metric.Status == domain.DataMissing || len(metric.RawValue) == 0 {
			continue
		}
		var value struct {
			VoltageCode                  string   `json:"voltageCode"`
			StationSecondaryVoltageKV    *float64 `json:"stationSecondaryVoltageKv"`
			VoltageKV                    *float64 `json:"voltageKv"`
			NearestHighVoltageLineMeters *float64 `json:"nearestHighVoltageLineMeters"`
			NearestStationMeters         *float64 `json:"nearestStationMeters"`
			DistanceToAreaMeters         *float64 `json:"distanceToAreaMeters"`
		}
		if json.Unmarshal(metric.RawValue, &value) != nil {
			continue
		}
		voltage := maxVoltage(value.VoltageCode, value.StationSecondaryVoltageKV, value.VoltageKV)
		distance := nearestDistance(value.NearestHighVoltageLineMeters, value.NearestStationMeters, value.DistanceToAreaMeters)
		if voltage <= 0 || math.IsInf(distance, 1) {
			continue
		}
		if limit := electricalLimitForEvidence(voltage, distance); limit > bestLimit {
			bestLimit = limit
		}
	}
	return bestLimit
}

func maxVoltage(code string, values ...*float64) float64 {
	best := 0.0
	for _, match := range voltageNumberPattern.FindAllStringSubmatch(code, -1) {
		if value, err := strconv.ParseFloat(match[1], 64); err == nil && value > best {
			best = value
		}
	}
	for _, value := range values {
		if value != nil && *value > best {
			best = *value
		}
	}
	return best
}

func nearestDistance(distances ...*float64) float64 {
	best := math.Inf(1)
	for _, distance := range distances {
		if distance != nil && *distance >= 0 && *distance < best {
			best = *distance
		}
	}
	return best
}

func electricalLimitForEvidence(voltageKV, distanceMeters float64) int {
	switch {
	case distanceMeters > 5_000:
		return 1
	case distanceMeters > 1_000:
		if voltageKV >= 22 {
			return 2
		}
		return 1
	case voltageKV >= 22:
		return 4
	case voltageKV >= 12:
		return 2
	default:
		return 1
	}
}

func minCabinetLimit(left, right int) int {
	if left < right {
		return left
	}
	return right
}

func validStation(r StationRecommendation) bool {
	textValid := strings.TrimSpace(r.TH.Reason) != "" && strings.TrimSpace(r.EN.Reason) != "" && r.TH.Assumptions != nil && r.EN.Assumptions != nil && r.TH.MissingData != nil && r.EN.MissingData != nil
	if !r.RecommendationAvailable {
		return r.PowerKW == 0 && r.ChargerCount == 0 && textValid
	}
	return (r.PowerKW == 120 || r.PowerKW == 180 || r.PowerKW == 240) && r.ChargerCount >= 1 && r.ChargerCount <= 100 && textValid
}
