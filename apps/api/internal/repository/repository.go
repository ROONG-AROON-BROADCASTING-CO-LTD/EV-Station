package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/rbc/ev-station/apps/api/internal/domain"
)

var ErrNotFound = errorString("not found")

type errorString string

func (e errorString) Error() string { return string(e) }

type Repository interface {
	CreateUser(context.Context, domain.User, string) (domain.User, error)
	GetUserByEmail(context.Context, string) (domain.User, string, error)
	ListUsers(context.Context) ([]domain.User, error)
	SetLineNotificationRecipient(context.Context, string) error
	GetLineNotificationRecipient(context.Context) (string, error)
	SetSiteAccess(context.Context, domain.SiteAccess) error
	ListSiteAccess(context.Context, uuid.UUID) ([]domain.SiteAccess, error)
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
	CompleteAnalysis(context.Context, domain.AnalysisRun) error
	UpdateAnalysisScoring(context.Context, domain.AnalysisRun) error
	UpdateAnalysisStationRecommendation(context.Context, uuid.UUID, []byte) error
	UpdateAnalysisAIAssessments(context.Context, uuid.UUID, []byte) error
	GetAnalysis(context.Context, uuid.UUID) (domain.AnalysisRun, error)
	GetLatestCompletedAnalysisForSite(context.Context, uuid.UUID) (domain.AnalysisRun, error)
}
