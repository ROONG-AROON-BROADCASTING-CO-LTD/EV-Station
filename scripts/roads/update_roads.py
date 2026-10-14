"""Weekly refresh worker. A failed download/import preserves the published roads."""
import hashlib
import json
import os
from pathlib import Path
import subprocess
import time
import urllib.request
import psycopg

BASE = 'https://download.geofabrik.de/asia/thailand-latest.osm.pbf'
TARGET = Path('/data/thailand-latest.osm.pbf')

def refresh():
    TARGET.parent.mkdir(parents=True, exist_ok=True)
    with psycopg.connect(os.environ['ROAD_DATABASE_URL']) as conn:
        due = conn.execute("SELECT NOT EXISTS(SELECT 1 FROM osm_road_datasets WHERE status='current' AND imported_at > now()-interval '7 days')").fetchone()[0]
    if not due:
        return
    request = urllib.request.Request(BASE, headers={'User-Agent':'RBC-EV-Station-road-refresh/1.0'})
    temporary = TARGET.with_suffix('.download')
    try:
        started = time.monotonic()
        with urllib.request.urlopen(request, timeout=60) as response, temporary.open('wb') as output:
            if not response.url.startswith('https://download.geofabrik.de/'):
                raise RuntimeError('Unexpected download host')
            count = 0
            while chunk := response.read(1024*1024):
                output.write(chunk)
                count += len(chunk)
                if count > 2*1024**3 or time.monotonic()-started > 1800:
                    raise RuntimeError('Download size/time budget exceeded')
        with temporary.open('rb') as source:
            digest = hashlib.file_digest(source, 'md5').hexdigest()
        with urllib.request.urlopen(BASE+'.md5', timeout=30) as response:
            expected = response.read(1024).decode().split()[0]
        if digest != expected:
            raise RuntimeError('Download checksum mismatch; keeping published data')
        # Only one downloaded source is retained, plus its temporary replacement.
        temporary.replace(TARGET)
        subprocess.run(['python','-u','/work/import_roads.py',str(TARGET)],check=True,timeout=3600)
        print(json.dumps({'event':'road_refresh_completed'}),flush=True)
    finally:
        temporary.unlink(missing_ok=True)

if __name__=='__main__':
    while True:
        try:
            refresh()
            Path('/tmp/road-worker-heartbeat').touch()
            time.sleep(6*3600)
        except Exception as exc:
            # Avoid logging connection URLs, credentials or subprocess arguments.
            print(json.dumps({'event':'road_refresh_failed','error_type':type(exc).__name__}),flush=True)
            time.sleep(6*3600)
