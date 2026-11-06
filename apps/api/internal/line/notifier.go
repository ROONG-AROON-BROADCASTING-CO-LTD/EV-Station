package line

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/rbc/ev-station/apps/api/internal/domain"
)

const defaultAPIBaseURL = "https://api.line.me"

// Notifier sends an internal alert through the Messaging API linked to the
// RBC EV Station LINE Official Account. The recipient is a LINE user, group,
// or multi-person-chat ID — never the public @account handle.
type Notifier struct {
	channelAccessToken string
	channelSecret      string
	recipientID        string
	recipients         interface {
		SetLineNotificationRecipient(context.Context, string) error
		GetLineNotificationRecipient(context.Context) (string, error)
	}
	apiBaseURL     string
	client         *http.Client
	setupMu        sync.Mutex
	pairingHash    [32]byte
	pairingExpires time.Time
	lastDelivery   *DeliveryStatus
	now            func() time.Time
}

func NewNotifier(channelAccessToken, channelSecret, recipientID, apiBaseURL string, recipients interface {
	SetLineNotificationRecipient(context.Context, string) error
	GetLineNotificationRecipient(context.Context) (string, error)
}, client *http.Client) *Notifier {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	if strings.TrimSpace(apiBaseURL) == "" {
		apiBaseURL = defaultAPIBaseURL
	}
	return &Notifier{
		channelAccessToken: strings.TrimSpace(channelAccessToken),
		channelSecret:      strings.TrimSpace(channelSecret),
		recipientID:        strings.TrimSpace(recipientID),
		recipients:         recipients,
		apiBaseURL:         strings.TrimRight(strings.TrimSpace(apiBaseURL), "/"),
		client:             client,
		now:                time.Now,
	}
}

func (n *Notifier) NotifyCustomerSubmission(ctx context.Context, site domain.Site) error {
	// An incomplete LINE configuration must not block a customer submission.
	if n == nil || n.channelAccessToken == "" {
		return nil
	}
	recipientID, err := n.notificationRecipient(ctx)
	if err != nil {
		return err
	}
	// Only explicit operator configuration or owner-approved pairing is valid.
	if recipientID == "" {
		return nil
	}
	return n.pushText(ctx, recipientID, customerSubmissionText(site))
}

func (n *Notifier) pushText(ctx context.Context, recipientID, text string) (err error) {
	defer func() { n.recordDelivery(err) }()
	payload := struct {
		To       string `json:"to"`
		Messages []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"messages"`
	}{To: recipientID}
	payload.Messages = append(payload.Messages, struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}{Type: "text", Text: text})

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal LINE push message: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.apiBaseURL+"/v2/bot/message/push", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create LINE push request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+n.channelAccessToken)
	req.Header.Set("Content-Type", "application/json")
	response, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("send LINE push message: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 2048))
		return fmt.Errorf("LINE push message returned HTTP %d", response.StatusCode)
	}
	return nil
}

// RecordWebhook verifies origin, not recipient authorization. Group membership
// events must never modify where customer information is sent.
func (n *Notifier) RecordWebhook(ctx context.Context, body []byte, signature string) error {
	if n == nil || n.channelSecret == "" {
		return fmt.Errorf("LINE webhook is not configured")
	}
	mac := hmac.New(sha256.New, []byte(n.channelSecret))
	_, _ = mac.Write(body)
	expected := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(signature), []byte(expected)) {
		return ErrInvalidSignature
	}
	var webhook struct {
		Events []pairingEvent `json:"events"`
	}
	if err := json.Unmarshal(body, &webhook); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidWebhookPayload, err)
	}
	for _, event := range webhook.Events {
		if err := n.acceptPairing(ctx, event); err != nil {
			return err
		}
	}
	return nil
}

var ErrInvalidSignature = fmt.Errorf("invalid LINE webhook signature")
var ErrInvalidWebhookPayload = fmt.Errorf("invalid LINE webhook payload")

func customerSubmissionText(site domain.Site) string {
	location := site.Name
	if location == "" {
		location = "ไม่ระบุชื่อพื้นที่"
	}
	lines := []string{
		"มีข้อมูลพื้นที่ใหม่จากลูกค้า",
		"โครงการ/สถานที่: " + location,
	}
	if site.ContactName != "" {
		lines = append(lines, "ผู้ติดต่อ: "+site.ContactName)
	}
	if site.ContactPhone != "" {
		lines = append(lines, "โทร: "+site.ContactPhone)
	}
	lines = append(lines, fmt.Sprintf("ขนาดพื้นที่: %g %s", site.LandSize, landSizeUnit(site.LandSizeUnit)))
	if site.GoogleMapsURL != "" {
		lines = append(lines, "Google Maps: "+site.GoogleMapsURL)
	}
	lines = append(lines, "กรุณาเปิดระบบ RBC EV Station เพื่อตรวจสอบข้อมูล")
	return strings.Join(lines, "\n")
}

func landSizeUnit(unit string) string {
	switch unit {
	case "rai":
		return "ไร่"
	case "ngan":
		return "งาน"
	case "sqwah":
		return "ตารางวา"
	default:
		return "ตารางเมตร"
	}
}
