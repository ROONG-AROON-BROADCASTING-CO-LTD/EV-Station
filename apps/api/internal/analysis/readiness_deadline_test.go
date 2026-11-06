package analysis

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rbc/ev-station/apps/api/internal/domain"
	"github.com/rbc/ev-station/apps/api/internal/provider"
	"github.com/rbc/ev-station/apps/api/internal/repository"
	"github.com/rbc/ev-station/apps/api/internal/scoring"
)

type waitingEvidenceRepository struct {
	repository.Repository
	started chan struct{}
	release chan struct{}
}

func (r waitingEvidenceRepository) GetSiteImages(ctx context.Context, _ uuid.UUID) ([]domain.SiteImage, error) {
	close(r.started)
	select {
	case <-r.release:
		return nil, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func TestAnalysisCancellationDoesNotWaitForSiteEvidence(t *testing.T) {
	repo := waitingEvidenceRepository{Repository: repository.NewMemory(), started: make(chan struct{}), release: make(chan struct{})}
	site := domain.Site{ID: uuid.New(), Name: "Deadline fixture", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if _, err := repo.CreateSite(context.Background(), site); err != nil {
		t.Fatal(err)
	}
	engine, err := scoring.New(scoring.DefaultWeights)
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(repo, provider.FixtureProvider{}, engine)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	type result struct {
		run domain.AnalysisRun
		err error
	}
	completed := make(chan result, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		run, err := service.Run(ctx, site.ID, 3000)
		completed <- result{run, err}
	}()
	defer func() {
		close(repo.release)
		<-done
	}()
	select {
	case <-repo.started:
	case <-time.After(time.Second):
		t.Fatal("site evidence lookup did not start")
	}
	cancel()
	select {
	case got := <-completed:
		if got.err != nil || got.run.Status != "completed" {
			t.Fatalf("available evidence must finish as a completed report: status=%s error=%v", got.run.Status, got.err)
		}
		var available, readiness bool
		for _, metric := range got.run.Metrics {
			if metric.Type == "population" && metric.Status != domain.DataMissing {
				available = true
			}
			if metric.Type == "site_readiness" {
				readiness = metric.Status == domain.DataMissing && strings.Contains(strings.Join(metric.Assumptions, " "), "did not finish")
			}
		}
		if !available || !readiness {
			t.Fatalf("expected retained provider data and an explicitly unfinished visual assessment: available=%v readiness=%v", available, readiness)
		}
		stored, err := repo.GetAnalysis(context.Background(), got.run.ID)
		if err != nil || stored.Status != "completed" {
			t.Fatalf("terminal state must be persisted after cancellation: status=%s error=%v", stored.Status, err)
		}
	case <-time.After(300 * time.Millisecond):
		t.Fatal("analysis remained blocked on independent site-evidence lookup after its work context ended")
	}
}

func TestReadinessAlreadyAvailableIsPreservedAtDeadline(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	runID := uuid.New()
	expected := domain.Metric{ID: uuid.New(), AnalysisRunID: runID, Type: "site_readiness", Status: domain.DataPreliminary}
	for range 100 {
		readiness := make(chan domain.Metric, 1)
		readiness <- expected
		got := waitForSiteReadiness(ctx, runID, readiness)
		if got.ID != expected.ID || got.Status != expected.Status {
			t.Fatalf("already completed visual assessment was lost at cancellation: %+v", got)
		}
	}
}
