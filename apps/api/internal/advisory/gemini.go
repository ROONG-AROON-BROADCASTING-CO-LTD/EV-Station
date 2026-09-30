package advisory

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"

	"github.com/rbc/ev-station/apps/api/internal/domain"
)

var (
	ErrNotConfigured = errors.New("Gemini advisory is not configured")
	ErrUnavailable   = errors.New("Gemini advisory is unavailable")
	ErrInvalidOutput = errors.New("Gemini advisory returned an invalid response")
)

type GeminiConfig struct {
	APIKey  string
	Model   string
	BaseURL string
	Timeout time.Duration
}

type Service struct {
	config GeminiConfig
	client *http.Client
}

type geminiRequest struct {
	Contents []struct {
		Role  string `json:"role"`
		Parts []struct {
			Text string `json:"text"`
		} `json:"parts"`
	} `json:"contents"`
	GenerationConfig struct {
		Temperature        float64        `json:"temperature"`
		MaxOutputTokens    int            `json:"maxOutputTokens"`
		ResponseMimeType   string         `json:"responseMimeType"`
		ResponseJSONSchema map[string]any `json:"responseJsonSchema"`
	} `json:"generationConfig"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

type generatedAssessment struct {
	Summary        string   `json:"summary"`
	Decision       string   `json:"decision"`
	Recommendation string   `json:"recommendation"`
	Strengths      []string `json:"strengths"`
	Risks          []string `json:"risks"`
	RequiredChecks []string `json:"requiredChecks"`
	Disclaimer     string   `json:"disclaimer"`
}

type generatedScoring struct {
	MetricScores   []generatedMetricScore `json:"metricScores"`
	Recommendation string                 `json:"recommendation"`
	Disclaimer     string                 `json:"disclaimer"`
}

type generatedMetricScore struct {
	MetricType string  `json:"metricType"`
	Score      float64 `json:"score"`
}

type generatedSiteSurface struct {
	Summary                 string                  `json:"summary"`
	Suitability             string                  `json:"suitability"`
	Score                   float64                 `json:"score"`
	SurfaceTypes            []string                `json:"surfaceTypes"`
	ObservedRisks           []string                `json:"observedRisks"`
	RecommendedImprovements []string                `json:"recommendedImprovements"`
	Disclaimer              string                  `json:"disclaimer"`
	EntranceWidthEstimate   *generatedEntranceWidth `json:"entranceWidthEstimate,omitempty"`
}

type generatedEntranceWidth struct {
	MinimumMeters  float64  `json:"minimumMeters"`
	MaximumMeters  float64  `json:"maximumMeters"`
	Confidence     string   `json:"confidence"`
	VisualEvidence string   `json:"visualEvidence"`
	Obstructions   []string `json:"obstructions"`
}

// FallbackAssessment keeps the analysis report useful when Gemini is
// temporarily unavailable or does not return the requested structured output.
// It deliberately uses only the completed run and never introduces new site
// facts. The model field makes the fallback transparent to staff.
func FallbackAssessment(run domain.AnalysisRun, language string) domain.AIAssessment {
	if language != "en" {
		language = "th"
	}
	decision := domain.InvestmentDecisionForScore(run.OverallScore)
	if language == "en" {
		recommendation := "The screening result supports moving to staff verification before any investment decision."
		if decision == domain.InvestmentDecisionNotRecommended {
			recommendation = "The screening result does not yet support proceeding. Complete the outstanding verification before reconsidering the site."
		}
		return domain.AIAssessment{
			Summary: "A system-generated summary is shown because the AI narrative could not be produced at this time.", Decision: decision, Recommendation: recommendation,
			Strengths:      []string{"The completed screening metrics remain available in this report."},
			Risks:          []string{"Some evidence may still be preliminary, estimated, or require field verification."},
			RequiredChecks: []string{"Review the evidence with a staff member.", "Confirm electrical capacity and the actual site conditions before installation."},
			Disclaimer:     "This is a system fallback summary, not an AI-generated assessment. Human review is required.", Language: language, Model: "system-fallback", GeneratedAt: time.Now().UTC(), DecisionPolicy: domain.InvestmentDecisionPolicyVersion,
		}
	}
	recommendation := "ผลคัดกรองสนับสนุนให้ดำเนินการตรวจสอบโดยเจ้าหน้าที่ก่อนตัดสินใจลงทุน"
	if decision == domain.InvestmentDecisionNotRecommended {
		recommendation = "ผลคัดกรองยังไม่สนับสนุนให้ดำเนินการ ควรตรวจสอบข้อมูลที่ค้างอยู่ให้ครบก่อนพิจารณาอีกครั้ง"
	}
	return domain.AIAssessment{
		Summary:  "แสดงสรุปจากระบบแทนชั่วคราว เนื่องจากบริการ AI ยังไม่สามารถสร้างคำอธิบายสำหรับรายงานนี้ได้",
		Decision: decision, Recommendation: recommendation,
		Strengths:      []string{"ผลการคัดกรองและตัวชี้วัดที่เสร็จสิ้นยังแสดงอยู่ในรายงานนี้"},
		Risks:          []string{"ข้อมูลบางส่วนอาจเป็นข้อมูลเบื้องต้น ข้อมูลประมาณการ หรือต้องยืนยันหน้างาน"},
		RequiredChecks: []string{"ให้เจ้าหน้าที่ตรวจสอบหลักฐานในรายงาน", "ยืนยันกำลังไฟฟ้าและสภาพพื้นที่จริงก่อนติดตั้ง"},
		Disclaimer:     "นี่คือสรุปสำรองจากระบบ ไม่ใช่ข้อความที่สร้างโดย AI และต้องตรวจสอบโดยเจ้าหน้าที่", Language: language, Model: "system-fallback", GeneratedAt: time.Now().UTC(), DecisionPolicy: domain.InvestmentDecisionPolicyVersion,
	}
}

func NewGeminiService(config GeminiConfig, client *http.Client) *Service {
	if config.Model == "" {
		config.Model = "gemini-3.5-flash-lite"
	}
	if config.BaseURL == "" {
		config.BaseURL = "https://generativelanguage.googleapis.com/v1beta"
	}
	if config.Timeout <= 0 {
		config.Timeout = 20 * time.Second
	}
	if client == nil {
		client = &http.Client{Timeout: config.Timeout}
	}
	return &Service{config: config, client: client}
}

func (s *Service) Generate(ctx context.Context, run domain.AnalysisRun, language string) (domain.AIAssessment, error) {
	if strings.TrimSpace(s.config.APIKey) == "" {
		return domain.AIAssessment{}, ErrNotConfigured
	}
	if language != "th" && language != "en" {
		language = "th"
	}
	payload, err := s.buildRequest(run, language)
	if err != nil {
		return domain.AIAssessment{}, err
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return domain.AIAssessment{}, err
	}
	endpoint := strings.TrimRight(s.config.BaseURL, "/") + "/models/" + url.PathEscape(s.config.Model) + ":generateContent"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return domain.AIAssessment{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", s.config.APIKey)
	response, err := s.client.Do(req)
	if err != nil {
		return domain.AIAssessment{}, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	defer response.Body.Close()
	responseBody, readErr := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if readErr != nil {
		return domain.AIAssessment{}, fmt.Errorf("%w: %v", ErrUnavailable, readErr)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return domain.AIAssessment{}, fmt.Errorf("%w: Gemini returned HTTP %d", ErrUnavailable, response.StatusCode)
	}
	var generated geminiResponse
	if err = json.Unmarshal(responseBody, &generated); err != nil || len(generated.Candidates) == 0 || len(generated.Candidates[0].Content.Parts) == 0 {
		return domain.AIAssessment{}, ErrInvalidOutput
	}
	var result generatedAssessment
	if err = json.Unmarshal([]byte(generated.Candidates[0].Content.Parts[0].Text), &result); err != nil || !validResult(result) || !assessmentMatchesLanguage(result, language) {
		return domain.AIAssessment{}, ErrInvalidOutput
	}
	return domain.AIAssessment{Summary: result.Summary, Decision: domain.InvestmentDecisionForScore(run.OverallScore), Recommendation: result.Recommendation, Strengths: result.Strengths, Risks: result.Risks, RequiredChecks: result.RequiredChecks, Disclaimer: result.Disclaimer, Language: language, Model: s.config.Model, GeneratedAt: time.Now().UTC(), DecisionPolicy: domain.InvestmentDecisionPolicyVersion}, nil
}

// Score asks Gemini to assess only the evidence that the backend has already
// collected. It does not send a precise location, customer details or place
// lists, and its proposed scores are validated again by the analysis service.
func (s *Service) Score(ctx context.Context, run domain.AnalysisRun, language string) (domain.AIScoring, error) {
	if strings.TrimSpace(s.config.APIKey) == "" {
		return domain.AIScoring{}, ErrNotConfigured
	}
	if language != "th" && language != "en" {
		language = "th"
	}
	payload, err := s.buildScoringRequest(run, language)
	if err != nil {
		return domain.AIScoring{}, err
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return domain.AIScoring{}, err
	}
	endpoint := strings.TrimRight(s.config.BaseURL, "/") + "/models/" + url.PathEscape(s.config.Model) + ":generateContent"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return domain.AIScoring{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", s.config.APIKey)
	response, err := s.client.Do(req)
	if err != nil {
		return domain.AIScoring{}, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	defer response.Body.Close()
	responseBody, readErr := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if readErr != nil {
		return domain.AIScoring{}, fmt.Errorf("%w: %v", ErrUnavailable, readErr)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return domain.AIScoring{}, fmt.Errorf("%w: Gemini returned HTTP %d", ErrUnavailable, response.StatusCode)
	}
	var generated geminiResponse
	if err = json.Unmarshal(responseBody, &generated); err != nil || len(generated.Candidates) == 0 || len(generated.Candidates[0].Content.Parts) == 0 {
		return domain.AIScoring{}, ErrInvalidOutput
	}
	var result generatedScoring
	if err = json.Unmarshal([]byte(generated.Candidates[0].Content.Parts[0].Text), &result); err != nil || !validScoringResult(result) {
		return domain.AIScoring{}, ErrInvalidOutput
	}
	metricScores := make(map[string]float64, len(result.MetricScores))
	for _, item := range result.MetricScores {
		metricScores[item.MetricType] = item.Score
	}
	return domain.AIScoring{MetricScores: metricScores, Recommendation: result.Recommendation, Disclaimer: result.Disclaimer, Language: language, Model: s.config.Model, GeneratedAt: time.Now().UTC()}, nil
}

// AnalyzeSiteSurface assesses preliminary visible site conditions from
// customer-supplied images, map captures, and PDF documents. The returned
// score is intentionally separate from the location score.
func (s *Service) AnalyzeSiteSurface(ctx context.Context, images []domain.SiteImage, language string) (domain.SiteSurfaceAssessment, error) {
	if strings.TrimSpace(s.config.APIKey) == "" {
		return domain.SiteSurfaceAssessment{}, ErrNotConfigured
	}
	if len(images) == 0 || len(images) > 10 {
		return domain.SiteSurfaceAssessment{}, ErrInvalidOutput
	}
	if language != "th" && language != "en" {
		language = "th"
	}
	languageName := map[string]string{"th": "Thai", "en": "English"}[language]
	prompt := "You are an EV-charging-site field-condition analyst. Write in " + languageName + ". Analyze the supplied customer evidence together: site photos, map captures, and PDF documents such as a title deed or parcel plan. Assess visible ground surface materials, evenness, visible drainage/water-ponding indicators, and likely construction preparation. A PDF title deed or parcel plan may support parcel shape, printed dimensions, and context only when those details are legible; do not repeat owner names, national IDs, deed numbers, or any personal information. Never claim that a document verifies a legal boundary, current road access, or ownership. Also, ONLY when a clear entrance boundary and a reasonably reliable visual scale/reference are visible across the evidence, provide entranceWidthEstimate as an approximate meter range. Never provide a single exact width; use confidence low, moderate, or high; identify visible obstructions. Omit entranceWidthEstimate when the entrance, boundaries, perspective, document scale, or map alignment are insufficient. A vehicle, a photo, or a parcel diagram alone is not confirmed scale. This estimate is not a survey, does not prove two-way access, and must never be used as engineering approval. Do NOT assess or mention electricity, traffic, population, competitors, flood maps, ROI, or the overall location score. Give a preliminary ground-surface suitability score from 0 to 100 based only on what is visible. Do not claim soil bearing capacity, underground conditions, or a final engineering approval. If evidence is not visible, state that limitation. Return concise JSON matching the schema."
	parts := make([]map[string]any, 0, len(images)+1)
	parts = append(parts, map[string]any{"text": prompt})
	for _, image := range images {
		parts = append(parts, map[string]any{"inline_data": map[string]string{"mime_type": image.MIMEType, "data": base64.StdEncoding.EncodeToString(image.Data)}})
	}
	payload := map[string]any{
		"contents":         []map[string]any{{"role": "user", "parts": parts}},
		"generationConfig": map[string]any{"temperature": 0, "maxOutputTokens": 900, "responseMimeType": "application/json", "responseJsonSchema": siteSurfaceSchema()},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return domain.SiteSurfaceAssessment{}, err
	}
	endpoint := strings.TrimRight(s.config.BaseURL, "/") + "/models/" + url.PathEscape(s.config.Model) + ":generateContent"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return domain.SiteSurfaceAssessment{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", s.config.APIKey)
	response, err := s.client.Do(req)
	if err != nil {
		return domain.SiteSurfaceAssessment{}, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	defer response.Body.Close()
	responseBody, readErr := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if readErr != nil {
		return domain.SiteSurfaceAssessment{}, fmt.Errorf("%w: %v", ErrUnavailable, readErr)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return domain.SiteSurfaceAssessment{}, fmt.Errorf("%w: Gemini returned HTTP %d", ErrUnavailable, response.StatusCode)
	}
	var generated geminiResponse
	if err = json.Unmarshal(responseBody, &generated); err != nil || len(generated.Candidates) == 0 || len(generated.Candidates[0].Content.Parts) == 0 {
		return domain.SiteSurfaceAssessment{}, ErrInvalidOutput
	}
	var result generatedSiteSurface
	if err = json.Unmarshal([]byte(generated.Candidates[0].Content.Parts[0].Text), &result); err != nil || !validSiteSurface(result) {
		return domain.SiteSurfaceAssessment{}, ErrInvalidOutput
	}
	assessment := domain.SiteSurfaceAssessment{Summary: result.Summary, Suitability: result.Suitability, Score: result.Score, SurfaceTypes: result.SurfaceTypes, ObservedRisks: result.ObservedRisks, RecommendedImprovements: result.RecommendedImprovements, Disclaimer: result.Disclaimer, Model: s.config.Model}
	if validEntranceWidthEstimate(result.EntranceWidthEstimate) {
		assessment.EntranceWidthEstimate = &domain.EntranceWidthEstimate{MinimumMeters: result.EntranceWidthEstimate.MinimumMeters, MaximumMeters: result.EntranceWidthEstimate.MaximumMeters, Confidence: result.EntranceWidthEstimate.Confidence, VisualEvidence: result.EntranceWidthEstimate.VisualEvidence, Obstructions: result.EntranceWidthEstimate.Obstructions}
	}
	return assessment, nil
}

func (s *Service) buildRequest(run domain.AnalysisRun, language string) (geminiRequest, error) {
	facts, err := compactRunFacts(run)
	if err != nil {
		return geminiRequest{}, err
	}
	facts["systemDecision"] = domain.InvestmentDecisionForScore(run.OverallScore)
	factJSON, err := json.Marshal(facts)
	if err != nil {
		return geminiRequest{}, err
	}
	prompt := assessmentPrompt(language, string(factJSON))
	var request geminiRequest
	request.Contents = append(request.Contents, struct {
		Role  string `json:"role"`
		Parts []struct {
			Text string `json:"text"`
		} `json:"parts"`
	}{Role: "user", Parts: []struct {
		Text string `json:"text"`
	}{{Text: prompt}}})
	request.GenerationConfig.Temperature = 0
	request.GenerationConfig.MaxOutputTokens = 700
	request.GenerationConfig.ResponseMimeType = "application/json"
	request.GenerationConfig.ResponseJSONSchema = assessmentSchema()
	return request, nil
}

func (s *Service) buildScoringRequest(run domain.AnalysisRun, language string) (geminiRequest, error) {
	facts, err := compactRunFacts(run)
	if err != nil {
		return geminiRequest{}, err
	}
	eligible := make([]string, 0, len(run.Metrics))
	for _, metric := range run.Metrics {
		if metric.NormalizedScore != nil {
			eligible = append(eligible, metric.Type)
		}
	}
	facts["eligibleMetricTypes"] = eligible
	factJSON, err := json.Marshal(facts)
	if err != nil {
		return geminiRequest{}, err
	}
	languageName := map[string]string{"th": "Thai", "en": "English"}[language]
	prompt := "You are an evidence-bound EV-station screening scorer. Write in " + languageName + ". Use ONLY the supplied facts and score ONLY eligibleMetricTypes. For each eligible metric, return one metricScores array item with its metricType and a score from 0 to 100 based on observed values, source status and limitations. Do not include missing data or electrical capacity. Do not invent location data, traffic, demand, grid capacity, ROI, payback, competitors or sources. The backend will calculate the overall weighted total; do not provide an overall score. Return concise JSON matching the schema. The disclaimer must state that the result is preliminary and requires human review.\n\nFACTS:\n" + string(factJSON)
	return scoringRequest(prompt), nil
}

func scoringRequest(prompt string) geminiRequest {
	var request geminiRequest
	request.Contents = append(request.Contents, struct {
		Role  string `json:"role"`
		Parts []struct {
			Text string `json:"text"`
		} `json:"parts"`
	}{Role: "user", Parts: []struct {
		Text string `json:"text"`
	}{{Text: prompt}}})
	request.GenerationConfig.Temperature = 0
	request.GenerationConfig.MaxOutputTokens = 700
	request.GenerationConfig.ResponseMimeType = "application/json"
	request.GenerationConfig.ResponseJSONSchema = scoringSchema()
	return request
}

func compactRunFacts(run domain.AnalysisRun) (map[string]any, error) {
	metrics := make([]map[string]any, 0, len(run.Metrics))
	for _, metric := range run.Metrics {
		item := map[string]any{"type": metric.Type, "status": metric.Status, "source": metric.Source.Name, "methodology": metric.Source.Methodology, "assumptions": metric.Assumptions}
		var raw map[string]any
		if len(metric.RawValue) > 0 && json.Unmarshal(metric.RawValue, &raw) == nil {
			delete(raw, "places")
			delete(raw, "sources")
			delete(raw, "latitude")
			delete(raw, "longitude")
			item["observedValues"] = raw
		}
		metrics = append(metrics, item)
	}
	facts := map[string]any{"assessmentStatus": run.AssessmentStatus, "analysisRadiusMeters": run.AnalysisRadiusMeters, "overallScore": run.OverallScore, "metrics": metrics, "financialAvailable": run.Financial != nil}
	if run.Financial != nil {
		facts["financialEstimate"] = map[string]any{
			"initialInvestment": run.Financial.InitialInvestment, "monthlyRevenue": run.Financial.MonthlyRevenue,
			"monthlyOperatingCost": run.Financial.MonthlyOperatingCost, "monthlyProfit": run.Financial.MonthlyProfit,
			"annualProfit": run.Financial.AnnualProfit, "roiPercentage": run.Financial.ROIPercentage,
			"paybackMonths": run.Financial.PaybackMonths, "assumptions": run.Financial.Assumptions,
		}
	}
	return facts, nil
}

func assessmentSchema() map[string]any {
	stringArray := map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "maxItems": 5}
	return map[string]any{"type": "object", "properties": map[string]any{"summary": map[string]any{"type": "string"}, "decision": map[string]any{"type": "string", "enum": []string{"invest", "not_recommended"}}, "recommendation": map[string]any{"type": "string"}, "strengths": stringArray, "risks": stringArray, "requiredChecks": stringArray, "disclaimer": map[string]any{"type": "string"}}, "required": []string{"summary", "decision", "recommendation", "strengths", "risks", "requiredChecks", "disclaimer"}, "additionalProperties": false}
}

func scoringSchema() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{
		"metricScores":   map[string]any{"type": "array", "minItems": 1, "maxItems": 7, "items": map[string]any{"type": "object", "properties": map[string]any{"metricType": map[string]any{"type": "string", "enum": []string{"traffic", "road_accessibility", "ev_demand", "population", "poi", "competition", "flood"}}, "score": map[string]any{"type": "number", "minimum": 0, "maximum": 100}}, "required": []string{"metricType", "score"}, "additionalProperties": false}},
		"recommendation": map[string]any{"type": "string"},
		"disclaimer":     map[string]any{"type": "string"},
	}, "required": []string{"metricScores", "recommendation", "disclaimer"}, "additionalProperties": false}
}

func siteSurfaceSchema() map[string]any {
	stringArray := map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "maxItems": 8}
	entranceWidthEstimate := map[string]any{"type": "object", "properties": map[string]any{
		"minimumMeters":  map[string]any{"type": "number", "minimum": 0.5, "maximum": 100},
		"maximumMeters":  map[string]any{"type": "number", "minimum": 0.5, "maximum": 100},
		"confidence":     map[string]any{"type": "string", "enum": []string{"low", "moderate", "high"}},
		"visualEvidence": map[string]any{"type": "string"},
		"obstructions":   stringArray,
	}, "required": []string{"minimumMeters", "maximumMeters", "confidence", "visualEvidence", "obstructions"}, "additionalProperties": false}
	return map[string]any{"type": "object", "properties": map[string]any{
		"summary":      map[string]any{"type": "string"},
		"suitability":  map[string]any{"type": "string", "enum": []string{"low", "moderate", "high"}},
		"score":        map[string]any{"type": "number", "minimum": 0, "maximum": 100},
		"surfaceTypes": stringArray, "observedRisks": stringArray, "recommendedImprovements": stringArray,
		"disclaimer":            map[string]any{"type": "string"},
		"entranceWidthEstimate": entranceWidthEstimate,
	}, "required": []string{"summary", "suitability", "score", "surfaceTypes", "observedRisks", "recommendedImprovements", "disclaimer"}, "additionalProperties": false}
}

func validResult(value generatedAssessment) bool {
	return strings.TrimSpace(value.Summary) != "" && (value.Decision == "invest" || value.Decision == "not_recommended") && strings.TrimSpace(value.Recommendation) != "" && strings.TrimSpace(value.Disclaimer) != ""
}

func assessmentPrompt(language, facts string) string {
	if language == "th" {
		return "คุณเป็นนักวิเคราะห์คัดกรองทำเลสถานีชาร์จ EV ที่ยึดหลักฐานเป็นหลัก\n" +
			"ข้อกำหนดด้านภาษา: ตอบเป็นภาษาไทยเท่านั้น ทุกข้อความที่แสดงต่อผู้ใช้ในฟิลด์ summary, recommendation, strengths, risks, requiredChecks และ disclaimer ต้องเป็นภาษาไทย ห้ามเขียนประโยคภาษาอังกฤษ ยกเว้นชื่อเฉพาะ คำย่อ รหัสถนน หน่วยวัด หรือชื่อแหล่งข้อมูลที่จำเป็น\n" +
			"ใน risks และ requiredChecks ให้พิจารณาการเดินทางเข้าถึงพื้นที่ช่วงน้ำท่วมด้วย แม้คะแนนทำเลรวมสูง หากมีรายงานน้ำท่วมระดับอำเภอให้ระบุระดับหลักฐานนั้น และให้ตรวจประวัติถนนหน้าแปลงและเส้นทางเข้า–ออก ห้ามอ้างว่าถนนเส้นใดเคยท่วมจากข้อมูลระดับอำเภอเพียงอย่างเดียว หากไม่มีข้อมูลระดับถนนให้ระบุว่ายังไม่ยืนยัน\n" +
			"ใช้เฉพาะข้อเท็จจริงที่ให้มาเท่านั้น ห้ามสร้างข้อมูลทำเล ความต้องการใช้ EV ปริมาณจราจร กำลังไฟฟ้า ROI ระยะคืนทุน คะแนน คู่แข่ง หรือการอ้างอิงแหล่งข้อมูลขึ้นเอง ห้ามคำนวณ เปลี่ยนแปลง หรือเสนอคะแนนตัวเลขหรือ ROI\n" +
			"ค่า systemDecision ในข้อเท็จจริงเป็นผลตัดสินที่ระบบคำนวณจากคะแนนคัดกรองแล้ว ต้องส่งค่า decision ให้ตรงกับ systemDecision ทุกครั้ง ห้ามเปลี่ยนสถานะเอง; recommendation อธิบายหลักฐานที่สนับสนุนสถานะนี้เท่านั้น ห้ามวิเคราะห์ผลกระทบต่อการลงทุนหรือการคืนทุน\n" +
			"ให้พิจารณา metric ชนิด electrical เป็นพิเศษ: หาก observedValues มีระยะ nearestHighVoltageLineMeters หรือ nearestStationMeters ให้ระบุตัวเลขระยะนั้นตามข้อมูลจริง พร้อมชื่อสถานี/ระดับแรงดันเมื่อมี; หากไม่มีหลักฐาน ให้ระบุขอบเขตรัศมีค้นหาตาม searchRadiusMeters และบอกเพียงว่าไม่พบในชั้นข้อมูลสาธารณะ ห้ามสรุปว่าไม่มีไฟฟ้า, ต้องลากสาย, หรือต้องติดตั้งหม้อแปลงแน่นอน\n" +
			"เมื่อความพร้อมไฟฟ้ายังไม่ยืนยัน ให้ระบุว่าค่าใช้จ่ายที่ต้องขอประเมินจากการไฟฟ้าอาจครอบคลุมค่าคำขอและสำรวจ, ค่ามิเตอร์หรือค่าธรรมเนียมเชื่อมต่อ, งานขยายเขตหรือสายไฟ, หม้อแปลงและตู้สวิตช์/ระบบป้องกัน, งานโยธาแนวร้อยสาย และงานติดตั้งตู้ชาร์จ/ใบอนุญาต โดยห้ามใส่ราคา และต้องชี้ชัดว่าแต่ละรายการจะเกิดขึ้นหรือไม่ขึ้นกับผลสำรวจจริง\n" +
			"ห้ามวิเคราะห์หรือสรุป ROI ระยะคืนทุน หรือผลตอบแทน แม้มี financialEstimate อยู่ในข้อมูล ใน risks ให้เขียนความเสี่ยงจากข้อเท็จจริง และใน requiredChecks ให้เขียนรายการขอข้อมูล/ดำเนินการที่ทำได้จริง อธิบายว่าเป็นผลเบื้องต้นเมื่อข้อมูลเป็นข้อมูลประมาณการ ข้อมูลเบื้องต้น หรือไม่มีข้อมูล ตอบ JSON แบบกระชับให้ตรงตาม schema เท่านั้น\n\nข้อเท็จจริง:\n" + facts
	}
	return "You are an evidence-bound EV-station analyst. Include flood-related access risks and checks even when the overall location score is strong. District history does not confirm floods on a specific frontage road. Identify road-level flood history as unverified when that evidence is absent, and require checking frontage and approach routes. Write every user-facing value in English. Use ONLY the supplied facts. The systemDecision field is already determined by the screening-score rule: return that exact value in decision and do not change the status yourself. Recommendation must explain only the evidence supporting that system decision. Do not invent location data, demand, traffic, grid capacity, ROI, payback, scores, competitors, or source claims. Do not calculate, alter, or propose numeric scores or ROI. Explain that the result is preliminary where evidence is estimated, preliminary, or missing. Return concise JSON matching the schema.\n\nFACTS:\n" + facts
}

func assessmentMatchesLanguage(value generatedAssessment, language string) bool {
	if language != "th" {
		return true
	}
	texts := []string{value.Summary, value.Recommendation, value.Disclaimer}
	texts = append(texts, value.Strengths...)
	texts = append(texts, value.Risks...)
	texts = append(texts, value.RequiredChecks...)
	for _, text := range texts {
		if strings.TrimSpace(text) != "" && !containsThai(text) {
			return false
		}
	}
	return true
}

func containsThai(text string) bool {
	for _, character := range text {
		if unicode.Is(unicode.Thai, character) {
			return true
		}
	}
	return false
}

func validScoringResult(value generatedScoring) bool {
	if len(value.MetricScores) == 0 || strings.TrimSpace(value.Recommendation) == "" || strings.TrimSpace(value.Disclaimer) == "" {
		return false
	}
	seen := make(map[string]bool, len(value.MetricScores))
	for _, item := range value.MetricScores {
		if item.MetricType == "" || seen[item.MetricType] || item.Score < 0 || item.Score > 100 {
			return false
		}
		seen[item.MetricType] = true
	}
	return true
}

func validSiteSurface(value generatedSiteSurface) bool {
	return strings.TrimSpace(value.Summary) != "" && (value.Suitability == "low" || value.Suitability == "moderate" || value.Suitability == "high") && value.Score >= 0 && value.Score <= 100 && strings.TrimSpace(value.Disclaimer) != ""
}

func validEntranceWidthEstimate(value *generatedEntranceWidth) bool {
	return value != nil && value.MinimumMeters > 0 && value.MaximumMeters >= value.MinimumMeters && value.MaximumMeters <= 100 && (value.Confidence == "low" || value.Confidence == "moderate" || value.Confidence == "high") && strings.TrimSpace(value.VisualEvidence) != ""
}
