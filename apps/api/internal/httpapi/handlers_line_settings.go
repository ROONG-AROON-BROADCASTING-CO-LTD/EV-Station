package httpapi

import (
	"context"
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/rbc/ev-station/apps/api/internal/line"
	"net/http"
	"time"
)

type lineNotificationSettings interface {
	NotificationStatus(context.Context) (line.NotificationStatus, error)
	StartNotificationPairing(context.Context) (line.PairingChallenge, error)
	CancelNotificationPairing()
	TestNotification(context.Context) error
}

func (h *Handler) lineSettings(c *gin.Context) (lineNotificationSettings, bool) {
	if !h.requireOwner(c) {
		return nil, false
	}
	c.Header("Cache-Control", "no-store")
	settings, ok := h.notifier.(lineNotificationSettings)
	if !ok {
		writeError(c, http.StatusServiceUnavailable, "LINE_NOT_CONFIGURED", "LINE notifications are not configured.")
	}
	return settings, ok
}
func (h *Handler) LineNotificationStatus(c *gin.Context) {
	settings, ok := h.lineSettings(c)
	if !ok {
		return
	}
	status, err := settings.NotificationStatus(c.Request.Context())
	if err != nil {
		writeError(c, 503, "LINE_STATUS_UNAVAILABLE", "Unable to read LINE notification settings.")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": status})
}
func (h *Handler) StartLineNotificationPairing(c *gin.Context) {
	settings, ok := h.lineSettings(c)
	if !ok {
		return
	}
	challenge, err := settings.StartNotificationPairing(c.Request.Context())
	if errors.Is(err, line.ErrRecipientManaged) {
		writeError(c, 409, "LINE_SERVER_MANAGED", "The recipient is managed by server configuration.")
		return
	}
	if err != nil {
		writeError(c, 503, "LINE_SETUP_UNAVAILABLE", "Configure the LINE channel token, secret, and recipient storage before pairing.")
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": challenge})
}
func (h *Handler) CancelLineNotificationPairing(c *gin.Context) {
	settings, ok := h.lineSettings(c)
	if !ok {
		return
	}
	settings.CancelNotificationPairing()
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"cancelled": true}})
}
func (h *Handler) TestLineNotification(c *gin.Context) {
	settings, ok := h.lineSettings(c)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	err := settings.TestNotification(ctx)
	if errors.Is(err, line.ErrRecipientMissing) {
		writeError(c, 409, "LINE_RECIPIENT_MISSING", "Link a staff LINE chat before testing delivery.")
		return
	}
	if err != nil {
		writeError(c, 502, "LINE_DELIVERY_FAILED", "LINE did not accept the test notification. Check the bot permissions, token and message quota.")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"accepted": true}})
}
