package analysis

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/rbc/ev-station/apps/api/internal/advisory"
	"github.com/rbc/ev-station/apps/api/internal/domain"
	"github.com/rbc/ev-station/apps/api/internal/provider"
	"github.com/rbc/ev-station/apps/api/internal/repository"
	"github.com/rbc/ev-station/apps/api/internal/scoring"
)

type Service struct {
	repo     repository.Repository
	provider provider.AnalysisProvider
	scoring  *scoring.Engine
	aiScorer *advisory.Service
}

func NewService(repo repository.Repository, dataProvider provider.AnalysisProvider, scoringEngine *scoring.Engine, aiScorer ...*advisory.Service) *Service {
	service := &Service{repo: repo, provider: dataProvider, scoring: scoringEngine}
	if len(aiScorer) > 0 {
		service.aiScorer = aiScorer[0]
	}
	return service
}

func (s *Service) Run(ctx context.Context, siteID uuid.UUID, radius int) (domain.AnalysisRun, error) {
	site, err := s.repo.GetSite(ctx, siteID)
	if err != nil {
		return domain.AnalysisRun{}, err
	}
	if radius == 0 {
		radius = 3000
	}
	now := time.Now().UTC()
	run := domain.AnalysisRun{
		ID: uuid.New(), SiteID: siteID, Status: "running", AnalysisRadiusMeters: radius,
		AssessmentStatus: domain.DataPreliminary, Recommendation: "Analysis in progress.", StartedAt: now, CreatedAt: now,
	}
	if _, err = s.repo.CreateAnalysis(ctx, run); err != nil {
		return domain.AnalysisRun{}, err
	}

	observations, err := s.provider.Collect(ctx, site, radius)
	if err != nil {
		run.Status = "failed"
		run.Recommendation = "Analysis failed while collecting provider data."
		_ = s.repo.CompleteAnalysis(ctx, run)
		return run, err
	}

	for _, observation := range observations {
		metric := domain.Metric{
			ID: uuid.New(), AnalysisRunID: run.ID, Type: observation.MetricType,
			RawValue: observation.RawValue, NormalizedScore: observation.NormalizedScore,
			Status: observation.Status, Source: observation.Source, Assumptions: observation.Assumptions, CreatedAt: time.Now().UTC(),
		}
		run.Metrics = append(run.Metrics, metric)
	}
	run.Metrics = append(run.Metrics, s.siteReadinessMetric(ctx, run.ID, siteID))
	s.refreshSiteRequirementsMetric(&run)

	result := s.scoring.EvaluatePreliminary(run.Metrics)
	s.applyDeterministicScore(&run, result)
	// Investment status and score must remain reproducible from the collected
	// evidence and published scoring rules. Gemini is used separately for the
	// site-surface visual assessment and the staff-requested narrative; it must
	// never revise a metric score or the resulting recommendation.
	run.Status = "completed"
	completed := time.Now().UTC()
	run.CompletedAt = &completed
	if err = s.repo.CompleteAnalysis(ctx, run); err != nil {
		return domain.AnalysisRun{}, err
	}
	return run, nil
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (domain.AnalysisRun, error) {
	return s.repo.GetAnalysis(ctx, id)
}

func (s *Service) GetLatestCompletedForSite(ctx context.Context, siteID uuid.UUID) (domain.AnalysisRun, error) {
	return s.repo.GetLatestCompletedAnalysisForSite(ctx, siteID)
}

// RecalculatePreliminary applies the current deterministic screening rules to
// the evidence already stored on a completed run. This makes older runs usable
// without silently replacing their provider data.
func (s *Service) RecalculatePreliminary(ctx context.Context, id uuid.UUID) (domain.AnalysisRun, error) {
	run, err := s.repo.GetAnalysis(ctx, id)
	if err != nil {
		return domain.AnalysisRun{}, err
	}
	normalizeIncompleteLegacyPOI(&run)
	s.refreshSiteReadinessMetric(ctx, &run)
	s.refreshSiteRequirementsMetric(&run)
	result := s.scoring.EvaluatePreliminary(run.Metrics)
	s.applyDeterministicScore(&run, result)
	// Recalculation is deliberately deterministic: it corrects historical
	// scores from already-stored evidence without spending an AI request or
	// allowing an older AI suggestion to override the current scoring rules.
	if err := s.repo.UpdateAnalysisScoring(ctx, run); err != nil {
		return domain.AnalysisRun{}, err
	}
	return run, nil
}

// normalizeIncompleteLegacyPOI repairs reports written before POI coverage was
// stored. A zero count from a community map is not proof of zero nearby places.
func normalizeIncompleteLegacyPOI(run *domain.AnalysisRun) {
	const assumption = "The place query returned zero mapped records. This is treated as incomplete map coverage, not evidence that no important places exist, and is excluded from scoring."
	for index := range run.Metrics {
		metric := &run.Metrics[index]
		if metric.Type != "poi" || len(metric.RawValue) == 0 {
			continue
		}
		var raw map[string]any
		if json.Unmarshal(metric.RawValue, &raw) != nil {
			continue
		}
		count, ok := raw["count"].(float64)
		if !ok || count != 0 {
			continue
		}
		if coverageComplete, exists := raw["coverageComplete"].(bool); exists && coverageComplete {
			continue
		}
		raw["coverageComplete"] = false
		updated, err := json.Marshal(raw)
		if err != nil {
			continue
		}
		metric.RawValue = updated
		metric.Status = domain.DataPreliminary
		metric.Assumptions = appendScoringRule(metric.Assumptions, assumption)
	}
}

func (s *Service) refreshSiteReadinessMetric(ctx context.Context, run *domain.AnalysisRun) {
	metric := s.siteReadinessMetric(ctx, run.ID, run.SiteID)
	for index := range run.Metrics {
		if run.Metrics[index].Type == "site_readiness" {
			metric.ID = run.Metrics[index].ID
			metric.CreatedAt = run.Metrics[index].CreatedAt
			run.Metrics[index] = metric
			return
		}
	}
	run.Metrics = append(run.Metrics, metric)
}

func (s *Service) refreshSiteRequirementsMetric(run *domain.AnalysisRun) {
	metric := siteRequirementsMetric(run.ID, run.Metrics)
	for index := range run.Metrics {
		if run.Metrics[index].Type == "site_requirements" {
			metric.ID = run.Metrics[index].ID
			metric.CreatedAt = run.Metrics[index].CreatedAt
			run.Metrics[index] = metric
			return
		}
	}
	run.Metrics = append(run.Metrics, metric)
}

func siteRequirementsMetric(runID uuid.UUID, metrics []domain.Metric) domain.Metric {
	now := time.Now().UTC()
	metric := domain.Metric{ID: uuid.New(), AnalysisRunID: runID, Type: "site_requirements", Status: domain.DataPreliminary,
		Source:      domain.DataSource{Name: "Customer / field survey — site requirements", Type: "customer_supplied_site_survey", Authority: "customer_supplied", GeographicScope: "plot", SiteVerification: "preliminary_map_lookup", RetrievedAt: now},
		Assumptions: []string{"ระบบตรวจข้อมูลอัตโนมัติจากแหล่งข้อมูลที่เชื่อมต่อแล้วเท่านั้น อินเทอร์เน็ต 2.4 GHz หน้ากว้างจริง และการปรับระดับพื้นที่ยังไม่สามารถยืนยันจากข้อมูลแผนที่สาธารณะได้ จึงไม่ถูกนำมาคิดคะแนนจนกว่าจะมีแหล่งข้อมูลที่ตรวจสอบได้."}, CreatedAt: now}
	var nearestElectricalKM *float64
	var terrain json.RawMessage
	for _, item := range metrics {
		if item.Type != "site_requirements" || len(item.RawValue) == 0 || item.Status == domain.DataMissing {
			continue
		}
		var value struct {
			AssessmentType string `json:"assessmentType"`
		}
		if json.Unmarshal(item.RawValue, &value) == nil && value.AssessmentType == "gistda_elevation_sampling" {
			terrain = item.RawValue
			metric.Status = item.Status
			metric.Source = item.Source
			metric.Assumptions = append(metric.Assumptions, item.Assumptions...)
		}
	}
	for _, item := range metrics {
		if item.Type != "electrical" {
			continue
		}
		var value struct {
			NearestHighVoltageLineMeters *float64 `json:"nearestHighVoltageLineMeters"`
			NearestStationMeters         *float64 `json:"nearestStationMeters"`
			DistanceToAreaMeters         *float64 `json:"distanceToAreaMeters"`
		}
		if json.Unmarshal(item.RawValue, &value) != nil {
			continue
		}
		for _, distance := range []*float64{value.NearestHighVoltageLineMeters, value.NearestStationMeters, value.DistanceToAreaMeters} {
			if distance == nil || *distance < 0 {
				continue
			}
			km := *distance / 1000
			if nearestElectricalKM == nil || km < *nearestElectricalKM {
				nearestElectricalKM = &km
			}
		}
	}
	raw, err := json.Marshal(map[string]any{"assessmentType": "automatic_site_requirements", "nearestPublishedElectricalKm": nearestElectricalKM, "terrain": json.RawMessage(terrain)})
	if err != nil {
		return metric
	}
	metric.RawValue = raw
	if len(terrain) == 0 {
		metric.Source = domain.DataSource{Name: "Automatic screening from connected map evidence", Type: "automatic_screening", Authority: "unknown", GeographicScope: "plot", SiteVerification: "preliminary_map_lookup", RetrievedAt: now, Methodology: "Uses only already-collected public electrical-map proximity. It does not estimate internet coverage, Wi-Fi capability, frontage, levelling or actual cable routing."}
	}
	return metric
}

func ensureSiteReadinessMetric(run *domain.AnalysisRun) {
	for _, metric := range run.Metrics {
		if metric.Type == "site_readiness" {
			return
		}
	}
	run.Metrics = append(run.Metrics, domain.Metric{
		ID: uuid.New(), AnalysisRunID: run.ID, Type: "site_readiness", Status: domain.DataMissing,
		Source:      domain.DataSource{Name: "Customer / field survey — site condition", Type: "customer_supplied_site_survey", Authority: "customer_supplied", GeographicScope: "plot", SiteVerification: "preliminary_map_lookup", RetrievedAt: time.Now().UTC()},
		Assumptions: []string{"Site-condition data is required before Gemini can assess or score this site."}, CreatedAt: time.Now().UTC(),
	})
}

func (s *Service) siteReadinessMetric(ctx context.Context, runID, siteID uuid.UUID) domain.Metric {
	now := time.Now().UTC()
	metric := domain.Metric{ID: uuid.New(), AnalysisRunID: runID, Type: "site_readiness", Status: domain.DataMissing,
		Source:      domain.DataSource{Name: "Customer / field survey — site condition", Type: "customer_supplied_site_survey", Authority: "customer_supplied", GeographicScope: "plot", SiteVerification: "preliminary_map_lookup", RetrievedAt: now},
		Assumptions: []string{"Site-condition photos are required before Gemini can assess the visible ground surface."}, CreatedAt: now}
	images, err := s.repo.GetSiteImages(ctx, siteID)
	if err != nil {
		return metric
	}
	visualEvidence := make([]domain.SiteImage, 0, len(images))
	for _, image := range images {
		if image.MIMEType == "image/jpeg" || image.MIMEType == "image/png" || image.MIMEType == "image/webp" {
			visualEvidence = append(visualEvidence, image)
		}
	}
	if len(visualEvidence) == 0 {
		metric.Assumptions = []string{"Site-condition photos are required before Gemini can assess the visible ground surface. PDF documents are retained as supporting evidence."}
		return metric
	}
	if s.aiScorer == nil {
		metric.Assumptions = []string{"Site-condition photos were supplied, but Gemini is not configured to analyze them."}
		return metric
	}
	assessment, err := s.aiScorer.AnalyzeSiteSurface(ctx, visualEvidence, "th")
	if err != nil {
		metric.Assumptions = []string{"Site-condition photos were supplied, but Gemini could not analyze them right now."}
		return metric
	}
	rawValue, err := json.Marshal(map[string]any{"analysisScope": "ground_surface_only", "imageCount": len(visualEvidence), "summary": assessment.Summary, "suitability": assessment.Suitability, "score": assessment.Score, "surfaceTypes": assessment.SurfaceTypes, "observedRisks": assessment.ObservedRisks, "recommendedImprovements": assessment.RecommendedImprovements, "disclaimer": assessment.Disclaimer, "model": assessment.Model})
	if err != nil {
		return metric
	}
	metric.RawValue = rawValue
	metric.Status = domain.DataPreliminary
	metric.Source = domain.DataSource{Name: "Customer-supplied site photos analyzed by Gemini", Type: "customer_supplied_image_analysis", Authority: "customer_supplied", GeographicScope: "plot", SiteVerification: "preliminary_map_lookup", RetrievedAt: now, Methodology: "Gemini assessed visible ground-surface conditions from customer-supplied images only."}
	metric.Assumptions = []string{"This is a preliminary visual assessment of ground surface only; it is not part of the location score.", "Photos cannot confirm soil bearing capacity, underground conditions, drainage capacity, or engineering suitability."}
	return metric
}

func (s *Service) applyDeterministicScore(run *domain.AnalysisRun, result scoring.PreliminaryResult) {
	run.Scoring = &result.Summary
	for index := range run.Metrics {
		if score, exists := result.MetricScores[run.Metrics[index].Type]; exists {
			run.Metrics[index].NormalizedScore = &score
			run.Metrics[index].Assumptions = appendScoringRule(run.Metrics[index].Assumptions, scoring.ScoringRuleAssumption(result.Rules[run.Metrics[index].Type]))
		} else {
			run.Metrics[index].NormalizedScore = nil
		}
	}
	if result.Overall != nil {
		run.OverallScore = result.Overall
		run.AssessmentStatus = domain.DataPreliminary
		run.Recommendation = screeningRecommendation(*result.Overall)
		return
	}
	run.OverallScore = nil
	run.AssessmentStatus = domain.DataMissing
	run.Recommendation = "A preliminary score requires at least 60% weighted data coverage."
}

func screeningRecommendation(score float64) string {
	if score < 60 {
		return "คะแนนคัดกรองเบื้องต้นต่ำกว่า 60/100: ยังไม่แนะนำลงทุน"
	}
	return "คะแนนคัดกรองเบื้องต้นตั้งแต่ 60/100 ขึ้นไป: น่าลงทุน"
}

func appendScoringRule(assumptions []string, rule string) []string {
	for _, assumption := range assumptions {
		if assumption == rule {
			return assumptions
		}
	}
	return append(assumptions, rule)
}
