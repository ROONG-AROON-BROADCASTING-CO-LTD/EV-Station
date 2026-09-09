package repository

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/rbc/ev-station/apps/api/internal/domain"
)

// SaveLineSubmission saves the identity binding and site in one transaction.
// A repeated request returns the original site, including after a lost response.
func (p *Postgres) SaveLineSubmission(ctx context.Context, user string, requestID uuid.UUID, s domain.Site) (domain.Site, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return s, err
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, user+requestID.String())
	if err != nil {
		return s, err
	}
	var existing uuid.UUID
	err = tx.QueryRow(ctx, `SELECT site_id FROM line_submissions WHERE line_user_id=$1 AND request_id=$2`, user, requestID).Scan(&existing)
	if err == nil {
		return p.GetSite(ctx, existing)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return s, err
	}
	s, err = scanSite(tx.QueryRow(ctx, `INSERT INTO sites (id,name,contact_name,contact_phone,address,latitude,longitude,location,land_size,land_size_unit,google_maps_url,notes,internet_available,frontage_meters,input_status,created_at,updated_at)
 VALUES ($1,$2,$3,$4,$5,$6,$7,CASE WHEN $6::double precision IS NULL THEN NULL ELSE ST_SetSRID(ST_MakePoint($7,$6),4326)::geography END,$8,$9,$10,$11,$12,$13,$14,$15,$15) RETURNING `+siteColumns,
		s.ID, s.Name, s.ContactName, s.ContactPhone, s.Address, s.Latitude, s.Longitude, s.LandSize, s.LandSizeUnit, s.GoogleMapsURL, s.Notes, s.InternetAvailable, s.FrontageMeters, s.InputStatus, s.CreatedAt))
	if err != nil {
		return s, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO line_submissions(site_id,line_user_id,request_id) VALUES ($1,$2,$3)`, s.ID, user, requestID)
	if err != nil {
		return s, err
	}
	return s, tx.Commit(ctx)
}

func (p *Postgres) OwnsLineSite(ctx context.Context, user string, siteID uuid.UUID) (bool, error) {
	var owns bool
	err := p.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM line_submissions WHERE site_id=$1 AND line_user_id=$2)`, siteID, user).Scan(&owns)
	return owns, err
}

// ListLineSites returns only the sites submitted by one verified LINE user.
func (p *Postgres) ListLineSites(ctx context.Context, user string) ([]domain.Site, error) {
	rows, err := p.pool.Query(ctx, `SELECT `+siteColumns+` FROM sites s
		JOIN line_submissions ls ON ls.site_id = s.id
		WHERE ls.line_user_id = $1
		ORDER BY s.updated_at DESC`, user)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sites := []domain.Site{}
	for rows.Next() {
		site, err := scanSite(rows)
		if err != nil {
			return nil, err
		}
		sites = append(sites, site)
	}
	return sites, rows.Err()
}
