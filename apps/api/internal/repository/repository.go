package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/rbc/ev-station/apps/api/internal/domain"
)

var ErrNotFound = errorString("not found")
var ErrSiteEvidenceLimit = errorString("site evidence limit exceeded")

const MaxSiteEvidenceFiles = 10
const MaxSiteEvidenceBytes = 50 << 20

type errorString string

func (e errorString) Error() string { return string(e) }

type Repository interface {
	CreateUser(context.Context, domain.User, string) (domain.User, error)
	GetUserByEmail(context.Context, string) (domain.User, string, error)
	GetUserByID(context.Context, uuid.UUID) (domain.User, error)
	ListUsers(context.Context) ([]domain.User, error)
	UpdateUser(context.Context, domain.User, *string) (domain.User, error)
	DeleteUser(context.Context, uuid.UUID) error
	SetLineNotificationRecipient(context.Context, string) error
	GetLineNotificationRecipient(context.Context) (string, error)
	SetSiteAccess(context.Context, domain.SiteAccess) error
	ListSiteAccess(context.Context, uuid.UUID) ([]domain.SiteAccess, error)
	ListAccessibleSiteIDs(context.Context, uuid.UUID) ([]uuid.UUID, error)
	DeleteSiteAccess(context.Context, uuid.UUID, uuid.UUID) error
	CreateSite(context.Context, domain.Site) (domain.Site, error)
	ListSites(context.Context) ([]domain.Site, error)
	GetSite(context.Context, uuid.UUID) (domain.Site, error)
	UpdateSite(context.Context, domain.Site) (domain.Site, error)
	DeleteSite(context.Context, uuid.UUID) error
	AddSiteImages(context.Context, uuid.UUID, []domain.SiteImage) error
	GetSiteImages(context.Context, uuid.UUID) ([]domain.SiteImage, error)
	ListSiteImages(context.Context, uuid.UUID) ([]domain.SiteImage, error)
	GetSiteImage(context.Context, uuid.UUID, uuid.UUID) (domain.SiteImage, error)
	CreateAnalysis(context.Context, domain.AnalysisRun) (domain.AnalysisRun, error)
	ClaimNextAnalysis(context.Context) (domain.AnalysisRun, error)
	CompleteAnalysis(context.Context, domain.AnalysisRun) error
	UpdateAnalysisScoring(context.Context, domain.AnalysisRun) error
	UpdateAnalysisStationRecommendation(context.Context, uuid.UUID, []byte) error
	UpdateAnalysisAIAssessments(context.Context, uuid.UUID, []byte) error
	RecordAPIUsage(context.Context, string, int64) error
	ListAPIUsage(context.Context, time.Time) ([]domain.APIUsageSummary, error)
	GetAnalysis(context.Context, uuid.UUID) (domain.AnalysisRun, error)
	GetLatestCompletedAnalysisForSite(context.Context, uuid.UUID) (domain.AnalysisRun, error)
}
