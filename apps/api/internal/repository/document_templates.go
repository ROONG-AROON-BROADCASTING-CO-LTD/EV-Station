package repository

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/rbc/ev-station/apps/api/internal/domain"
)

func (p *Postgres) ListDocumentTemplates(ctx context.Context) ([]domain.DocumentTemplate, error) {
	rows, err := p.pool.Query(ctx, "SELECT id,title,filename,octet_length(content),sha256,created_at FROM document_templates ORDER BY created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []domain.DocumentTemplate{}
	for rows.Next() {
		var d domain.DocumentTemplate
		if err = rows.Scan(&d.ID, &d.Title, &d.Filename, &d.Size, &d.SHA256, &d.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, d)
	}
	return result, rows.Err()
}
func (p *Postgres) SaveDocumentTemplate(ctx context.Context, d domain.DocumentTemplate, actor uuid.UUID) (domain.DocumentTemplate, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return d, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(260016)"); err != nil {
		return d, err
	}
	var total int64
	if err = tx.QueryRow(ctx, "SELECT COALESCE(sum(octet_length(content)),0) FROM document_templates WHERE id<>$1", d.ID).Scan(&total); err != nil {
		return d, err
	}
	if total+int64(len(d.Content)) > 100<<20 {
		return d, ErrSiteEvidenceLimit
	}
	var created bool
	err = tx.QueryRow(ctx, "INSERT INTO document_templates(id,title,filename,content,sha256,uploaded_by) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(id) DO NOTHING RETURNING true", d.ID, d.Title, d.Filename, d.Content, d.SHA256, actor).Scan(&created)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return d, err
	}
	if created {
		if _, err = tx.Exec(ctx, "INSERT INTO document_template_events(template_id,actor_id,action) VALUES($1,$2,'uploaded')", d.ID, actor); err != nil {
			return d, err
		}
	}
	err = tx.QueryRow(ctx, "SELECT title,filename,octet_length(content),sha256,created_at FROM document_templates WHERE id=$1", d.ID).Scan(&d.Title, &d.Filename, &d.Size, &d.SHA256, &d.CreatedAt)
	if err != nil {
		return d, err
	}
	return d, tx.Commit(ctx)
}
func (p *Postgres) ReadDocumentTemplate(ctx context.Context, id, actor uuid.UUID, action string) (domain.DocumentTemplate, error) {
	var d domain.DocumentTemplate
	d.ID = id
	err := p.pool.QueryRow(ctx, "SELECT title,filename,content,sha256,created_at FROM document_templates WHERE id=$1", id).Scan(&d.Title, &d.Filename, &d.Content, &d.SHA256, &d.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return d, ErrNotFound
	}
	if err != nil {
		return d, err
	}
	_, err = p.pool.Exec(ctx, "INSERT INTO document_template_events(template_id,actor_id,action) VALUES($1,$2,$3)", id, actor, action)
	return d, err
}
