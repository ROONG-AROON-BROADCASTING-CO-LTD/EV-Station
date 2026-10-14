"""Destructive fixtures ONLY for a disposable *_test database."""
import json
import os
import subprocess
from pathlib import Path
from datetime import datetime, timezone, timedelta
import osmium
import psycopg

URL=os.environ['ROAD_DATABASE_URL']
with psycopg.connect(URL) as conn:
    if not conn.execute('SELECT current_database()').fetchone()[0].endswith('_test'):
        raise RuntimeError('Retention fixtures require a disposable *_test database')

def fixture(path, timestamp):
    header=osmium.io.Header()
    header.set('osmosis_replication_timestamp',timestamp)
    with osmium.SimpleWriter(str(path),header=header,overwrite=True) as writer:
        for index,(lat,lon) in enumerate([(13.771269,100.582770),(18.7883,98.9853),(16.4419,102.8359),(7.0084,100.4747),(14.353,100.5692)]):
            n=index*2+1
            writer.add_node(osmium.osm.mutable.Node(id=n,location=(lon,lat)))
            writer.add_node(osmium.osm.mutable.Node(id=n+1,location=(lon+.001,lat+.001)))
            writer.add_way(osmium.osm.mutable.Way(id=index+1,nodes=[n,n+1],tags={'highway':'primary','name':'Fixture road'}))

def run(path, minimum=1, success=True):
    r=subprocess.run(['python','/work/import_roads.py',str(path),'--minimum-roads',str(minimum)],capture_output=True,text=True)
    if (r.returncode==0)!=success:
        raise RuntimeError('Unexpected fixture outcome: '+r.stderr)

paths=[]
start=datetime.now(timezone.utc)-timedelta(seconds=10)
for i in range(3):
    path=Path('/tmp')/f'roads-fixture-{i}.osm.pbf'
    fixture(path,(start+timedelta(seconds=i*3)).isoformat().replace('+00:00','Z'))
    paths.append(path)
run(paths[0])
with psycopg.connect(URL) as conn:
    before=conn.execute("SELECT id FROM osm_road_datasets WHERE status='current'").fetchone()[0]
run(paths[1],minimum=100,success=False)
with psycopg.connect(URL) as conn:
    assert conn.execute("SELECT id FROM osm_road_datasets WHERE status='current'").fetchone()[0]==before
    assert conn.execute("SELECT count(*) FROM osm_road_datasets WHERE status='staging'").fetchone()[0]==0
run(paths[1])
run(paths[2])
run(paths[2])  # Same-file retry must be a no-op.
with psycopg.connect(URL) as conn:
    statuses=dict(conn.execute('SELECT status,count(*) FROM osm_road_datasets GROUP BY status').fetchall())
    assert statuses=={'current':1,'previous':1},statuses
    assert conn.execute("SELECT count(*) FROM pg_inherits WHERE inhparent='osm_roads'::regclass").fetchone()[0]==2
print(json.dumps({'failed_import_preserves_current':True,'two_snapshot_retention':True,'same_file_idempotent':True}))
