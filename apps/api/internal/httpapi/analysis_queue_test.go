package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rbc/ev-station/apps/api/internal/analysis"
	"github.com/rbc/ev-station/apps/api/internal/domain"
	"github.com/rbc/ev-station/apps/api/internal/provider"
	"github.com/rbc/ev-station/apps/api/internal/repository"
	"github.com/rbc/ev-station/apps/api/internal/scoring"
)

func TestRunAnalysisAcceptsDurableJob(t *testing.T) {
	repo := repository.NewMemory()
	now := time.Now().UTC()
	site := domain.Site{ID: uuid.New(), Name: "Queued site", Address: "Bangkok", LandSize: 100, LandSizeUnit: "sqm", CreatedAt: now, UpdatedAt: now}
	if _, err := repo.CreateSite(context.Background(), site); err != nil {
		t.Fatal(err)
	}
	engine, _ := scoring.New(scoring.DefaultWeights)
	handler := &Handler{repo: repo, analyses: analysis.NewService(repo, provider.FixtureProvider{}, engine)}
	response := httptest.NewRecorder()
	requestContext, _ := gin.CreateTestContext(response)
	requestContext.Request = httptest.NewRequest(http.MethodPost, "/api/v1/sites/"+site.ID.String()+"/analyses", strings.NewReader(`{"radiusMeters":3000}`))
	requestContext.Request.Header.Set("Content-Type", "application/json")
	requestContext.Params = gin.Params{{Key: "id", Value: site.ID.String()}}
	requestContext.Set("userId", uuid.NewString())
	requestContext.Set("role", domain.RoleAdmin)
	handler.RunAnalysis(requestContext)
	if response.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", response.Code, response.Body.String())
	}
	var result struct {
		Data domain.AnalysisRun `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Data.Status != "pending" || result.Data.ID == uuid.Nil {
		t.Fatalf("expected saved pending job, got %+v", result.Data)
	}
	if response.Header().Get("Location") != "/api/v1/analyses/"+result.Data.ID.String() {
		t.Fatalf("unexpected job location: %s", response.Header().Get("Location"))
	}
}
