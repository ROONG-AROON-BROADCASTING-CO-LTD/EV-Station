package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rbc/ev-station/apps/api/internal/advisory"
)

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
	refresh := c.Query("refresh") == "true"
	if !refresh && len(run.StationRecommendation) > 0 {
		var cached advisory.StationRecommendation
		if err := json.Unmarshal(run.StationRecommendation, &cached); err == nil {
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
	site, err := h.repo.GetSite(c.Request.Context(), run.SiteID)
	if err != nil {
		writeError(c, 500, "SITE_LOAD_FAILED", "Unable to load site.")
		return
	}
	if h.advisory == nil {
		writeError(c, 503, "AI_NOT_CONFIGURED", "AI is not configured.")
		return
	}
	result, err := h.advisory.Station(c.Request.Context(), run, site)
	if err != nil {
		writeError(c, 502, "AI_UNAVAILABLE", "Unable to generate a station recommendation.")
		return
	}
	if err := h.repo.RecordAPIUsage(c.Request.Context(), "gemini-advisory", 1); err != nil {
		writeError(c, 500, "API_USAGE_RECORD_FAILED", "Unable to record API usage.")
		return
	}
	persisted, err := json.Marshal(result)
	if err != nil {
		writeError(c, 500, "STATION_RECOMMENDATION_ENCODE_FAILED", "Unable to save the station recommendation.")
		return
	}
	if err := h.repo.UpdateAnalysisStationRecommendation(c.Request.Context(), run.ID, persisted); err != nil {
		writeError(c, 500, "STATION_RECOMMENDATION_SAVE_FAILED", "Unable to save the station recommendation.")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
}
