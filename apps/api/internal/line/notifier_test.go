package line

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/rbc/ev-station/apps/api/internal/domain"
)

type recipientStore struct {
	recipientID string
}

func (s *recipientStore) SetLineNotificationRecipient(_ context.Context, recipientID string) error {
	s.recipientID = recipientID
	return nil
}

func (s *recipientStore) GetLineNotificationRecipient(_ context.Context) (string, error) {
	return s.recipientID, nil
}

func TestNotifierPushesCustomerSubmission(t *testing.T) {
	var gotAuthorization string
	var got struct {
		To       string `json:"to"`
		Messages []struct {
			Text string `json:"text"`
		} `json:"messages"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/v2/bot/message/push" {
			writer.WriteHeader(http.StatusNotFound)
			return
		}
		gotAuthorization = request.Header.Get("Authorization")
		if err := json.NewDecoder(request.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		writer.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	notifier := NewNotifier("secret-token", "channel-secret", "C0123456789", server.URL, nil, server.Client())
	err := notifier.NotifyCustomerSubmission(context.Background(), domain.Site{ID: uuid.New(), Name: "พื้นที่ทดสอบ", ContactName: "ลูกค้าทดสอบ", ContactPhone: "0812345678", LandSize: 2, LandSizeUnit: "rai", GoogleMapsURL: "https://maps.google.com/example"})
	if err != nil {
		t.Fatalf("NotifyCustomerSubmission() error = %v", err)
	}
	if gotAuthorization != "Bearer secret-token" || got.To != "C0123456789" || len(got.Messages) != 1 {
		t.Fatalf("unexpected push request: authorization=%q to=%q messages=%d", gotAuthorization, got.To, len(got.Messages))
	}
	if got.Messages[0].Text == "" {
		t.Fatal("expected notification text")
	}
}

func TestNotifierSkipsWhenNotConfigured(t *testing.T) {
	notifier := NewNotifier("", "", "", "", nil, nil)
	if err := notifier.NotifyCustomerSubmission(context.Background(), domain.Site{}); err != nil {
		t.Fatalf("NotifyCustomerSubmission() error = %v, want nil", err)
	}
}

func TestNotifierRecordsVerifiedGroupWebhook(t *testing.T) {
	store := &recipientStore{}
	secret := "webhook-secret"
	body := []byte(`{"events":[{"type":"message","source":{"type":"group","groupId":"C1234567890"}}]}`)

	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	notifier := NewNotifier("", secret, "", "", store, nil)
	if err := notifier.RecordWebhook(context.Background(), body, signature); err != nil {
		t.Fatalf("RecordWebhook() error = %v", err)
	}
	if store.recipientID != "C1234567890" {
		t.Fatalf("stored recipient = %q, want group ID", store.recipientID)
	}
}
