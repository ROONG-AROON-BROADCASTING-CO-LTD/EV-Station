package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type DataStatus string

type UserRole string

const (
	RoleSuperAdmin UserRole = "super_admin"
	RoleAdmin      UserRole = "admin"
	RoleSales      UserRole = "sales"
	RoleCustomer   UserRole = "customer"
	// Legacy names are retained in code so existing authorization call sites
	// continue to mean the highest and lowest privilege respectively.
	RoleOwner  = RoleSuperAdmin
	RoleViewer = RoleCustomer
)

type User struct {
	ID          uuid.UUID `json:"id"`
	Email       string    `json:"email"`
	DisplayName string    `json:"displayName"`
	SalesCode   string    `json:"salesCode,omitempty"`
	Role        UserRole  `json:"role"`
	IsActive    bool      `json:"isActive"`
	CreatedAt   time.Time `json:"createdAt"`
}

type SiteAccess struct {
	SiteID uuid.UUID `json:"siteId"`
	UserID uuid.UUID `json:"userId"`
	Role   string    `json:"role"`
}

const (
	DataVerified    DataStatus = "verified"
	DataEstimated   DataStatus = "estimated"
	DataPreliminary DataStatus = "preliminary"
	DataMissing     DataStatus = "missing"
)

type Site struct {
	ID                    uuid.UUID  `json:"id"`
	ReferenceCode         string     `json:"referenceCode,omitempty"`
	Name                  string     `json:"name"`
	ContactName           string     `json:"contactName,omitempty"`
	ContactPhone          string     `json:"contactPhone,omitempty"`
	Address               string     `json:"address,omitempty"`
	Latitude              *float64   `json:"latitude,omitempty"`
	Longitude             *float64   `json:"longitude,omitempty"`
	LandSize              float64    `json:"landSize"`
	LandSizeUnit          string     `json:"landSizeUnit"`
	GoogleMapsURL         string     `json:"googleMapsUrl,omitempty"`
	Notes                 string     `json:"notes,omitempty"`
	InternetAvailable     *bool      `json:"internetAvailable,omitempty"`
	InternetSupports24GHz *bool      `json:"internetSupports24GHz,omitempty"`
	LandLevelingRequired  *bool      `json:"landLevelingRequired,omitempty"`
	FrontageMeters        *float64   `json:"frontageMeters,omitempty"`
	ElectricalExtensionKM *float64   `json:"electricalExtensionKm,omitempty"`
	InputStatus           DataStatus `json:"inputStatus"`
	CreatedAt             time.Time  `json:"createdAt"`
	UpdatedAt             time.Time  `json:"updatedAt"`
}

type CreateSiteInput struct {
	Name                  string   `json:"name" binding:"required,max=160"`
	ContactName           string   `json:"contactName" binding:"max=160"`
	ContactPhone          string   `json:"contactPhone" binding:"max=40"`
	Address               string   `json:"address" binding:"max=1000"`
	Latitude              *float64 `json:"latitude"`
	Longitude             *float64 `json:"longitude"`
	LandSize              float64  `json:"landSize" binding:"required,gt=0"`
	LandSizeUnit          string   `json:"landSizeUnit" binding:"required,oneof=sqm rai ngan sqwah"`
	GoogleMapsURL         string   `json:"googleMapsUrl" binding:"omitempty,url"`
	Notes                 string   `json:"notes" binding:"max=5000"`
	InternetAvailable     *bool    `json:"internetAvailable"`
	InternetSupports24GHz *bool    `json:"internetSupports24GHz"`
	LandLevelingRequired  *bool    `json:"landLevelingRequired"`
	FrontageMeters        *float64 `json:"frontageMeters" binding:"omitempty,gte=0,lte=100"`
	ElectricalExtensionKM *float64 `json:"electricalExtensionKm" binding:"omitempty,gte=0,lte=50"`
}

// SiteImage stores customer-supplied visual or PDF evidence for a submitted
// plot. Image bytes can be used for the site-condition assessment; PDFs are
// retained as supporting documents and are not sent to the image assessor.
type SiteImage struct {
	ID        uuid.UUID
	SiteID    uuid.UUID
	MIMEType  string
	Data      []byte
	SizeBytes int64
	CreatedAt time.Time
}

// SiteSurfaceAssessment is an image-based, preliminary assessment of the
// current ground surface only. It is intentionally not a location-score input.
type SiteSurfaceAssessment struct {
	Summary                 string   `json:"summary"`
	Suitability             string   `json:"suitability"`
	Score                   float64  `json:"score"`
	SurfaceTypes            []string `json:"surfaceTypes"`
	ObservedRisks           []string `json:"observedRisks"`
	RecommendedImprovements []string `json:"recommendedImprovements"`
	Disclaimer              string   `json:"disclaimer"`
	Model                   string   `json:"model"`
}

type DataSource struct {
	Name string `json:"name"`
	Type string `json:"type"`
	// Authority identifies who publishes the underlying dataset. GeographicScope
	// makes clear the level at which the source is factual; neither field alone
	// means that a conclusion has been confirmed for the submitted plot.
	Authority        string     `json:"authority,omitempty"`
	GeographicScope  string     `json:"geographicScope,omitempty"`
	SiteVerification string     `json:"siteVerification,omitempty"`
	ReferenceURI     string     `json:"referenceUri,omitempty"`
	DatasetVersion   string     `json:"datasetVersion,omitempty"`
	ObservedAt       *time.Time `json:"observedAt,omitempty"`
	RetrievedAt      time.Time  `json:"retrievedAt"`
	Methodology      string     `json:"methodology,omitempty"`
	License          string     `json:"license,omitempty"`
}

type Metric struct {
	ID              uuid.UUID       `json:"id"`
	AnalysisRunID   uuid.UUID       `json:"analysisRunId"`
	Type            string          `json:"type"`
	RawValue        json.RawMessage `json:"rawValue,omitempty"`
	NormalizedScore *float64        `json:"normalizedScore,omitempty"`
	Status          DataStatus      `json:"status"`
	Source          DataSource      `json:"source"`
	Assumptions     []string        `json:"assumptions"`
	CreatedAt       time.Time       `json:"createdAt"`
}

type FinancialResult struct {
	InitialInvestment    float64  `json:"initialInvestment"`
	MonthlyRevenue       float64  `json:"monthlyRevenue"`
	MonthlyOperatingCost float64  `json:"monthlyOperatingCost"`
	MonthlyProfit        float64  `json:"monthlyProfit"`
	AnnualProfit         float64  `json:"annualProfit"`
	ROIPercentage        *float64 `json:"roiPercentage,omitempty"`
	PaybackMonths        *float64 `json:"paybackMonths,omitempty"`
	Assumptions          []string `json:"assumptions"`
}

type ScoringSummary struct {
	Version                string   `json:"version"`
	CoveragePercentage     float64  `json:"coveragePercentage"`
	ScoredMetricCount      int      `json:"scoredMetricCount"`
	RequiredMetricCount    int      `json:"requiredMetricCount"`
	ExcludedMetrics        []string `json:"excludedMetrics"`
	MinimumCoveragePercent float64  `json:"minimumCoveragePercentage"`
	Limitations            []string `json:"limitations"`
}

type AnalysisRun struct {
	ID                    uuid.UUID        `json:"id"`
	SiteID                uuid.UUID        `json:"siteId"`
	Status                string           `json:"status"`
	AnalysisRadiusMeters  int              `json:"analysisRadiusMeters"`
	OverallScore          *float64         `json:"overallScore,omitempty"`
	AssessmentStatus      DataStatus       `json:"assessmentStatus"`
	Recommendation        string           `json:"recommendation"`
	Metrics               []Metric         `json:"metrics"`
	Financial             *FinancialResult `json:"financial,omitempty"`
	Scoring               *ScoringSummary  `json:"scoring,omitempty"`
	StationRecommendation json.RawMessage  `json:"stationRecommendation,omitempty"`
	AIAssessments         json.RawMessage  `json:"aiAssessments,omitempty"`
	StartedAt             time.Time        `json:"startedAt"`
	CompletedAt           *time.Time       `json:"completedAt,omitempty"`
	CreatedAt             time.Time        `json:"createdAt"`
}

// AIAssessment is explanatory output generated from an existing analysis run.
type AIAssessment struct {
	Summary        string    `json:"summary"`
	Decision       string    `json:"decision"`
	Recommendation string    `json:"recommendation"`
	Strengths      []string  `json:"strengths"`
	Risks          []string  `json:"risks"`
	RequiredChecks []string  `json:"requiredChecks"`
	Disclaimer     string    `json:"disclaimer"`
	Language       string    `json:"language"`
	Model          string    `json:"model"`
	GeneratedAt    time.Time `json:"generatedAt"`
	DecisionPolicy string    `json:"decisionPolicy"`
}

const (
	InvestmentDecisionInvest         = "invest"
	InvestmentDecisionNotRecommended = "not_recommended"
	InvestmentDecisionPolicyVersion  = "score-threshold-60-v1"
)

// InvestmentDecisionForScore is the single screening rule for the displayed
// investment status. AI can explain the evidence, but never changes this rule.
func InvestmentDecisionForScore(score *float64) string {
	if score != nil && *score >= 60 {
		return InvestmentDecisionInvest
	}
	return InvestmentDecisionNotRecommended
}

// AIScoring contains Gemini's evidence-bound scoring recommendation. It can only
// score metrics for which the backend has already collected usable evidence.
// The backend validates every value and calculates the weighted total itself.
type AIScoring struct {
	MetricScores   map[string]float64 `json:"metricScores"`
	Recommendation string             `json:"recommendation"`
	Disclaimer     string             `json:"disclaimer"`
	Language       string             `json:"language"`
	Model          string             `json:"model"`
	GeneratedAt    time.Time          `json:"generatedAt"`
}
