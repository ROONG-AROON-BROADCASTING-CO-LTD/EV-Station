package line

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/rbc/ev-station/apps/api/internal/domain"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLIFFIdentityValidation(t *testing.T) {
	for _, tc := range []struct {
		name, response string
		status         int
		valid          bool
	}{
		{"valid", `{"sub":"Uverified","aud":"123"}`, 200, true},
		{"wrong-channel", `{"sub":"Uverified","aud":"456"}`, 200, false},
		{"missing-user", `{"aud":"123"}`, 200, false},
		{"expired-token", `{}`, 400, false},
		{"invalid-json", `bad`, 200, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/oauth2/v2.1/verify" || r.Method != "POST" {
					t.Error("wrong endpoint")
				}
				r.ParseForm()
				if r.Form.Get("id_token") != "test-id-token" || r.Form.Get("client_id") != "123" {
					t.Error("missing verification parameters")
				}
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.response)
			}))
			defer server.Close()
			n := NewNotifier("token", "secret", "", server.URL, nil, server.Client())
			user, err := n.VerifyIDToken(context.Background(), "test-id-token", "123")
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v error=%v", tc.valid, err)
			}
			if tc.valid && user != "Uverified" {
				t.Fatal("wrong identity")
			}
		})
	}
}

func TestLIFFConfirmationRecipientAndRetry(t *testing.T) {
	for _, status := range []int{200, 409, 429, 500} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			id := uuid.New()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body struct {
					To string `json:"to"`
					Messages []struct { Text string `json:"text"` } `json:"messages"`
				}
				json.NewDecoder(r.Body).Decode(&body)
				if body.To != "Ucustomer" || r.Header.Get("X-Line-Retry-Key") != id.String() || r.Header.Get("Authorization") != "Bearer token" {
					 t.Error("incorrect delivery identity or retry key")
				}
				if len(body.Messages) != 1 || !strings.Contains(body.Messages[0].Text, "เลขที่รายการ:") {
					t.Error("confirmation message did not include the reference number")
				}
				if status == 409 {
					w.Header().Set("X-Line-Accepted-Request-Id", "accepted")
				}
				w.WriteHeader(status)
			}))
			defer server.Close()
			n := NewNotifier("token", "secret", "Cinternal-group", server.URL, nil, server.Client())
			err := n.ConfirmSubmission(context.Background(), "Ucustomer", domain.Site{ID: id})
			if (err == nil) != (status == 200 || status == 409) {
				t.Fatalf("status=%d err=%v", status, err)
			}
		})
	}
}
