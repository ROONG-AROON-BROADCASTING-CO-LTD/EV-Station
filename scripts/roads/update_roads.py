"""Weekly refresh worker. A failed download/import preserves the published roads."""
import hashlib
import json
import os
from pathlib import Path
import subprocess
import time
import urllib.error
import urllib.request
import psycopg

BASE = 'https://download.geofabrik.de/asia/thailand-latest.osm.pbf'
TARGET = Path('/data/thailand-latest.osm.pbf')


class RefreshFailure(Exception):
    def __init__(self, stage, cause):
        # The cause can contain connection strings; never put it in the message.
        super().__init__(f'Road refresh failed during {stage}')
        self.stage = stage
        self.cause = cause


def refresh():
    TARGET.parent.mkdir(parents=True, exist_ok=True)
    temporary = TARGET.with_suffix('.download')
    stage = 'database'
    try:
        with psycopg.connect(os.environ['ROAD_DATABASE_URL'], connect_timeout=10) as conn:
            due = conn.execute("SELECT NOT EXISTS(SELECT 1 FROM osm_road_datasets WHERE status='current' AND imported_at > now()-interval '7 days')").fetchone()[0]
        if not due:
            return
        stage = 'download'
        request = urllib.request.Request(BASE, headers={'User-Agent':'RBC-EV-Station-road-refresh/1.0'})
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
        stage = 'checksum'
        with temporary.open('rb') as source:
            digest = hashlib.file_digest(source, 'md5').hexdigest()
        with urllib.request.urlopen(BASE+'.md5', timeout=30) as response:
            expected = response.read(1024).decode().split()[0]
        if digest != expected:
            raise RuntimeError('Download checksum mismatch; keeping published data')
        # Only one downloaded source is retained, plus its temporary replacement.
        temporary.replace(TARGET)
        stage = 'import'
        subprocess.run(['python','-u','/work/import_roads.py',str(TARGET)],check=True,timeout=3600)
        print(json.dumps({'event':'road_refresh_completed'}),flush=True)
    except Exception as exc:
        raise RefreshFailure(stage, exc) from None
    finally:
        temporary.unlink(missing_ok=True)


def retry_delay(failures):
    return min(300 * 2 ** min(max(failures - 1, 0), 4), 3600)


def failure_event(exc, delay):
    cause = exc.cause if isinstance(exc, RefreshFailure) else exc
    event = {'event': 'road_refresh_failed', 'stage': getattr(exc, 'stage', 'worker'),
             'error_type': type(cause).__name__, 'retry_after_seconds': delay}
    # Emit allowlisted diagnostic fields only. Error text, URLs and command
    # arguments may contain secrets from psycopg or an HTTP proxy.
    if isinstance(cause, urllib.error.HTTPError):
        event['http_status'] = cause.code
    elif isinstance(cause, urllib.error.URLError):
        event['reason_type'] = type(cause.reason).__name__
    if isinstance(cause, psycopg.Error) and cause.sqlstate:
        event['sqlstate'] = cause.sqlstate
    if isinstance(cause, subprocess.CalledProcessError):
        event['exit_code'] = cause.returncode
    if isinstance(cause, OSError) and isinstance(cause.errno, int):
        event['errno'] = cause.errno
    return event


def run_worker():
    failures = 0
    while True:
        try:
            refresh()
            Path('/tmp/road-worker-heartbeat').touch()
            failures = 0
            delay = 6 * 3600
        except Exception as exc:
            failures += 1
            delay = retry_delay(failures)
            print(json.dumps(failure_event(exc, delay)), flush=True)
        time.sleep(delay)


if __name__ == '__main__':
    run_worker()
