package line

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/rbc/ev-station/apps/api/internal/domain"
	"github.com/rbc/ev-station/apps/api/internal/repository"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const testGroup = "C0123456789abcdef0123456789abcdef"
const anotherGroup = "Cfedcba9876543210fedcba9876543210"
const testSecret = "synthetic-channel-secret"

func pairingBody(command, group string) []byte {
	body, _ := json.Marshal(map[string]any{"events": []any{map[string]any{"type": "message", "source": map[string]string{"type": "group", "groupId": group}, "message": map[string]string{"type": "text", "text": command}}}})
	return body
}
func signPairing(body []byte) string {
	mac := hmac.New(sha256.New, []byte(testSecret))
	_, _ = mac.Write(body)
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func TestOwnerPairingRequiresSignedMatchingUnexpiredSingleUseCommand(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewMemory()
	_ = repo.SetLineNotificationRecipient(ctx, anotherGroup)
	n := NewNotifier("synthetic-token", testSecret, "", "", repo, nil)
	now := time.Date(2026, 11, 5, 1, 0, 0, 0, time.UTC)
	n.now = func() time.Time { return now }
	challenge, err := n.StartNotificationPairing(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(challenge.Command) != 49 || !challenge.ExpiresAt.Equal(now.Add(10*time.Minute)) {
		t.Fatal("invalid challenge lifetime or entropy")
	}
	body := pairingBody(challenge.Command, testGroup)
	if n.RecordWebhook(ctx, body, "forged-signature") == nil {
		t.Fatal("accepted forged webhook")
	}
	wrong := pairingBody("RBC-LINK "+strings.Repeat("0", 40), testGroup)
	if err := n.RecordWebhook(ctx, wrong, signPairing(wrong)); err != nil {
		t.Fatal(err)
	}
	status, _ := n.NotificationStatus(ctx)
	if status.RecipientConfigured {
		t.Fatal("legacy or wrong command authorized recipient")
	}
	now = now.Add(11 * time.Minute)
	if err := n.RecordWebhook(ctx, body, signPairing(body)); err != nil {
		t.Fatal(err)
	}
	status, _ = n.NotificationStatus(ctx)
	if status.RecipientConfigured {
		t.Fatal("expired command authorized recipient")
	}
	challenge, err = n.StartNotificationPairing(ctx)
	if err != nil {
		t.Fatal(err)
	}
	body = pairingBody(challenge.Command, testGroup)
	if err := n.RecordWebhook(ctx, body, signPairing(body)); err != nil {
		t.Fatal(err)
	}
	status, _ = n.NotificationStatus(ctx)
	if !status.RecipientConfigured || status.RecipientSource != "paired" || status.RecipientType != "group" || status.PairingExpiresAt != nil {
		t.Fatalf("unexpected pairing state: %+v", status)
	}
	if strings.Contains(status.MaskedRecipient, testGroup) {
		t.Fatal("status exposed full recipient")
	}
	for _, group := range []string{testGroup, anotherGroup} {
		body = pairingBody(challenge.Command, group)
		if err := n.RecordWebhook(ctx, body, signPairing(body)); err != nil {
			t.Fatal(err)
		}
	}
	stored, _ := repo.GetApprovedLineNotificationRecipient(ctx)
	if stored != testGroup {
		t.Fatal("replay changed the recipient")
	}
	// A new service process can load only the explicitly approved recipient.
	restarted := NewNotifier("synthetic-token", testSecret, "", "", repo, nil)
	status, _ = restarted.NotificationStatus(ctx)
	if !status.RecipientConfigured {
		t.Fatal("approved recipient lost after restart")
	}
}

func TestPairingReplacementCancellationAndEnvironmentPrecedence(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewMemory()
	n := NewNotifier("token", testSecret, "", "", repo, nil)
	old, _ := n.StartNotificationPairing(ctx)
	fresh, _ := n.StartNotificationPairing(ctx)
	for _, command := range []string{old.Command, fresh.Command} {
		if command == fresh.Command {
			n.CancelNotificationPairing()
		}
		body := pairingBody(command, testGroup)
		if err := n.RecordWebhook(ctx, body, signPairing(body)); err != nil {
			t.Fatal(err)
		}
	}
	status, _ := n.NotificationStatus(ctx)
	if status.RecipientConfigured {
		t.Fatal("replaced or cancelled command authorized recipient")
	}
	n.recipientID = anotherGroup
	if _, err := n.StartNotificationPairing(ctx); !errors.Is(err, ErrRecipientManaged) {
		t.Fatal("server-managed recipient allowed pairing")
	}
	_ = repo.SetApprovedLineNotificationRecipient(ctx, testGroup)
	got, _ := n.notificationRecipient(ctx)
	if got != anotherGroup {
		t.Fatal("paired value overrode server configuration")
	}
}

func TestPairedDeliveryAndSafeTestMessage(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewMemory()
	var messages []string
	var targets []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			To       string `json:"to"`
			Messages []struct {
				Text string `json:"text"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
			return
		}
		if r.URL.Path != "/v2/bot/message/push" {
			t.Error("unexpected endpoint")
		}
		targets = append(targets, payload.To)
		messages = append(messages, payload.Messages[0].Text)
		w.WriteHeader(200)
	}))
	defer server.Close()
	n := NewNotifier("token", testSecret, "", server.URL, repo, server.Client())
	if !errors.Is(n.TestNotification(ctx), ErrRecipientMissing) {
		t.Fatal("test must require explicit recipient")
	}
	if err := n.NotifyCustomerSubmission(ctx, domain.Site{Name: "not yet authorized"}); err != nil {
		t.Fatal(err)
	}
	if len(messages) != 0 {
		t.Fatal("sent without explicit recipient")
	}
	_ = repo.SetApprovedLineNotificationRecipient(ctx, testGroup)
	if err := n.TestNotification(ctx); err != nil {
		t.Fatal(err)
	}
	if err := n.NotifyCustomerSubmission(ctx, domain.Site{Name: "Synthetic project"}); err != nil {
		t.Fatal(err)
	}
	if len(messages) != 2 || targets[0] != testGroup || !strings.Contains(messages[0], "ไม่มีข้อมูลลูกค้า") || !strings.Contains(messages[1], "Synthetic project") {
		t.Fatal("incorrect recipient or payload")
	}
	status, _ := n.NotificationStatus(ctx)
	if status.LastDelivery == nil || !status.LastDelivery.Succeeded {
		t.Fatal("successful delivery not recorded")
	}
}

func TestDeliveryFailureAndMissingChannelConfiguration(t *testing.T) {
	ctx := context.Background()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte("do-not-log-upstream-payload"))
	}))
	defer server.Close()
	n := NewNotifier("token", testSecret, testGroup, server.URL, nil, server.Client())
	err := n.TestNotification(ctx)
	if err == nil || strings.Contains(err.Error(), "do-not-log") {
		t.Fatal("failed request accepted or leaked upstream payload")
	}
	status, _ := n.NotificationStatus(ctx)
	if status.LastDelivery == nil || status.LastDelivery.Succeeded {
		t.Fatal("delivery failure missing")
	}
	n = NewNotifier("", testSecret, "", "", repository.NewMemory(), nil)
	if _, err = n.StartNotificationPairing(ctx); !errors.Is(err, ErrSetupUnavailable) {
		t.Fatal("pairing without token allowed")
	}
}

type failingApprovalStore struct {
	*repository.Memory
	fail bool
}

func (s *failingApprovalStore) SetApprovedLineNotificationRecipient(ctx context.Context, recipient string) error {
	if s.fail {
		return errors.New("synthetic storage failure")
	}
	return s.Memory.SetApprovedLineNotificationRecipient(ctx, recipient)
}

func TestPairingRetriesPersistenceFailureWithoutReusingSuccessfulCode(t *testing.T) {
	ctx := context.Background()
	store := &failingApprovalStore{Memory: repository.NewMemory(), fail: true}
	n := NewNotifier("token", testSecret, "", "", store, nil)
	challenge, _ := n.StartNotificationPairing(ctx)
	badSource := pairingBody(challenge.Command, "C-invalid")
	if err := n.RecordWebhook(ctx, badSource, signPairing(badSource)); err != nil {
		t.Fatal(err)
	}
	body := pairingBody(challenge.Command, testGroup)
	if err := n.RecordWebhook(ctx, body, signPairing(body)); err == nil {
		t.Fatal("persistence failure was hidden")
	}
	status, _ := n.NotificationStatus(ctx)
	if status.PairingExpiresAt == nil || status.RecipientConfigured {
		t.Fatal("failed persistence consumed code or approved recipient")
	}
	store.fail = false
	if err := n.RecordWebhook(ctx, body, signPairing(body)); err != nil {
		t.Fatal(err)
	}
	status, _ = n.NotificationStatus(ctx)
	if status.PairingExpiresAt != nil || !status.RecipientConfigured {
		t.Fatal("retry did not persist and consume pairing")
	}
}

type blockingApprovalStore struct{ *repository.Memory }

func (s *blockingApprovalStore) SetApprovedLineNotificationRecipient(ctx context.Context, _ string) error {
	<-ctx.Done()
	return ctx.Err()
}

func TestPairingStorageWaitIsBoundedAndDoesNotConsumeCommand(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	store := &blockingApprovalStore{Memory: repository.NewMemory()}
	n := NewNotifier("token", testSecret, "", "", store, nil)
	challenge, _ := n.StartNotificationPairing(ctx)
	body := pairingBody(challenge.Command, testGroup)
	done := make(chan error, 1)
	go func() { done <- n.RecordWebhook(ctx, body, signPairing(body)) }()
	select {
	case err := <-done:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("expected bounded storage timeout, got %v", err)
		}
	case <-time.After(4 * time.Second):
		t.Fatal("pairing storage wait was not bounded")
	}
	status, err := n.NotificationStatus(ctx)
	if err != nil || status.PairingExpiresAt == nil || status.RecipientConfigured {
		t.Fatal("storage timeout consumed command or retained setup lock")
	}
	n.CancelNotificationPairing()
}
