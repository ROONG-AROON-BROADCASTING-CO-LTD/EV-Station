package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rbc/ev-station/apps/api/internal/advisory"
	"github.com/rbc/ev-station/apps/api/internal/analysis"
	"github.com/rbc/ev-station/apps/api/internal/domain"
	"github.com/rbc/ev-station/apps/api/internal/repository"
)

func TestCachedFallbackCanRecoverWithoutForcingRefresh(t *testing.T) {
	for _, tc := range []struct {
		name      string
		model     string
		age       time.Duration
		role      domain.UserRole
		readOnly  bool
		wantCalls int32
	}{
		{"staff recovers stale fallback", "system-fallback", 2 * time.Minute, domain.RoleAdmin, false, 1},
		{"staff preserves recent fallback cooldown", "system-fallback", 10 * time.Second, domain.RoleAdmin, false, 0},
		{"staff keeps successful AI cache", "test-model", 2 * time.Minute, domain.RoleAdmin, false, 0},
		{"customer reads fallback without provider cost", "system-fallback", 2 * time.Minute, domain.RoleCustomer, false, 0},
		{"viewer reads fallback without provider cost", "system-fallback", 2 * time.Minute, domain.RoleViewer, false, 0},
		{"sales with write grant recovers fallback", "system-fallback", 2 * time.Minute, domain.RoleSales, false, 1},
		{"sales with read-only grant keeps cached read", "system-fallback", 2 * time.Minute, domain.RoleSales, true, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{"candidates": []any{map[string]any{"content": map[string]any{"parts": []any{map[string]any{"text": `{"summary":"AI recovered.","decision":"not_recommended","recommendation":"Review the site.","strengths":[],"risks":[],"requiredChecks":[],"disclaimer":"Human review required."}`}}}}}})
			}))
			defer server.Close()
			handler, repo, run, userID := advisoryCacheFixture(t, server, tc.model, tc.age, tc.role)
			if tc.readOnly {
				// A legacy viewer grant is read-only; RoleViewer itself now aliases customer.
				if err := repo.SetSiteAccess(context.Background(), domain.SiteAccess{SiteID: run.SiteID, UserID: userID, Role: "viewer"}); err != nil {
					t.Fatal(err)
				}
			}
			result := invokeCachedAssessment(t, handler, run.ID, userID, tc.role)
			if calls.Load() != tc.wantCalls {
				t.Fatalf("provider calls=%d want=%d", calls.Load(), tc.wantCalls)
			}
			if tc.wantCalls == 1 {
				if result.Model != "test-model" {
					t.Fatalf("stale fallback was not replaced: %+v", result)
				}
				stored, _ := repo.GetAnalysis(context.Background(), run.ID)
				var cached map[string]domain.AIAssessment
				if err := json.Unmarshal(stored.AIAssessments, &cached); err != nil || cached["en"].Model != "test-model" {
					t.Fatalf("recovered AI result not persisted: %s", stored.AIAssessments)
				}
				invokeCachedAssessment(t, handler, run.ID, userID, tc.role)
				if calls.Load() != 1 {
					t.Fatal("a successful recovery must be reused on another view")
				}
			}
		})
	}
}

func TestFailedFallbackRecoveryRenewsCooldown(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()
	handler, _, run, userID := advisoryCacheFixture(t, server, "system-fallback", 2*time.Minute, domain.RoleAdmin)
	start := time.Now().UTC()
	result := invokeCachedAssessment(t, handler, run.ID, userID, domain.RoleAdmin)
	if calls.Load() != 3 || result.Model != "system-fallback" || result.GeneratedAt.Before(start) {
		t.Fatalf("failed recovery must be bounded and renew the fallback timestamp: calls=%d result=%+v", calls.Load(), result)
	}
	invokeCachedAssessment(t, handler, run.ID, userID, domain.RoleAdmin)
	if calls.Load() != 3 {
		t.Fatal("an immediate subsequent view must not start another retry cycle")
	}
}

func advisoryCacheFixture(t *testing.T, server *httptest.Server, model string, age time.Duration, role domain.UserRole) (*Handler, *repository.Memory, domain.AnalysisRun, uuid.UUID) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	repo := repository.NewMemory()
	site := domain.Site{ID: uuid.New(), Name: "Synthetic retry QA"}
	if _, err := repo.CreateSite(context.Background(), site); err != nil {
		t.Fatal(err)
	}
	userID := uuid.New()
	if err := repo.SetSiteAccess(context.Background(), domain.SiteAccess{SiteID: site.ID, UserID: userID, Role: string(role)}); err != nil {
		t.Fatal(err)
	}
	run := domain.AnalysisRun{ID: uuid.New(), SiteID: site.ID, Status: "completed"}
	fallback := advisory.FallbackAssessment(run, "en")
	fallback.Model = model
	fallback.GeneratedAt = time.Now().UTC().Add(-age)
	run.AIAssessments, _ = json.Marshal(map[string]domain.AIAssessment{"en": fallback})
	if err := repo.CompleteAnalysis(context.Background(), run); err != nil {
		t.Fatal(err)
	}
	gemini := advisory.NewGeminiService(advisory.GeminiConfig{APIKey: "test-key", Model: "test-model", BaseURL: server.URL, Timeout: 5 * time.Second}, server.Client())
	return &Handler{repo: repo, analyses: analysis.NewService(repo, nil, nil), advisory: gemini}, repo, run, userID
}

func invokeCachedAssessment(t *testing.T, handler *Handler, runID, userID uuid.UUID, role domain.UserRole) domain.AIAssessment {
	t.Helper()
	response := httptest.NewRecorder()
	requestContext, _ := gin.CreateTestContext(response)
	requestContext.Request = httptest.NewRequest(http.MethodPost, "/analyses/"+runID.String()+"/ai-assessment", strings.NewReader(`{"language":"en"}`))
	requestContext.Request.Header.Set("Content-Type", "application/json")
	requestContext.Params = gin.Params{{Key: "id", Value: runID.String()}}
	requestContext.Set("userId", userID.String())
	requestContext.Set("role", role)
	handler.GenerateAIAssessment(requestContext)
	if response.Code != http.StatusOK {
		t.Fatalf("expected readable assessment, got status=%d body=%s", response.Code, response.Body.String())
	}
	var result struct {
		Data domain.AIAssessment `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	return result.Data
}
