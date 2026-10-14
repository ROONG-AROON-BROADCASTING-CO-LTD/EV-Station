"""Import only mapped motor roads; atomically retain current + previous snapshots."""
import argparse
import hashlib
import json
import os
import time
import uuid
from datetime import datetime, timezone

import osmium
import psycopg
from psycopg import sql

CLASSES = {'motorway','motorway_link','trunk','trunk_link','primary','primary_link',
           'secondary','secondary_link','tertiary','tertiary_link','unclassified',
           'residential','living_street','service','road','track'}

def partition_name(dataset_id):
    return 'osm_roads_' + uuid.UUID(str(dataset_id)).hex

class Roads(osmium.SimpleHandler):
    def __init__(self, output, dataset_id):
        super().__init__()
        self.output, self.dataset_id = output, dataset_id
        self.factory = osmium.geom.WKBFactory()
        self.count = self.skipped = 0

    def way(self, way):
        road_class = way.tags.get('highway', '')
        if road_class not in CLASSES or way.tags.get('area') == 'yes':
            return
        try:
            geometry = self.factory.create_linestring(way)
        except (osmium.InvalidLocationError, RuntimeError):
            self.skipped += 1
            return
        self.output.write_row((self.dataset_id, way.id, road_class,
            way.tags.get('name:th', way.tags.get('name', '')), way.tags.get('ref', ''),
            way.tags.get('oneway', ''), way.tags.get('access', ''),
            way.tags.get('motor_vehicle', ''), geometry))
        self.count += 1
        if self.count % 100000 == 0:
            print(json.dumps({'roads_loaded': self.count}), flush=True)

def main():
    args = argparse.ArgumentParser()
    args.add_argument('pbf')
    args.add_argument('--minimum-roads', type=int, default=100000)
    parsed = args.parse_args()
    started = time.monotonic()
    with osmium.io.Reader(parsed.pbf) as reader:
        timestamp = reader.header().get('osmosis_replication_timestamp')
    if not timestamp:
        raise RuntimeError('Source timestamp missing; cannot publish undated evidence')
    source_date = datetime.fromisoformat(timestamp.replace('Z', '+00:00'))
    if source_date > datetime.now(timezone.utc):
        raise RuntimeError('Source timestamp is in the future')
    with open(parsed.pbf, 'rb') as source:
        digest = hashlib.file_digest(source, 'sha256').hexdigest()
    dataset_id = uuid.uuid4()
    table = partition_name(dataset_id)
    # The connection is supplied through environment only, never printed.
    with psycopg.connect(os.environ['ROAD_DATABASE_URL'], autocommit=True) as conn:
        if not conn.execute("SELECT pg_try_advisory_lock(727263724)").fetchone()[0]:
            raise RuntimeError('Another road import is already running')
        existing = conn.execute("SELECT sha256,source_timestamp,road_count FROM osm_road_datasets WHERE status='current'").fetchone()
        if existing and existing[0] == digest:
            print(json.dumps({'result': 'unchanged', 'sha256': digest}))
            return
        if existing and source_date <= existing[1]:
            raise RuntimeError('Refusing to replace current roads with an older snapshot')
        # Remove remnants of failed imports only; published snapshots remain intact.
        for (old_id,) in conn.execute("SELECT id FROM osm_road_datasets WHERE status='staging'").fetchall():
            conn.execute(sql.SQL('DROP TABLE IF EXISTS {}').format(sql.Identifier(partition_name(old_id))))
            conn.execute('DELETE FROM osm_road_datasets WHERE id=%s', (old_id,))
        conn.execute("INSERT INTO osm_road_datasets(id,status,source_url,source_timestamp,sha256) VALUES (%s,'staging',%s,%s,%s)",
                     (dataset_id, 'https://download.geofabrik.de/asia/thailand-latest.osm.pbf', source_date, digest))
        published = False
        try:
            conn.execute(sql.SQL('CREATE TABLE {} PARTITION OF osm_roads FOR VALUES IN ({})').format(sql.Identifier(table), sql.Literal(dataset_id)))
            with conn.cursor() as cur:
                with cur.copy(sql.SQL('COPY {} FROM STDIN').format(sql.Identifier(table))) as copy:
                    handler = Roads(copy, dataset_id)
                    handler.apply_file(parsed.pbf, locations=True, idx='flex_mem')
            if handler.count < parsed.minimum_roads or handler.skipped > handler.count * 0.01:
                raise RuntimeError('Road coverage/geometry validation failed')
            database_name = conn.execute('SELECT current_database()').fetchone()[0]
            if existing and not database_name.endswith('_test') and handler.count < existing[2]*0.8:
                raise RuntimeError('Road count dropped more than 20%; manual review required')
            conn.execute(sql.SQL('CREATE INDEX {} ON {} USING gist ((geom::geography))').format(sql.Identifier(table+'_geo'), sql.Identifier(table)))
            conn.execute(sql.SQL('ANALYZE {}').format(sql.Identifier(table)))
            # All five geographic audit points must have mapped roads within 10 km.
            for lat, lon in [(13.771269,100.582770),(18.7883,98.9853),(16.4419,102.8359),(7.0084,100.4747),(14.353,100.5692)]:
                found = conn.execute(sql.SQL('SELECT EXISTS(SELECT 1 FROM {} WHERE ST_DWithin(geom::geography,ST_SetSRID(ST_MakePoint(%s,%s),4326)::geography,10000))').format(sql.Identifier(table)),(lon,lat)).fetchone()[0]
                if not found:
                    raise RuntimeError('Regional coverage validation failed')
            with conn.transaction():
                for (old_id,) in conn.execute("SELECT id FROM osm_road_datasets WHERE status='previous'").fetchall():
                    conn.execute(sql.SQL('DROP TABLE {}').format(sql.Identifier(partition_name(old_id))))
                    conn.execute('DELETE FROM osm_road_datasets WHERE id=%s',(old_id,))
                conn.execute("UPDATE osm_road_datasets SET status='previous' WHERE status='current'")
                conn.execute("UPDATE osm_road_datasets SET status='current',road_count=%s,skipped_count=%s WHERE id=%s", (handler.count,handler.skipped,dataset_id))
            published = True
            size = conn.execute('SELECT pg_total_relation_size(%s::regclass)',(table,)).fetchone()[0]
            print(json.dumps({'result':'published','dataset_id':str(dataset_id),'source_timestamp':timestamp,'roads':handler.count,'skipped':handler.skipped,'table_and_index_bytes':size,'seconds':round(time.monotonic()-started,2)}))
        except Exception:
            if not published:
                conn.execute(sql.SQL('DROP TABLE IF EXISTS {}').format(sql.Identifier(table)))
                conn.execute('DELETE FROM osm_road_datasets WHERE id=%s',(dataset_id,))
            raise

if __name__ == '__main__':
    main()
