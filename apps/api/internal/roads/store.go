package roads

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"time"
)

var ErrUnavailable = errors.New("local road snapshot unavailable or older than 30 days")

type Result struct {
	DatasetID                                      uuid.UUID
	SourceTime                                     time.Time
	ImportedAt                                     time.Time
	Counts                                         map[string]int
	Count                                          int
	Distance                                       *float64
	Name, Ref, Class, Oneway, Access, MotorVehicle string
}
type Lookup interface {
	Nearby(context.Context, float64, float64, int) (Result, error)
}
type Store struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

func (s *Store) Nearby(ctx context.Context, lat, lon float64, radius int) (Result, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	var result Result
	err := s.pool.QueryRow(ctx, `SELECT id,source_timestamp,imported_at FROM osm_road_datasets WHERE status='current' AND source_timestamp >= now()-interval '30 days'`).Scan(&result.DatasetID, &result.SourceTime, &result.ImportedAt)
	if err != nil {
		return result, ErrUnavailable
	}
	result.Counts = map[string]int{}
	rows, err := s.pool.Query(ctx, `SELECT road_class,count(*) FROM osm_roads WHERE dataset_id=$1 AND road_class IN ('motorway','trunk','primary','secondary','tertiary') AND ST_DWithin(geom::geography,ST_SetSRID(ST_MakePoint($2,$3),4326)::geography,$4) GROUP BY road_class`, result.DatasetID, lon, lat, radius)
	if err != nil {
		return result, err
	}
	for rows.Next() {
		var class string
		var count int
		if err = rows.Scan(&class, &count); err != nil {
			rows.Close()
			return result, err
		}
		result.Counts[class] = count
		result.Count += count
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return result, err
	}
	if result.Count == 0 {
		return result, nil
	}
	err = s.pool.QueryRow(ctx, `SELECT ST_Distance(geom::geography,ST_SetSRID(ST_MakePoint($2,$3),4326)::geography),name,ref,road_class,oneway,access,motor_vehicle FROM osm_roads WHERE dataset_id=$1 AND road_class IN ('motorway','trunk','primary','secondary','tertiary') AND ST_DWithin(geom::geography,ST_SetSRID(ST_MakePoint($2,$3),4326)::geography,$4) ORDER BY ST_Distance(geom::geography,ST_SetSRID(ST_MakePoint($2,$3),4326)::geography),osm_id LIMIT 1`, result.DatasetID, lon, lat, radius).Scan(&result.Distance, &result.Name, &result.Ref, &result.Class, &result.Oneway, &result.Access, &result.MotorVehicle)
	return result, err
}
