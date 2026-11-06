package line

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"
)

var ErrSetupUnavailable = errors.New("LINE notification setup is unavailable")
var ErrRecipientManaged = errors.New("LINE recipient is managed by server configuration")
var ErrRecipientMissing = errors.New("LINE notification recipient is not linked")

type approvedRecipientStore interface {
	SetApprovedLineNotificationRecipient(context.Context, string) error
	GetApprovedLineNotificationRecipient(context.Context) (string, error)
}

type DeliveryStatus struct {
	AttemptedAt time.Time `json:"attemptedAt"`
	Succeeded   bool      `json:"succeeded"`
}

type NotificationStatus struct {
	TokenConfigured     bool            `json:"tokenConfigured"`
	WebhookConfigured   bool            `json:"webhookConfigured"`
	RecipientConfigured bool            `json:"recipientConfigured"`
	RecipientSource     string          `json:"recipientSource"`
	RecipientType       string          `json:"recipientType"`
	MaskedRecipient     string          `json:"maskedRecipient"`
	CanPair             bool            `json:"canPair"`
	PairingExpiresAt    *time.Time      `json:"pairingExpiresAt,omitempty"`
	LastDelivery        *DeliveryStatus `json:"lastDelivery,omitempty"`
}

type PairingChallenge struct {
	Command   string    `json:"command"`
	ExpiresAt time.Time `json:"expiresAt"`
}

func (n *Notifier) notificationRecipient(ctx context.Context) (string, error) {
	if n.recipientID != "" {
		return n.recipientID, nil
	}
	if store, ok := n.recipients.(approvedRecipientStore); ok {
		return store.GetApprovedLineNotificationRecipient(ctx)
	}
	// Legacy webhook discovery never constitutes approval.
	return "", nil
}

func (n *Notifier) NotificationStatus(ctx context.Context) (NotificationStatus, error) {
	if n == nil {
		return NotificationStatus{}, nil
	}
	recipient, err := n.notificationRecipient(ctx)
	if err != nil {
		return NotificationStatus{}, err
	}
	_, hasStore := n.recipients.(approvedRecipientStore)
	status := NotificationStatus{TokenConfigured: n.channelAccessToken != "", WebhookConfigured: n.channelSecret != "", RecipientConfigured: recipient != "", CanPair: hasStore && n.channelAccessToken != "" && n.channelSecret != "" && n.recipientID == ""}
	if recipient != "" {
		status.RecipientSource = "paired"
		if n.recipientID != "" {
			status.RecipientSource = "environment"
		}
		status.RecipientType = recipientType(recipient)
		if len(recipient) > 8 {
			status.MaskedRecipient = recipient[:4] + "…" + recipient[len(recipient)-4:]
		} else {
			status.MaskedRecipient = "configured"
		}
	}
	n.setupMu.Lock()
	defer n.setupMu.Unlock()
	if n.now().Before(n.pairingExpires) {
		expires := n.pairingExpires
		status.PairingExpiresAt = &expires
	}
	if n.lastDelivery != nil {
		delivery := *n.lastDelivery
		status.LastDelivery = &delivery
	}
	return status, nil
}

// Only authenticated owner endpoints call this. The secret is shown once,
// never logged, and proves approval when sent in the intended LINE chat.
func (n *Notifier) StartNotificationPairing(ctx context.Context) (PairingChallenge, error) {
	if n == nil {
		return PairingChallenge{}, ErrSetupUnavailable
	}
	if n.recipientID != "" {
		return PairingChallenge{}, ErrRecipientManaged
	}
	if _, ok := n.recipients.(approvedRecipientStore); !ok || n.channelAccessToken == "" || n.channelSecret == "" {
		return PairingChallenge{}, ErrSetupUnavailable
	}
	if err := ctx.Err(); err != nil {
		return PairingChallenge{}, err
	}
	var token [20]byte
	if _, err := rand.Read(token[:]); err != nil {
		return PairingChallenge{}, err
	}
	command := "RBC-LINK " + hex.EncodeToString(token[:])
	n.setupMu.Lock()
	defer n.setupMu.Unlock()
	n.pairingHash = sha256.Sum256([]byte(command))
	n.pairingExpires = n.now().Add(10 * time.Minute)
	return PairingChallenge{Command: command, ExpiresAt: n.pairingExpires}, nil
}

func (n *Notifier) CancelNotificationPairing() {
	n.setupMu.Lock()
	defer n.setupMu.Unlock()
	n.pairingExpires = time.Time{}
	n.pairingHash = [32]byte{}
}

type pairingEvent struct {
	Type   string `json:"type"`
	Source struct {
		Type    string `json:"type"`
		UserID  string `json:"userId"`
		GroupID string `json:"groupId"`
		RoomID  string `json:"roomId"`
	} `json:"source"`
	Message struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"message"`
}

func (n *Notifier) acceptPairing(ctx context.Context, event pairingEvent) error {
	if n.recipientID != "" || event.Type != "message" || event.Message.Type != "text" {
		return nil
	}
	command := strings.TrimSpace(event.Message.Text)
	if !strings.HasPrefix(command, "RBC-LINK ") || len(command) != 49 {
		return nil
	}
	recipient := ""
	switch event.Source.Type {
	case "user":
		recipient = event.Source.UserID
	case "group":
		recipient = event.Source.GroupID
	case "room":
		recipient = event.Source.RoomID
	}
	if !validRecipient(recipient, event.Source.Type) {
		return nil
	}
	n.setupMu.Lock()
	defer n.setupMu.Unlock()
	if !n.now().Before(n.pairingExpires) {
		return nil
	}
	hash := sha256.Sum256([]byte(command))
	if !hmac.Equal(hash[:], n.pairingHash[:]) {
		return nil
	}
	store, ok := n.recipients.(approvedRecipientStore)
	if !ok {
		return ErrSetupUnavailable
	}
	// Bound storage waits while holding the single-use lock. A stalled pool or
	// row lock must not indefinitely block status, cancellation or notifications.
	storageCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	err := store.SetApprovedLineNotificationRecipient(storageCtx, recipient)
	cancel()
	if err != nil {
		return err
	}
	// Consume atomically after persistence. Duplicate/replayed events are harmless.
	n.pairingHash = [32]byte{}
	n.pairingExpires = time.Time{}
	return nil
}

func validRecipient(id, kind string) bool {
	if len(id) != 33 || recipientType(id) != kind {
		return false
	}
	_, err := hex.DecodeString(id[1:])
	return err == nil
}
func recipientType(id string) string {
	if id == "" {
		return ""
	}
	switch id[0] {
	case 'U':
		return "user"
	case 'C':
		return "group"
	case 'R':
		return "room"
	}
	return "unknown"
}

func (n *Notifier) TestNotification(ctx context.Context) error {
	if n == nil || n.channelAccessToken == "" {
		return ErrSetupUnavailable
	}
	recipient, err := n.notificationRecipient(ctx)
	if err != nil {
		return err
	}
	if recipient == "" {
		return ErrRecipientMissing
	}
	return n.pushText(ctx, recipient, "ทดสอบการแจ้งเตือนเจ้าหน้าที่จาก RBC EV Station — เชื่อมต่อปลายทางนี้แล้ว (ข้อความทดสอบ ไม่มีข้อมูลลูกค้า)")
}
func (n *Notifier) recordDelivery(err error) {
	n.setupMu.Lock()
	defer n.setupMu.Unlock()
	n.lastDelivery = &DeliveryStatus{AttemptedAt: n.now().UTC(), Succeeded: err == nil}
}
