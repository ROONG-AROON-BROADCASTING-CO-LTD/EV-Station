package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/rbc/ev-station/apps/api/internal/domain"
)

// Kept separate from analysis evidence: contract bytes never enter AI prompts.
type DocumentStore interface {
	ListDocuments(context.Context) ([]domain.ProjectDocument, error)
	GetDocument(context.Context, uuid.UUID) (domain.ProjectDocument, error)
	SaveDocument(context.Context, domain.ProjectDocument, uuid.UUID, *domain.DocumentVersion) (domain.ProjectDocument, error)
	DocumentVersions(context.Context, uuid.UUID) ([]domain.DocumentVersion, error)
	DocumentFile(context.Context, uuid.UUID, int) (domain.DocumentVersion, error)
	DocumentEvents(context.Context, uuid.UUID) ([]domain.DocumentEvent, error)
	RecordDocumentRead(context.Context, uuid.UUID, uuid.UUID, string, int) error
}

func scanDocument(row pgx.Row) (domain.ProjectDocument, error) {
	var d domain.ProjectDocument
	var data []byte
	err := row.Scan(&d.ID, &d.SiteID, &d.SiteName, &data, &d.LatestVersion, &d.CreatedAt, &d.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return d, ErrNotFound
	}
	if err != nil {
		return d, err
	}
	err = json.Unmarshal(data, &d.Metadata)
	return d, err
}

const documentColumns = "d.id,d.site_id,s.name,d.metadata,d.latest_version,d.created_at,d.updated_at"

func (p *Postgres) ListDocuments(ctx context.Context) ([]domain.ProjectDocument, error) {
	rows, err := p.pool.Query(ctx, "SELECT "+documentColumns+" FROM project_documents d JOIN sites s ON s.id=d.site_id ORDER BY d.updated_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []domain.ProjectDocument{}
	for rows.Next() {
		d, e := scanDocument(rows)
		if e != nil {
			return nil, e
		}
		result = append(result, d)
	}
	return result, rows.Err()
}
func (p *Postgres) GetDocument(ctx context.Context, id uuid.UUID) (domain.ProjectDocument, error) {
	return scanDocument(p.pool.QueryRow(ctx, "SELECT "+documentColumns+" FROM project_documents d JOIN sites s ON s.id=d.site_id WHERE d.id=$1", id))
}
func (p *Postgres) SaveDocument(ctx context.Context, d domain.ProjectDocument, actor uuid.UUID, file *domain.DocumentVersion) (domain.ProjectDocument, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return d, err
	}
	defer tx.Rollback(ctx)
	// Serialize both version numbering and the project-wide 100 MiB quota.
	var site uuid.UUID
	if err = tx.QueryRow(ctx, "SELECT id FROM sites WHERE id=$1 FOR UPDATE", d.SiteID).Scan(&site); err != nil {
		return d, err
	}
	data, err := json.Marshal(d.Metadata)
	if err != nil {
		return d, err
	}
	var previous []byte
	var currentVersion int
	err = tx.QueryRow(ctx, "SELECT metadata,latest_version FROM project_documents WHERE id=$1 AND site_id=$2 FOR UPDATE", d.ID, d.SiteID).Scan(&previous, &currentVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		if file == nil {
			return d, ErrNotFound
		}
		_, err = tx.Exec(ctx, "INSERT INTO project_documents(id,site_id,metadata) VALUES($1,$2,$3)", d.ID, d.SiteID, data)
	}
	if err != nil {
		return d, err
	}
	if file == nil && currentVersion != d.LatestVersion {
		return d, ErrDocumentConflict
	}
	action := "metadata_updated"
	if file != nil {
		if len(previous) > 0 {
			if err = json.Unmarshal(previous, &d.Metadata); err != nil {
				return d, err
			}
		}
		hash := sha256.Sum256(file.Content)
		file.SHA256 = hex.EncodeToString(hash[:])
		var latestHash string
		err = tx.QueryRow(ctx, "SELECT sha256 FROM project_document_versions WHERE document_id=$1 ORDER BY version DESC LIMIT 1", d.ID).Scan(&latestHash)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return d, err
		}
		if latestHash == file.SHA256 {
			return p.GetDocument(ctx, d.ID)
		}
		var total int64
		if err = tx.QueryRow(ctx, "SELECT COALESCE(sum(octet_length(v.content)),0) FROM project_document_versions v JOIN project_documents d ON d.id=v.document_id WHERE d.site_id=$1", d.SiteID).Scan(&total); err != nil {
			return d, err
		}
		if total+int64(len(file.Content)) > 100<<20 {
			return d, ErrSiteEvidenceLimit
		}
		if err = tx.QueryRow(ctx, "UPDATE project_documents SET latest_version=latest_version+1 WHERE id=$1 RETURNING latest_version", d.ID).Scan(&file.Version); err != nil {
			return d, err
		}
		if _, err = tx.Exec(ctx, "INSERT INTO project_document_versions(document_id,version,filename,content,sha256,actor_id) VALUES($1,$2,$3,$4,$5,$6)", d.ID, file.Version, file.Filename, file.Content, file.SHA256, actor); err != nil {
			return d, err
		}
		// A new version always requires fresh customer-release approval.
		d.Metadata.Shared = false
		d.Metadata.SignaturesConfirmed = false
		d.Metadata.Status = "review"
		data, _ = json.Marshal(d.Metadata)
		action = "version_uploaded"
	}
	if _, err = tx.Exec(ctx, "UPDATE project_documents SET metadata=$2,updated_at=now() WHERE id=$1", d.ID, data); err != nil {
		return d, err
	}
	detail, _ := json.Marshal(map[string]any{"before": json.RawMessage(previous), "after": json.RawMessage(data)})
	if _, err = tx.Exec(ctx, "INSERT INTO project_document_events(document_id,actor_id,action,detail) VALUES($1,$2,$3,$4)", d.ID, actor, action, detail); err != nil {
		return d, err
	}
	if err = tx.Commit(ctx); err != nil {
		return d, err
	}
	return p.GetDocument(ctx, d.ID)
}
func (p *Postgres) DocumentVersions(ctx context.Context, id uuid.UUID) ([]domain.DocumentVersion, error) {
	rows, err := p.pool.Query(ctx, "SELECT version,filename,octet_length(content),sha256,actor_id,created_at FROM project_document_versions WHERE document_id=$1 ORDER BY version DESC", id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []domain.DocumentVersion{}
	for rows.Next() {
		var v domain.DocumentVersion
		if err = rows.Scan(&v.Version, &v.Filename, &v.Size, &v.SHA256, &v.ActorID, &v.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, v)
	}
	return result, rows.Err()
}
func (p *Postgres) DocumentFile(ctx context.Context, id uuid.UUID, version int) (domain.DocumentVersion, error) {
	var v domain.DocumentVersion
	err := p.pool.QueryRow(ctx, "SELECT version,filename,content,sha256,actor_id,created_at FROM project_document_versions WHERE document_id=$1 AND version=$2", id, version).Scan(&v.Version, &v.Filename, &v.Content, &v.SHA256, &v.ActorID, &v.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		err = ErrNotFound
	}
	return v, err
}
func (p *Postgres) DocumentEvents(ctx context.Context, id uuid.UUID) ([]domain.DocumentEvent, error) {
	rows, err := p.pool.Query(ctx, "SELECT e.id,e.actor_id,COALESCE(u.display_name,e.actor_id::text),e.action,e.detail,e.created_at FROM project_document_events e LEFT JOIN users u ON u.id=e.actor_id WHERE e.document_id=$1 ORDER BY e.id DESC LIMIT 200", id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []domain.DocumentEvent{}
	for rows.Next() {
		var e domain.DocumentEvent
		if err = rows.Scan(&e.ID, &e.ActorID, &e.ActorName, &e.Action, &e.Detail, &e.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, e)
	}
	return result, rows.Err()
}
func (p *Postgres) RecordDocumentRead(ctx context.Context, id, actor uuid.UUID, action string, version int) error {
	detail, _ := json.Marshal(map[string]int{"version": version})
	_, err := p.pool.Exec(ctx, "INSERT INTO project_document_events(document_id,actor_id,action,detail) VALUES($1,$2,$3,$4)", id, actor, action, detail)
	return err
}
