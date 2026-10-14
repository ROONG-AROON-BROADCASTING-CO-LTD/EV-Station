import os
import psycopg

with psycopg.connect(os.environ['ROAD_DATABASE_URL'],connect_timeout=5) as connection:
    healthy=connection.execute("SELECT EXISTS(SELECT 1 FROM osm_road_datasets WHERE status='current' AND source_timestamp>now()-interval '14 days')").fetchone()[0]
raise SystemExit(0 if healthy else 1)
