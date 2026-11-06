package httpapi

import (
	"bytes"
	"context"
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rbc/ev-station/apps/api/internal/domain"
	"github.com/rbc/ev-station/apps/api/internal/line"
	"github.com/rbc/ev-station/apps/api/internal/repository"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLineSettingsRequireOwnerEvenWithoutRouterAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	n := line.NewNotifier("synthetic-token", "synthetic-secret", "", "", repository.NewMemory(), nil)
	h := &Handler{notifier: n}
	for _, handler := range []gin.HandlerFunc{h.LineNotificationStatus, h.StartLineNotificationPairing, h.CancelLineNotificationPairing, h.TestLineNotification} {
		for _, role := range []domain.UserRole{"", domain.RoleCustomer, domain.RoleSales, domain.RoleAdmin} {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/settings/line-notifications", nil)
			if role != "" {
				c.Set("userId", uuid.NewString())
				c.Set("role", role)
			}
			handler(c)
			want := http.StatusForbidden
			if role == "" {
				want = http.StatusUnauthorized
			}
			if w.Code != want {
				t.Fatalf("role %s status %d want %d", role, w.Code, want)
			}
		}
	}
	status, _ := n.NotificationStatus(context.Background())
	if status.PairingExpiresAt != nil {
		t.Fatal("unauthorized request created a challenge")
	}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/settings/line-notifications/pairing", nil)
	c.Set("userId", uuid.NewString())
	c.Set("role", domain.RoleSuperAdmin)
	h.StartLineNotificationPairing(c)
	if w.Code != http.StatusCreated || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("owner cannot create a non-cacheable pairing challenge", w.Code)
	}
}

type failingLineWebhook struct{ err error }

func (n failingLineWebhook) NotifyCustomerSubmission(context.Context, domain.Site) error { return nil }
func (n failingLineWebhook) RecordWebhook(context.Context, []byte, string) error         { return n.err }

func TestLineWebhookSeparatesSignaturePayloadAndTemporaryFailures(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want int
	}{
		{line.ErrInvalidSignature, http.StatusUnauthorized},
		{line.ErrInvalidWebhookPayload, http.StatusBadRequest},
		{errors.New("synthetic storage unavailable"), http.StatusServiceUnavailable},
		{nil, http.StatusOK},
	} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/line/webhook", bytes.NewBufferString(`{"events":[]}`))
		h := &Handler{notifier: failingLineWebhook{err: tc.err}}
		h.LineWebhook(c)
		if w.Code != tc.want {
			t.Fatalf("webhook response %d want %d", w.Code, tc.want)
		}
	}
}
