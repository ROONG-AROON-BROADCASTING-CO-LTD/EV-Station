package line

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"github.com/rbc/ev-station/apps/api/internal/domain"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWebhookCannotChangeApprovedRecipient(t *testing.T) {
	store := &recipientStore{recipientID: "C-approved"}
	n := NewNotifier("synthetic-token", "synthetic-secret", "C-approved", "", store, nil)
	for _, group := range []string{"C-unapproved", "C-approved"} {
		body := []byte(`{"events":[{"source":{"type":"group","groupId":"` + group + `"}}]}`)
		mac := hmac.New(sha256.New, []byte("synthetic-secret"))
		mac.Write(body)
		if err := n.RecordWebhook(context.Background(), body, base64.StdEncoding.EncodeToString(mac.Sum(nil))); err != nil {
			t.Fatal(err)
		}
		if store.recipientID != "C-approved" {
			t.Fatal("webhook changed recipient", store.recipientID)
		}
	}
}

func TestConfiguredRecipientOverridesLegacyWebhookStorage(t *testing.T) {
	store := &recipientStore{recipientID: "C-unapproved"}
	got := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			To string `json:"to"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		got = body.To
		w.WriteHeader(200)
	}))
	defer server.Close()
	n := NewNotifier("synthetic-token", "synthetic-secret", "C-approved", server.URL, store, server.Client())
	if err := n.NotifyCustomerSubmission(context.Background(), domain.Site{Name: "Synthetic"}); err != nil {
		t.Fatal(err)
	}
	if got != "C-approved" {
		t.Fatal("wrong recipient", got)
	}
	n.recipientID = ""
	got = ""
	if err := n.NotifyCustomerSubmission(context.Background(), domain.Site{}); err != nil {
		t.Fatal(err)
	}
	if got != "" {
		t.Fatal("sent without approved recipient")
	}
}
