package analysis

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rbc/ev-station/apps/api/internal/advisory"
	"github.com/rbc/ev-station/apps/api/internal/domain"
	"github.com/rbc/ev-station/apps/api/internal/provider"
	"github.com/rbc/ev-station/apps/api/internal/repository"
	"github.com/rbc/ev-station/apps/api/internal/scoring"
)

func TestNormalizeIncompleteLegacyPOIMarksZeroCountAsPreliminary(t *testing.T) {
	run := domain.AnalysisRun{Metrics: []domain.Metric{{
		Type:            "poi",
		Status:          domain.DataVerified,
		NormalizedScore: floatPointer(10),
		RawValue:        json.RawMessage(`{"count":0,"radiusMeters":3000}`),
	}}}

	normalizeIncompleteLegacyPOI(&run)
	metric := run.Metrics[0]
	if metric.Status != domain.DataPreliminary {
		t.Fatalf("expected preliminary POI status, got %s", metric.Status)
	}
	var raw map[string]any
	if err := json.Unmarshal(metric.RawValue, &raw); err != nil || raw["coverageComplete"] != false {
		t.Fatalf("expected incomplete coverage marker, raw=%s err=%v", metric.RawValue, err)
	}
}

func floatPointer(value float64) *float64 { return &value }

type waitingProvider struct {
	started chan<- struct{}
	release <-chan struct{}
}

func (p waitingProvider) Collect(ctx context.Context, site domain.Site, radius int) ([]provider.Observation, error) {
	p.started <- struct{}{}
	select {
	case <-p.release:
		return provider.FixtureProvider{}.Collect(ctx, site, radius)
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func TestAnalysisOverlapsSitePhotosWithProviderCollection(t *testing.T) {
	repo := repository.NewMemory()
	now := time.Now().UTC()
	site := domain.Site{ID: uuid.New(), Name: "Site with photos", LandSize: 100, LandSizeUnit: "sqm", CreatedAt: now, UpdatedAt: now}
	if _, err := repo.CreateSite(context.Background(), site); err != nil {
		t.Fatal(err)
	}
	if err := repo.AddSiteImages(context.Background(), site.ID, []domain.SiteImage{{ID: uuid.New(), SiteID: site.ID, MIMEType: "image/jpeg", Data: []byte("photo"), CreatedAt: now}}); err != nil {
		t.Fatal(err)
	}
	providerStarted := make(chan struct{}, 1)
	geminiStarted := make(chan struct{}, 1)
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		geminiStarted <- struct{}{}
		select {
		case <-release:
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"{\"summary\":\"Visible gravel\",\"suitability\":\"moderate\",\"score\":55,\"disclaimer\":\"Photo only\"}"}]}}]}`))
		case <-request.Context().Done():
		}
	}))
	defer server.Close()
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
	}()
	engine, _ := scoring.New(scoring.DefaultWeights)
	gemini := advisory.NewGeminiService(advisory.GeminiConfig{APIKey: "test-key", Model: "test-model", BaseURL: server.URL, Timeout: 3 * time.Second}, server.Client())
	service := NewService(repo, waitingProvider{started: providerStarted, release: release}, engine, gemini)
	result := make(chan domain.AnalysisRun, 1)
	go func() {
		run, _ := service.Run(context.Background(), site.ID, 3000)
		result <- run
	}()
	for _, started := range []<-chan struct{}{providerStarted, geminiStarted} {
		select {
		case <-started:
		case <-time.After(2 * time.Second):
			t.Fatal("provider collection and site-photo analysis did not start concurrently")
		}
	}
	close(release)
	select {
	case run := <-result:
		if run.Status != "completed" {
			t.Fatalf("expected completed run, got %s", run.Status)
		}
		found := false
		for _, metric := range run.Metrics {
			if metric.Type == "site_readiness" {
				found = metric.Status == domain.DataPreliminary
			}
		}
		if !found {
			t.Fatal("parallel visual assessment was not persisted")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("analysis did not finish after both independent calls completed")
	}
}

func TestQueuedAnalysisCompletesAfterRequestContextEnds(t *testing.T) {
	repo := repository.NewMemory()
	now := time.Now().UTC()
	site := domain.Site{ID: uuid.New(), Name: "Queued site", LandSize: 100, LandSizeUnit: "sqm", CreatedAt: now, UpdatedAt: now}
	if _, err := repo.CreateSite(context.Background(), site); err != nil {
		t.Fatal(err)
	}
	engine, _ := scoring.New(scoring.DefaultWeights)
	service := NewService(repo, provider.FixtureProvider{}, engine)
	requestCtx, cancelRequest := context.WithCancel(context.Background())
	queued, err := service.Enqueue(requestCtx, site.ID, 3000)
	if err != nil || queued.Status != "pending" {
		t.Fatalf("expected pending job: %+v, %v", queued, err)
	}
	cancelRequest()
	workerCtx, stopWorker := context.WithCancel(context.Background())
	defer stopWorker()
	service.StartWorker(workerCtx, slog.New(slog.NewTextHandler(io.Discard, nil)))
	deadline := time.After(2 * time.Second)
	for {
		stored, getErr := service.Get(context.Background(), queued.ID)
		if getErr != nil {
			t.Fatal(getErr)
		}
		if stored.Status == "completed" {
			if stored.OverallScore == nil {
				t.Fatal("completed job did not persist its score")
			}
			return
		}
		select {
		case <-deadline:
			t.Fatalf("queued job never completed, status=%s", stored.Status)
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func TestRunWithExplicitFixtureProvider(t *testing.T) {
	repo := repository.NewMemory()
	now := time.Now().UTC()
	site := domain.Site{ID: uuid.New(), Name: "Test site", Address: "Bangkok", LandSize: 100, LandSizeUnit: "sqm", CreatedAt: now, UpdatedAt: now}
	if _, err := repo.CreateSite(context.Background(), site); err != nil {
		t.Fatal(err)
	}
	engine, _ := scoring.New(scoring.DefaultWeights)
	service := NewService(repo, provider.FixtureProvider{}, engine)
	run, err := service.Run(context.Background(), site.ID, 3000)
	if err != nil {
		t.Fatal(err)
	}
	if run.Status != "completed" || run.OverallScore == nil {
		t.Fatalf("unexpected run: %+v", run)
	}
	foundSiteReadiness, foundSiteRequirements := false, false
	for _, metric := range run.Metrics {
		if metric.Type == "site_readiness" {
			foundSiteReadiness = true
			if metric.Source.Type != "customer_supplied_site_survey" || metric.Status != domain.DataMissing || metric.NormalizedScore != nil {
				t.Fatalf("site condition must remain unscored until survey evidence is supplied: %+v", metric)
			}
			continue
		}
		if metric.Type == "site_requirements" {
			foundSiteRequirements = true
			if metric.Source.Type != "automatic_screening" || metric.Status != domain.DataPreliminary || metric.NormalizedScore != nil {
				t.Fatalf("automatic site requirements must remain unscored without a connected evidence source: %+v", metric)
			}
			continue
		}
		if metric.Source.Type != "fixture" || metric.Status != domain.DataPreliminary {
			t.Fatalf("fixture provenance was not preserved: %+v", metric)
		}
	}
	if !foundSiteReadiness || !foundSiteRequirements {
		t.Fatal("expected site-readiness and site-requirements metrics")
	}
}

func TestUnavailableProviderDoesNotInventScore(t *testing.T) {
	repo := repository.NewMemory()
	now := time.Now().UTC()
	site := domain.Site{ID: uuid.New(), Name: "Test site", Address: "Bangkok", LandSize: 100, LandSizeUnit: "sqm", CreatedAt: now, UpdatedAt: now}
	_, _ = repo.CreateSite(context.Background(), site)
	engine, _ := scoring.New(scoring.DefaultWeights)
	run, err := NewService(repo, provider.UnavailableProvider{}, engine).Run(context.Background(), site.ID, 3000)
	if err != nil {
		t.Fatal(err)
	}
	if run.OverallScore != nil {
		t.Fatal("overall score must be absent when required factual data is missing")
	}
}

func TestRunDoesNotAllowGeminiToChangeEvidenceBasedScore(t *testing.T) {
	repo := repository.NewMemory()
	now := time.Now().UTC()
	site := domain.Site{ID: uuid.New(), Name: "Test site", Address: "Bangkok", LandSize: 100, LandSizeUnit: "sqm", CreatedAt: now, UpdatedAt: now}
	if _, err := repo.CreateSite(context.Background(), site); err != nil {
		t.Fatal(err)
	}
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		calls++
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"{\"metricScores\":[{\"metricType\":\"traffic\",\"score\":0}]}"}]}}]}`))
	}))
	defer server.Close()
	engine, _ := scoring.New(scoring.DefaultWeights)
	gemini := advisory.NewGeminiService(advisory.GeminiConfig{APIKey: "test-key", Model: "test-model", BaseURL: server.URL, Timeout: time.Second}, server.Client())
	run, err := NewService(repo, provider.FixtureProvider{}, engine, gemini).Run(context.Background(), site.ID, 3000)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Fatalf("analysis scoring must not call Gemini, got %d calls", calls)
	}
	if run.Scoring == nil || run.Scoring.Version != scoring.PreliminaryVersion {
		t.Fatalf("expected deterministic scoring version, got %+v", run.Scoring)
	}
}

func TestGetLatestCompletedForSiteReturnsExistingRun(t *testing.T) {
	repo := repository.NewMemory()
	now := time.Now().UTC()
	site := domain.Site{ID: uuid.New(), Name: "Test site", Address: "Bangkok", LandSize: 100, LandSizeUnit: "sqm", CreatedAt: now, UpdatedAt: now}
	_, _ = repo.CreateSite(context.Background(), site)
	engine, _ := scoring.New(scoring.DefaultWeights)
	service := NewService(repo, provider.FixtureProvider{}, engine)
	first, err := service.Run(context.Background(), site.ID, 3000)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Millisecond)
	second, err := service.Run(context.Background(), site.ID, 3000)
	if err != nil {
		t.Fatal(err)
	}
	latest, err := service.GetLatestCompletedForSite(context.Background(), site.ID)
	if err != nil {
		t.Fatal(err)
	}
	if latest.ID != second.ID || latest.ID == first.ID {
		t.Fatalf("expected latest completed analysis %s, got %s", second.ID, latest.ID)
	}
}

func TestRecalculatePreliminaryUpgradesLegacyEvidenceWithoutReplacingIt(t *testing.T) {
	repo := repository.NewMemory()
	now := time.Now().UTC()
	site := domain.Site{ID: uuid.New(), Name: "Test site", Address: "Bangkok", LandSize: 100, LandSizeUnit: "sqm", CreatedAt: now, UpdatedAt: now}
	_, _ = repo.CreateSite(context.Background(), site)
	engine, _ := scoring.New(scoring.DefaultWeights)
	service := NewService(repo, provider.FixtureProvider{}, engine)
	run, err := service.Run(context.Background(), site.ID, 3000)
	if err != nil {
		t.Fatal(err)
	}
	// Simulate an analysis created before preliminary scoring was introduced.
	run.OverallScore = nil
	run.Scoring = nil
	run.AssessmentStatus = domain.DataMissing
	for index := range run.Metrics {
		run.Metrics[index].NormalizedScore = nil
		if run.Metrics[index].Type == "traffic" {
			run.Metrics[index].RawValue = []byte(`{"aadt":1000}`)
		}
		if run.Metrics[index].Type == "electrical" {
			run.Metrics[index].RawValue = []byte(`{"assessmentType":"pea_public_grid_evidence","nearestHighVoltageLineMeters":3674}`)
		}
		if run.Metrics[index].Type == "flood" {
			run.Metrics[index].RawValue = []byte(`{"assessmentType":"administrative_district_historical_reports","periodStartYear":2562,"periodEndYear":2567,"reportedFloodYearCount":3}`)
		}
	}
	if err = repo.UpdateAnalysisScoring(context.Background(), run); err != nil {
		t.Fatal(err)
	}

	recalculated, err := service.RecalculatePreliminary(context.Background(), run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if recalculated.OverallScore == nil || recalculated.Scoring == nil {
		t.Fatalf("expected legacy evidence to receive a preliminary score: %+v", recalculated)
	}
	if recalculated.Scoring.Version != scoring.PreliminaryVersion {
		t.Fatalf("unexpected scoring version: %s", recalculated.Scoring.Version)
	}
	if recalculated.Metrics[0].Source.Type != "fixture" || recalculated.Metrics[0].Status != domain.DataPreliminary {
		t.Fatal("recalculation must preserve evidence provenance and status")
	}
}
