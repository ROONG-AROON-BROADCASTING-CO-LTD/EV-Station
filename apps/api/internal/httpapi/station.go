package httpapi

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rbc/ev-station/apps/api/internal/advisory"
	"github.com/rbc/ev-station/apps/api/internal/domain"
)

// generateAndStoreStationRecommendation creates the initial S/M/L planning
// recommendation from a completed analysis and stores it with that analysis.
// The normal, unconfirmed-grid path is deterministic and does not call Gemini.
func (h *Handler) generateAndStoreStationRecommendation(ctx context.Context, run domain.AnalysisRun) (advisory.StationRecommendation, []byte, error) {
	site, err := h.repo.GetSite(ctx, run.SiteID)
	if err != nil {
		return advisory.StationRecommendation{}, nil, err
	}
	if h.advisory == nil {
		return advisory.StationRecommendation{}, nil, advisory.ErrNotConfigured
	}
	result, err := h.advisory.Station(ctx, run, site)
	if err != nil {
		return advisory.StationRecommendation{}, nil, err
	}
	if result.GeneratedByAI {
		if err := h.repo.RecordAPIUsage(ctx, "gemini-advisory", 1); err != nil {
			return advisory.StationRecommendation{}, nil, err
		}
	}
	persisted, err := json.Marshal(result)
	if err != nil {
		return advisory.StationRecommendation{}, nil, err
	}
	if err := h.repo.UpdateAnalysisStationRecommendation(ctx, run.ID, persisted); err != nil {
		return advisory.StationRecommendation{}, nil, err
	}
	return result, persisted, nil
}

func (h *Handler) RecommendStation(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	run, ok := h.requireAnalysisPermission(c, id, siteRead)
	if !ok {
		return
	}
	if run.Status != "completed" {
		writeError(c, 409, "ANALYSIS_NOT_READY", "Analysis must be completed.")
		return
	}
	site, err := h.repo.GetSite(c.Request.Context(), run.SiteID)
	if err != nil {
		writeError(c, http.StatusNotFound, "SITE_NOT_FOUND", "Site not found.")
		return
	}
	refresh := c.Query("refresh") == "true"
	if !refresh && advisory.StationBlocker(site) == "" && len(run.StationRecommendation) > 0 {
		var cached advisory.StationRecommendation
		if err := json.Unmarshal(run.StationRecommendation, &cached); err == nil && cached.RecommendationAvailable && cached.RecommendationStage != "" {
			c.JSON(http.StatusOK, gin.H{"data": cached})
			return
		}
	}
	if !h.requireStaff(c) {
		return
	}
	if !h.requireSitePermission(c, run.SiteID, siteWrite) {
		return
	}
	result, _, err := h.generateAndStoreStationRecommendation(c.Request.Context(), run)
	if err != nil {
		writeError(c, 502, "AI_UNAVAILABLE", "Unable to generate a station recommendation.")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
}
