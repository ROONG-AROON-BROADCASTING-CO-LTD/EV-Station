package httpapi

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/rbc/ev-station/apps/api/internal/domain"
)

const fallbackRetryCooldown = time.Minute

func (h *Handler) cachedFallbackRetryDue(ctx context.Context, siteID, userID uuid.UUID, assessment domain.AIAssessment, role domain.UserRole, now time.Time) bool {
	staff := role == domain.RoleSuperAdmin || role == domain.RoleAdmin || role == domain.RoleSales
	// A fallback is temporary, but repeated views must not spend provider quota
	// continuously. Customers/viewers may read it without triggering paid work.
	if !staff || assessment.Model != "system-fallback" || now.Sub(assessment.GeneratedAt) < fallbackRetryCooldown {
		return false
	}
	if role != domain.RoleSales {
		return true
	}
	// Read-only site grants must keep working when the fallback ages. Only sales
	// users with an existing write grant may turn a cached read into generation.
	access, err := h.repo.ListSiteAccess(ctx, siteID)
	if err != nil {
		return false
	}
	for _, grant := range access {
		if grant.UserID == userID && (grant.Role == string(domain.RoleSales) || grant.Role == string(domain.RoleCustomer)) {
			return true
		}
	}
	return false
}
