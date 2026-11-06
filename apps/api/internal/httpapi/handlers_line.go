package httpapi

import (
	"context"
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rbc/ev-station/apps/api/internal/line"
)

const maxLineWebhookBytes = 1024 * 1024

type lineWebhookRecorder interface {
	RecordWebhook(context.Context, []byte, string) error
}

func (h *Handler) LineWebhook(c *gin.Context) {
	if h.notifier == nil {
		writeError(c, http.StatusServiceUnavailable, "LINE_NOT_CONFIGURED", "LINE webhook is not configured.")
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, maxLineWebhookBytes))
	if err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_LINE_WEBHOOK", "Unable to read LINE webhook.")
		return
	}
	recorder, ok := h.notifier.(lineWebhookRecorder)
	if !ok {
		writeError(c, http.StatusServiceUnavailable, "LINE_NOT_CONFIGURED", "LINE webhook is not configured.")
		return
	}
	if err = recorder.RecordWebhook(c.Request.Context(), body, c.GetHeader("X-Line-Signature")); err != nil {
		if errors.Is(err, line.ErrInvalidSignature) {
			writeError(c, http.StatusUnauthorized, "INVALID_LINE_WEBHOOK", "LINE webhook signature is invalid.")
		} else if errors.Is(err, line.ErrInvalidWebhookPayload) {
			writeError(c, http.StatusBadRequest, "INVALID_LINE_WEBHOOK", "LINE webhook payload is invalid.")
		} else {
			// Do not report a transient persistence failure as a bad signature;
			// the unconsumed pairing can be retried by LINE webhook redelivery.
			writeError(c, http.StatusServiceUnavailable, "LINE_WEBHOOK_UNAVAILABLE", "LINE webhook processing is temporarily unavailable.")
		}
		return
	}
	c.Status(http.StatusOK)
}
