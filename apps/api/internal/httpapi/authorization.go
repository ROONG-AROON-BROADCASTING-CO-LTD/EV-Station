package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rbc/ev-station/apps/api/internal/domain"
	"github.com/rbc/ev-station/apps/api/internal/repository"
)

type sitePermission string

const (
	siteRead  sitePermission = "read"
	siteWrite sitePermission = "write"
)

func requestUser(c *gin.Context) (uuid.UUID, domain.UserRole, bool) {
	value, exists := c.Get("userId")
	if !exists {
		writeError(c, http.StatusUnauthorized, "AUTH_REQUIRED", "Please sign in to continue.")
		return uuid.Nil, "", false
	}
	userIDValue, ok := value.(string)
	if !ok {
		writeError(c, http.StatusUnauthorized, "INVALID_TOKEN", "Your session is invalid. Please sign in again.")
		return uuid.Nil, "", false
	}
	userID, err := uuid.Parse(userIDValue)
	if err != nil {
		writeError(c, http.StatusUnauthorized, "INVALID_TOKEN", "Your session is invalid. Please sign in again.")
		return uuid.Nil, "", false
	}
	roleValue, exists := c.Get("role")
	role, ok := roleValue.(domain.UserRole)
	if !exists || !ok {
		writeError(c, http.StatusUnauthorized, "INVALID_TOKEN", "Your session is invalid. Please sign in again.")
		return uuid.Nil, "", false
	}
	return userID, role, true
}

func (h *Handler) requireOwner(c *gin.Context) bool {
	_, role, ok := requestUser(c)
	if ok && role != domain.RoleOwner {
		writeError(c, http.StatusForbidden, "OWNER_REQUIRED", "Only an owner can perform this action.")
		return false
	}
	return ok
}

func (h *Handler) requireSitePermission(c *gin.Context, siteID uuid.UUID, permission sitePermission) bool {
	userID, role, ok := requestUser(c)
	if !ok {
		return false
	}
	if role == domain.RoleOwner {
		return true
	}
	access, err := h.repo.ListSiteAccess(c.Request.Context(), siteID)
	if err == repository.ErrNotFound {
		writeError(c, http.StatusNotFound, "SITE_NOT_FOUND", "Site not found.")
		return false
	}
	if err != nil {
		writeError(c, http.StatusInternalServerError, "ACCESS_UNAVAILABLE", "Unable to verify site access.")
		return false
	}
	for _, grant := range access {
		if grant.UserID != userID {
			continue
		}
		if permission == siteRead || grant.Role == string(domain.RoleOwner) || grant.Role == string(domain.RoleSales) {
			return true
		}
	}
	writeError(c, http.StatusForbidden, "SITE_ACCESS_DENIED", "You do not have permission to access this site.")
	return false
}

func (h *Handler) requireAnalysisPermission(c *gin.Context, analysisID uuid.UUID, permission sitePermission) (domain.AnalysisRun, bool) {
	run, err := h.analyses.Get(c.Request.Context(), analysisID)
	if err == repository.ErrNotFound {
		writeError(c, http.StatusNotFound, "ANALYSIS_NOT_FOUND", "Analysis not found.")
		return domain.AnalysisRun{}, false
	}
	if err != nil {
		writeError(c, http.StatusInternalServerError, "ANALYSIS_LOAD_FAILED", "Unable to load the analysis.")
		return domain.AnalysisRun{}, false
	}
	if !h.requireSitePermission(c, run.SiteID, permission) {
		return domain.AnalysisRun{}, false
	}
	return run, true
}
