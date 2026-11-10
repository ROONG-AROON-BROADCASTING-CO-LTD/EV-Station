package advisory

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rbc/ev-station/apps/api/internal/domain"
)

func TestGeminiServiceDoesNotSendCoordinatesOrPlaceLists(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("x-goog-api-key") != "test-key" {
			t.Fatalf("expected server-side API key header")
		}
		body, _ := io.ReadAll(request.Body)
		text := string(body)
		if strings.Contains(text, "secret-place") || strings.Contains(text, "13.7563") {
			t.Fatalf("sensitive raw fields must not be sent to Gemini: %s", text)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"{\"summary\":\"สรุปจากข้อมูลที่มี\",\"decision\":\"not_recommended\",\"recommendation\":\"ตรวจสอบระบบไฟฟ้าเพิ่มเติม\",\"strengths\":[\"มีข้อมูลจราจร\"],\"risks\":[\"ข้อมูลไฟฟ้าเป็นเบื้องต้น\"],\"requiredChecks\":[\"ยืนยันกับการไฟฟ้า\"],\"disclaimer\":\"ไม่ใช่คะแนนหรือผลตอบแทน\"}"}]}}]}`))
	}))
	defer server.Close()
	service := NewGeminiService(GeminiConfig{APIKey: "test-key", Model: "test-model", BaseURL: server.URL, Timeout: time.Second}, server.Client())
	result, err := service.Generate(context.Background(), domain.AnalysisRun{AssessmentStatus: domain.DataPreliminary, AnalysisRadiusMeters: 3000, Metrics: []domain.Metric{{Type: "competition", Status: domain.DataVerified, Source: domain.DataSource{Name: "OSM"}, RawValue: []byte(`{"count":1,"latitude":13.7563,"places":[{"name":"secret-place"}]}`)}}}, "th")
	if err != nil {
		t.Fatal(err)
	}
	if result.Model != "test-model" || result.Language != "th" || result.Summary == "" {
		t.Fatalf("unexpected assessment: %+v", result)
	}
}

func TestGeminiServiceRequiresAPIKey(t *testing.T) {
	service := NewGeminiService(GeminiConfig{}, nil)
	_, err := service.Generate(context.Background(), domain.AnalysisRun{}, "th")
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("expected ErrNotConfigured, got %v", err)
	}
}

func TestGeminiNarrativeHasRoomForLongThaiResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var payload geminiRequest
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Error(err)
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		// The captured Thai QA run needed 775 output tokens to finish its JSON.
		if payload.GenerationConfig.MaxOutputTokens < 775 {
			_, _ = writer.Write([]byte(`{"candidates":[{"finishReason":"MAX_TOKENS","content":{"parts":[{"text":"{\"summary\":\"สรุป"}]}}]}`))
			return
		}
		complete := `{"summary":"สรุปหลักฐาน","decision":"invest","recommendation":"ตรวจหน้างาน","disclaimer":"ผลเบื้องต้น"}`
		_ = json.NewEncoder(writer).Encode(map[string]any{"candidates": []any{
			map[string]any{"content": map[string]any{"parts": []any{map[string]string{"text": complete}}}},
		}})
	}))
	defer server.Close()
	service := NewGeminiService(GeminiConfig{APIKey: "test-key", Model: "test-model", BaseURL: server.URL, Timeout: time.Second}, server.Client())
	result, err := service.Generate(context.Background(), domain.AnalysisRun{}, "th")
	if err != nil || result.Summary != "สรุปหลักฐาน" {
		t.Fatalf("expected a complete Thai narrative, got summary %q and error %v", result.Summary, err)
	}
}

func TestGeminiServiceKeepsSystemDecisionWhenModelDisagrees(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"{\"summary\":\"Evidence supports the site\",\"decision\":\"not_recommended\",\"recommendation\":\"Review utility capacity before installation.\",\"strengths\":[\"Traffic evidence is available\"],\"risks\":[\"Utility capacity is preliminary\"],\"requiredChecks\":[\"Confirm supply with the utility\"],\"disclaimer\":\"This is preliminary.\"}"}]}}]}`))
	}))
	defer server.Close()
	service := NewGeminiService(GeminiConfig{APIKey: "test-key", Model: "test-model", BaseURL: server.URL, Timeout: time.Second}, server.Client())
	score := 60.0
	result, err := service.Generate(context.Background(), domain.AnalysisRun{OverallScore: &score}, "en")
	if err != nil {
		t.Fatal(err)
	}
	if result.Decision != domain.InvestmentDecisionInvest {
		t.Fatalf("expected system decision %q, got %q", domain.InvestmentDecisionInvest, result.Decision)
	}
}

func TestAssessmentMatchesRequestedThaiLanguage(t *testing.T) {
	thai := generatedAssessment{
		Summary: "สรุปผลจากข้อมูล", Recommendation: "ตรวจสอบเพิ่มเติม", Disclaimer: "เป็นผลเบื้องต้น",
		Strengths: []string{"มีข้อมูลจราจร"}, Risks: []string{"ข้อมูลไฟฟ้ายังไม่ครบ"}, RequiredChecks: []string{"ยืนยันกับการไฟฟ้า"},
	}
	if !assessmentMatchesLanguage(thai, "th") {
		t.Fatal("expected Thai assessment to be accepted")
	}
	english := generatedAssessment{Summary: "English only", Recommendation: "Review the site", Disclaimer: "Preliminary result", Strengths: []string{"Traffic data"}}
	if assessmentMatchesLanguage(english, "th") {
		t.Fatal("expected English-only assessment to be rejected for Thai")
	}
	if !assessmentMatchesLanguage(english, "en") {
		t.Fatal("expected English assessment to be accepted for English")
	}
}

func TestGeminiServiceScoresOnlyEvidenceWithoutCoordinates(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		text := string(body)
		if strings.Contains(text, "13.7563") || strings.Contains(text, "secret-place") {
			t.Fatalf("sensitive raw fields must not be sent to Gemini: %s", text)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"{\"metricScores\":[{\"metricType\":\"traffic\",\"score\":71}],\"recommendation\":\"ตรวจสอบทางเข้าออกเพิ่มเติม\",\"disclaimer\":\"เป็นผลคัดกรองเบื้องต้น ต้องตรวจสอบโดยพนักงาน\"}"}]}}]}`))
	}))
	defer server.Close()
	service := NewGeminiService(GeminiConfig{APIKey: "test-key", Model: "test-model", BaseURL: server.URL, Timeout: time.Second}, server.Client())
	score := 65.0
	result, err := service.Score(context.Background(), domain.AnalysisRun{AssessmentStatus: domain.DataPreliminary, AnalysisRadiusMeters: 3000, Metrics: []domain.Metric{{Type: "traffic", Status: domain.DataVerified, NormalizedScore: &score, Source: domain.DataSource{Name: "DOH"}, RawValue: []byte(`{"aadt":50000,"latitude":13.7563,"places":[{"name":"secret-place"}]}`)}}}, "th")
	if err != nil {
		t.Fatal(err)
	}
	if result.MetricScores["traffic"] != 71 || result.Model != "test-model" {
		t.Fatalf("unexpected scoring result: %+v", result)
	}
}

func TestGeminiServiceAnalyzesSiteSurfaceEvidence(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		payload := string(body)
		if !strings.Contains(payload, `"inline_data"`) || !strings.Contains(payload, `"mime_type":"image/jpeg"`) || !strings.Contains(payload, `"mime_type":"application/pdf"`) {
			t.Fatalf("expected customer image and PDF as Gemini inline data: %s", payload)
		}
		if !strings.Contains(payload, "PDF documents such as a title deed or parcel plan") || !strings.Contains(payload, "entranceWidthEstimate") || !strings.Contains(payload, "must never be used as engineering approval") {
			t.Fatalf("expected a safeguarded multimodal prompt: %s", payload)
		}
		if !strings.Contains(payload, "Never use score 0 or suitability low to represent missing evidence") || !strings.Contains(payload, `"type":["number","null"]`) {
			t.Fatalf("expected explicit insufficient-evidence instructions and a nullable score: %s", payload)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"{\"assessmentStatus\":\"assessed\",\"summary\":\"พื้นกรวดและดินปนหญ้า\",\"suitability\":\"moderate\",\"score\":55,\"surfaceTypes\":[\"กรวด\",\"ดิน\"],\"observedRisks\":[\"พื้นไม่เรียบ\"],\"recommendedImprovements\":[\"ปรับระดับและบดอัด\"],\"disclaimer\":\"ประเมินจากสิ่งที่เห็นในภาพเท่านั้น\",\"entranceWidthEstimate\":{\"minimumMeters\":5,\"maximumMeters\":6.5,\"confidence\":\"moderate\",\"visualEvidence\":\"เห็นขอบประตูทั้งสองด้าน\",\"obstructions\":[\"ประตูกั้น\"]}}"}]}}]}`))
	}))
	defer server.Close()

	service := NewGeminiService(GeminiConfig{APIKey: "test-key", Model: "test-model", BaseURL: server.URL, Timeout: time.Second}, server.Client())
	result, err := service.AnalyzeSiteSurface(context.Background(), []domain.SiteImage{{MIMEType: "image/jpeg", Data: []byte("image-data")}, {MIMEType: "application/pdf", Data: []byte("pdf-data")}}, "th")
	if err != nil {
		t.Fatal(err)
	}
	if result.AssessmentStatus != "assessed" || result.Score == nil || *result.Score != 55 || result.Suitability != "moderate" || len(result.RecommendedImprovements) != 1 || result.EntranceWidthEstimate == nil || result.EntranceWidthEstimate.MinimumMeters != 5 || result.EntranceWidthEstimate.MaximumMeters != 6.5 {
		t.Fatalf("unexpected surface assessment: %+v", result)
	}
}
