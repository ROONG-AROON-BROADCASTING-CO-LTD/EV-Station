package roads

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"sort"
	"testing"
	"time"
)

func TestNationalSnapshotDatabase(t *testing.T) {
	url := os.Getenv("ROAD_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("isolated road database not provided")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	store := New(pool)
	points := []struct {
		name     string
		lat, lon float64
	}{{"Bangkok", 13.771269, 100.582770}, {"Ayutthaya", 14.353, 100.5692}, {"Chiang Mai", 18.7883, 98.9853}, {"Khon Kaen", 16.4419, 102.8359}, {"Songkhla", 7.0084, 100.4747}, {"Rural Nan", 18.775, 100.773}, {"Rural Kanchanaburi", 14.35, 99.12}}
	for _, p := range points {
		t.Run(p.name, func(t *testing.T) {
			durations := []float64{}
			for range 10 {
				started := time.Now()
				r, err := store.Nearby(context.Background(), p.lat, p.lon, 3000)
				if err != nil {
					t.Fatal(err)
				}
				if p.name != "Rural Kanchanaburi" && (r.Count == 0 || r.Distance == nil) {
					t.Fatal("no mapped major road in sample")
				}
				if r.Count == 0 && r.Distance != nil {
					t.Fatal("empty coverage invented a distance")
				}
				durations = append(durations, float64(time.Since(started).Microseconds())/1000)
			}
			sort.Float64s(durations)
			t.Logf("query milliseconds: median=%.2f p95=%.2f", durations[5], durations[9])
		})
	}
}
