package repository

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
)

func (p *Postgres) SetApprovedLineNotificationRecipient(ctx context.Context, recipient string) error {
	_, err := p.pool.Exec(ctx, `INSERT INTO line_notification_recipient(id,recipient_id,approved_at,updated_at) VALUES(true,$1,now(),now()) ON CONFLICT(id) DO UPDATE SET recipient_id=EXCLUDED.recipient_id,approved_at=EXCLUDED.approved_at,updated_at=EXCLUDED.updated_at`, recipient)
	return err
}
func (p *Postgres) GetApprovedLineNotificationRecipient(ctx context.Context) (string, error) {
	var recipient string
	err := p.pool.QueryRow(ctx, `SELECT recipient_id FROM line_notification_recipient WHERE id=true AND approved_at IS NOT NULL`).Scan(&recipient)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return recipient, err
}
func (m *Memory) SetApprovedLineNotificationRecipient(_ context.Context, recipient string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lineNotificationRecipient = recipient
	m.lineNotificationApproved = true
	return nil
}
func (m *Memory) GetApprovedLineNotificationRecipient(_ context.Context) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if !m.lineNotificationApproved {
		return "", nil
	}
	return m.lineNotificationRecipient, nil
}
