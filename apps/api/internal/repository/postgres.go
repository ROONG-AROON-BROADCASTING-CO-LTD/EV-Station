package repository

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rbc/ev-station/apps/api/internal/domain"
)

type Postgres struct{ pool *pgxpool.Pool }

func (p *Postgres) CreateUser(ctx context.Context, user domain.User, passwordHash string) (domain.User, error) {
	err := p.pool.QueryRow(ctx, `INSERT INTO users (id,email,password_hash,display_name,role,is_active,created_at,updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$7) RETURNING id,email,display_name,role,is_active,created_at`, user.ID, user.Email, passwordHash, user.DisplayName, user.Role, user.IsActive, user.CreatedAt).Scan(&user.ID, &user.Email, &user.DisplayName, &user.Role, &user.IsActive, &user.CreatedAt)
	return user, err
}
func (p *Postgres) GetUserByEmail(ctx context.Context, email string) (domain.User, string, error) {
	var user domain.User
	var hash string
	err := p.pool.QueryRow(ctx, `SELECT id,email,password_hash,display_name,role,is_active,created_at FROM users WHERE email=$1`, email).Scan(&user.ID, &user.Email, &hash, &user.DisplayName, &user.Role, &user.IsActive, &user.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, "", ErrNotFound
	}
	return user, hash, err
}
func (p *Postgres) ListUsers(ctx context.Context) ([]domain.User, error) {
	rows, err := p.pool.Query(ctx, `SELECT id,email,display_name,role,is_active,created_at FROM users ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	users := []domain.User{}
	for rows.Next() {
		var user domain.User
		if err = rows.Scan(&user.ID, &user.Email, &user.DisplayName, &user.Role, &user.IsActive, &user.CreatedAt); err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, rows.Err()
}
func (p *Postgres) SetSiteAccess(ctx context.Context, access domain.SiteAccess) error {
	_, err := p.pool.Exec(ctx, `INSERT INTO site_access (site_id,user_id,access_role) VALUES ($1,$2,$3) ON CONFLICT (site_id,user_id) DO UPDATE SET access_role=EXCLUDED.access_role`, access.SiteID, access.UserID, access.Role)
	return err
}
func (p *Postgres) ListSiteAccess(ctx context.Context, siteID uuid.UUID) ([]domain.SiteAccess, error) {
	rows, err := p.pool.Query(ctx, `SELECT site_id,user_id,access_role FROM site_access WHERE site_id=$1`, siteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []domain.SiteAccess{}
	for rows.Next() {
		var access domain.SiteAccess
		if err = rows.Scan(&access.SiteID, &access.UserID, &access.Role); err != nil {
			return nil, err
		}
		result = append(result, access)
	}
	return result, rows.Err()
}

func (p *Postgres) DeleteSiteAccess(ctx context.Context, siteID, userID uuid.UUID) error {
	command, err := p.pool.Exec(ctx, `DELETE FROM site_access WHERE site_id=$1 AND user_id=$2`, siteID, userID)
	if err != nil {
		return err
	}
	if command.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func NewPostgres(ctx context.Context, databaseURL string) (*Postgres, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	if err = pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return &Postgres{pool: pool}, nil
}

func (p *Postgres) Close() { p.pool.Close() }

const siteColumns = `id, name, address, latitude, longitude, land_size, land_size_unit, google_maps_url, notes, internet_available, internet_supports_24ghz, land_leveling_required, frontage_meters, electrical_extension_km, input_status, created_at, updated_at`

func scanSite(row pgx.Row) (domain.Site, error) {
	var site domain.Site
	err := row.Scan(&site.ID, &site.Name, &site.Address, &site.Latitude, &site.Longitude, &site.LandSize, &site.LandSizeUnit, &site.GoogleMapsURL, &site.Notes, &site.InternetAvailable, &site.InternetSupports24GHz, &site.LandLevelingRequired, &site.FrontageMeters, &site.ElectricalExtensionKM, &site.InputStatus, &site.CreatedAt, &site.UpdatedAt)
	return site, err
}

func (p *Postgres) CreateSite(ctx context.Context, site domain.Site) (domain.Site, error) {
	query := `INSERT INTO sites (id,name,address,latitude,longitude,location,land_size,land_size_unit,google_maps_url,notes,internet_available,internet_supports_24ghz,land_leveling_required,frontage_meters,electrical_extension_km,input_status,created_at,updated_at)
	VALUES ($1,$2,$3,$4,$5,CASE WHEN $4::double precision IS NULL THEN NULL ELSE ST_SetSRID(ST_MakePoint($5,$4),4326)::geography END,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)
	RETURNING ` + siteColumns
	return scanSite(p.pool.QueryRow(ctx, query, site.ID, site.Name, site.Address, site.Latitude, site.Longitude, site.LandSize, site.LandSizeUnit, site.GoogleMapsURL, site.Notes, site.InternetAvailable, site.InternetSupports24GHz, site.LandLevelingRequired, site.FrontageMeters, site.ElectricalExtensionKM, site.InputStatus, site.CreatedAt, site.UpdatedAt))
}

func (p *Postgres) ListSites(ctx context.Context) ([]domain.Site, error) {
	rows, err := p.pool.Query(ctx, `SELECT `+siteColumns+` FROM sites ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var sites []domain.Site
	for rows.Next() {
		site, scanErr := scanSite(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		sites = append(sites, site)
	}
	return sites, rows.Err()
}

func (p *Postgres) GetSite(ctx context.Context, id uuid.UUID) (domain.Site, error) {
	site, err := scanSite(p.pool.QueryRow(ctx, `SELECT `+siteColumns+` FROM sites WHERE id=$1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Site{}, ErrNotFound
	}
	return site, err
}

func (p *Postgres) UpdateSite(ctx context.Context, site domain.Site) (domain.Site, error) {
	query := `UPDATE sites SET name=$2,address=$3,latitude=$4,longitude=$5,
		location=CASE WHEN $4::double precision IS NULL THEN NULL ELSE ST_SetSRID(ST_MakePoint($5,$4),4326)::geography END,
		land_size=$6,land_size_unit=$7,google_maps_url=$8,notes=$9,internet_available=$10,internet_supports_24ghz=$11,land_leveling_required=$12,frontage_meters=$13,electrical_extension_km=$14,updated_at=$15
		WHERE id=$1 RETURNING ` + siteColumns
	result, err := scanSite(p.pool.QueryRow(ctx, query, site.ID, site.Name, site.Address, site.Latitude, site.Longitude, site.LandSize, site.LandSizeUnit, site.GoogleMapsURL, site.Notes, site.InternetAvailable, site.InternetSupports24GHz, site.LandLevelingRequired, site.FrontageMeters, site.ElectricalExtensionKM, site.UpdatedAt))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Site{}, ErrNotFound
	}
	return result, err
}

// DeleteSite relies on the database foreign keys to remove analyses and their
// associated metrics atomically with the site.
func (p *Postgres) DeleteSite(ctx context.Context, id uuid.UUID) error {
	command, err := p.pool.Exec(ctx, `DELETE FROM sites WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if command.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (p *Postgres) AddSiteImages(ctx context.Context, siteID uuid.UUID, images []domain.SiteImage) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var exists bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM sites WHERE id=$1)`, siteID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return ErrNotFound
	}
	for _, image := range images {
		_, err = tx.Exec(ctx, `INSERT INTO site_images (id,site_id,mime_type,image_data,created_at) VALUES ($1,$2,$3,$4,$5)`, image.ID, siteID, image.MIMEType, image.Data, image.CreatedAt)
		if err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (p *Postgres) GetSiteImages(ctx context.Context, siteID uuid.UUID) ([]domain.SiteImage, error) {
	rows, err := p.pool.Query(ctx, `SELECT id,site_id,mime_type,image_data,created_at FROM site_images WHERE site_id=$1 ORDER BY created_at`, siteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	images := []domain.SiteImage{}
	for rows.Next() {
		var image domain.SiteImage
		if err = rows.Scan(&image.ID, &image.SiteID, &image.MIMEType, &image.Data, &image.CreatedAt); err != nil {
			return nil, err
		}
		images = append(images, image)
	}
	return images, rows.Err()
}

func (p *Postgres) CreateAnalysis(ctx context.Context, run domain.AnalysisRun) (domain.AnalysisRun, error) {
	_, err := p.pool.Exec(ctx, `INSERT INTO analysis_runs (id,site_id,status,analysis_radius_meters,assessment_status,recommendation,started_at,created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`, run.ID, run.SiteID, run.Status, run.AnalysisRadiusMeters, run.AssessmentStatus, run.Recommendation, run.StartedAt, run.CreatedAt)
	return run, err
}

func (p *Postgres) CompleteAnalysis(ctx context.Context, run domain.AnalysisRun) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	financialJSON, _ := json.Marshal(run.Financial)
	scoringJSON, _ := json.Marshal(run.Scoring)
	_, err = tx.Exec(ctx, `UPDATE analysis_runs SET status=$2,overall_score=$3,assessment_status=$4,recommendation=$5,financial_result=$6,completed_at=$7,scoring_summary=$8 WHERE id=$1`, run.ID, run.Status, run.OverallScore, run.AssessmentStatus, run.Recommendation, financialJSON, run.CompletedAt, scoringJSON)
	if err != nil {
		return err
	}
	for _, metric := range run.Metrics {
		sourceJSON, _ := json.Marshal(metric.Source)
		assumptionsJSON, _ := json.Marshal(metric.Assumptions)
		_, err = tx.Exec(ctx, `INSERT INTO analysis_metrics (id,analysis_run_id,metric_type,raw_value,normalized_score,data_status,source,assumptions,created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`, metric.ID, metric.AnalysisRunID, metric.Type, metric.RawValue, metric.NormalizedScore, metric.Status, sourceJSON, assumptionsJSON, metric.CreatedAt)
		if err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// UpdateAnalysisScoring applies a newer deterministic scoring policy to evidence
// that was already collected. It deliberately does not re-query providers or
// overwrite factual evidence, sources, or data-status labels.
func (p *Postgres) UpdateAnalysisScoring(ctx context.Context, run domain.AnalysisRun) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	scoringJSON, _ := json.Marshal(run.Scoring)
	command, err := tx.Exec(ctx, `UPDATE analysis_runs SET overall_score=$2,assessment_status=$3,recommendation=$4,scoring_summary=$5 WHERE id=$1`, run.ID, run.OverallScore, run.AssessmentStatus, run.Recommendation, scoringJSON)
	if err != nil {
		return err
	}
	if command.RowsAffected() == 0 {
		return ErrNotFound
	}
	for _, metric := range run.Metrics {
		sourceJSON, _ := json.Marshal(metric.Source)
		assumptionsJSON, _ := json.Marshal(metric.Assumptions)
		command, err = tx.Exec(ctx, `UPDATE analysis_metrics SET raw_value=$2, normalized_score=$3, data_status=$4, source=$5, assumptions=$6 WHERE id=$1 AND analysis_run_id=$7`, metric.ID, metric.RawValue, metric.NormalizedScore, metric.Status, sourceJSON, assumptionsJSON, run.ID)
		if err != nil {
			return err
		}
		if command.RowsAffected() == 0 {
			_, err = tx.Exec(ctx, `INSERT INTO analysis_metrics (id,analysis_run_id,metric_type,raw_value,normalized_score,data_status,source,assumptions,created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) ON CONFLICT (id) DO NOTHING`, metric.ID, metric.AnalysisRunID, metric.Type, metric.RawValue, metric.NormalizedScore, metric.Status, sourceJSON, assumptionsJSON, metric.CreatedAt)
			if err != nil {
				return err
			}
		}
	}
	return tx.Commit(ctx)
}

func (p *Postgres) UpdateAnalysisStationRecommendation(ctx context.Context, id uuid.UUID, recommendation []byte) error {
	command, err := p.pool.Exec(ctx, `UPDATE analysis_runs SET station_recommendation=$2 WHERE id=$1`, id, recommendation)
	if err != nil {
		return err
	}
	if command.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateAnalysisAIAssessments atomically merges newly generated locale entries
// so concurrent Thai and English generation cannot overwrite one another.
func (p *Postgres) UpdateAnalysisAIAssessments(ctx context.Context, id uuid.UUID, assessments []byte) error {
	command, err := p.pool.Exec(ctx, `UPDATE analysis_runs SET ai_assessments=COALESCE(ai_assessments, '{}'::jsonb) || $2::jsonb WHERE id=$1`, id, assessments)
	if err != nil {
		return err
	}
	if command.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (p *Postgres) GetAnalysis(ctx context.Context, id uuid.UUID) (domain.AnalysisRun, error) {
	var run domain.AnalysisRun
	var financialJSON, scoringJSON, stationRecommendationJSON, aiAssessmentsJSON []byte
	err := p.pool.QueryRow(ctx, `SELECT id,site_id,status,analysis_radius_meters,overall_score,assessment_status,recommendation,financial_result,scoring_summary,station_recommendation,ai_assessments,started_at,completed_at,created_at FROM analysis_runs WHERE id=$1`, id).Scan(&run.ID, &run.SiteID, &run.Status, &run.AnalysisRadiusMeters, &run.OverallScore, &run.AssessmentStatus, &run.Recommendation, &financialJSON, &scoringJSON, &stationRecommendationJSON, &aiAssessmentsJSON, &run.StartedAt, &run.CompletedAt, &run.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.AnalysisRun{}, ErrNotFound
	}
	if err != nil {
		return domain.AnalysisRun{}, err
	}
	if len(financialJSON) > 0 && string(financialJSON) != "null" {
		_ = json.Unmarshal(financialJSON, &run.Financial)
	}
	if len(scoringJSON) > 0 && string(scoringJSON) != "null" {
		_ = json.Unmarshal(scoringJSON, &run.Scoring)
	}
	run.StationRecommendation = stationRecommendationJSON
	run.AIAssessments = aiAssessmentsJSON
	rows, err := p.pool.Query(ctx, `SELECT id,analysis_run_id,metric_type,raw_value,normalized_score,data_status,source,assumptions,created_at FROM analysis_metrics WHERE analysis_run_id=$1 ORDER BY created_at`, id)
	if err != nil {
		return domain.AnalysisRun{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var metric domain.Metric
		var sourceJSON, assumptionsJSON []byte
		if err = rows.Scan(&metric.ID, &metric.AnalysisRunID, &metric.Type, &metric.RawValue, &metric.NormalizedScore, &metric.Status, &sourceJSON, &assumptionsJSON, &metric.CreatedAt); err != nil {
			return domain.AnalysisRun{}, err
		}
		_ = json.Unmarshal(sourceJSON, &metric.Source)
		_ = json.Unmarshal(assumptionsJSON, &metric.Assumptions)
		run.Metrics = append(run.Metrics, metric)
	}
	return run, rows.Err()
}

// GetLatestCompletedAnalysisForSite returns the most recent completed result so
// staff can reopen existing evidence without collecting provider data again.
func (p *Postgres) GetLatestCompletedAnalysisForSite(ctx context.Context, siteID uuid.UUID) (domain.AnalysisRun, error) {
	var run domain.AnalysisRun
	var financialJSON, scoringJSON, stationRecommendationJSON, aiAssessmentsJSON []byte
	err := p.pool.QueryRow(ctx, `SELECT id,site_id,status,analysis_radius_meters,overall_score,assessment_status,recommendation,financial_result,scoring_summary,station_recommendation,ai_assessments,started_at,completed_at,created_at
		FROM analysis_runs
		WHERE site_id=$1 AND status='completed'
		ORDER BY completed_at DESC NULLS LAST, created_at DESC
		LIMIT 1`, siteID).Scan(&run.ID, &run.SiteID, &run.Status, &run.AnalysisRadiusMeters, &run.OverallScore, &run.AssessmentStatus, &run.Recommendation, &financialJSON, &scoringJSON, &stationRecommendationJSON, &aiAssessmentsJSON, &run.StartedAt, &run.CompletedAt, &run.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.AnalysisRun{}, ErrNotFound
	}
	if err != nil {
		return domain.AnalysisRun{}, err
	}
	if len(financialJSON) > 0 && string(financialJSON) != "null" {
		_ = json.Unmarshal(financialJSON, &run.Financial)
	}
	if len(scoringJSON) > 0 && string(scoringJSON) != "null" {
		_ = json.Unmarshal(scoringJSON, &run.Scoring)
	}
	run.StationRecommendation = stationRecommendationJSON
	run.AIAssessments = aiAssessmentsJSON
	return run, nil
}
