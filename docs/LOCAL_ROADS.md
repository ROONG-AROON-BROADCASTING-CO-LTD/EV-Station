# Local Thailand road evidence

RBC uses a country-wide OpenStreetMap road snapshot in PostGIS as the primary
road source. Overpass remains a fallback when the local snapshot is unavailable,
older than 30 days, or fails its two-second lookup budget. Charging-station
queries remain separate remote requests with a nine-second budget when the local
road store is configured. A fast road lookup does not make every other provider fast.

## Source and interpretation

Source: [Geofabrik Thailand](https://download.geofabrik.de/asia/thailand.html),
OpenStreetMap contributors, ODbL 1.0. The import preserves the PBF header's
source timestamp and SHA-256. Geometry and selected road tags are retained;
contributor identities, unrelated map features and raw node tables are omitted.

Imported classes include major roads, links, residential roads, service roads,
unclassified roads and tracks. Existing screening rules still use major-road
classes (motorway, trunk, primary, secondary, tertiary). Counts represent OSM
way segments, not separate named roads or lanes. Full-line geography distance
replaces the previous nearest-vertex approximation. One-way, access and
motor-vehicle tags are retained and disclosed when present. Turn-restriction
relations are not imported. The data does not establish a legal driveway,
permitted turn, verified road width or physical access to the plot.

## Measured deployment — 14 October 2026

| Measurement | Result |
|---|---:|
| Source timestamp | 2026-10-13 20:21:31 UTC |
| Imported road ways | 2,826,704 |
| Skipped geometries | 0 |
| Road table and indexes, one production snapshot | 1,062,658,048 bytes (about 1.06 GB) |
| Disposable test import duration | 285.09 seconds |
| Production import duration | 323.4 seconds |
| Entire production database after import | about 1,075 MiB |
| Production Pracha Uthit road segments within 3 km | 541 |
| Nearest major-road line distance | 23.54 metres |
| New complete production report duration | 14.4 seconds |

Production report: `c8c815e0-92c1-44d1-a721-c5bd7d3572ac`, score 94.68,
weighted evidence coverage 79.17%, road source `local_open_data_snapshot`.
Whole-analysis timing is one observation, not a latency guarantee or a controlled
benchmark against the prior run.

Seven test pins, ten lookups each, measured local lookup medians of 38.77–104.27 ms
and maxima of 43.38–176.81 ms in the final isolated run. Samples covered Bangkok,
Ayutthaya, Chiang Mai, Khon Kaen, Songkhla, Nan and a rural Kanchanaburi point.
The Kanchanaburi pin had no mapped major roads within 3 km; an empty result is
preserved without inventing a distance. These samples do not certify every pin.
Production EXPLAIN ANALYZE confirmed the spatial GiST index was used (74.825 ms
for the sample count query).

## Retention and update safety

`osm_roads` is partitioned by dataset ID. `osm_road_datasets` identifies the
current and previous published datasets. A session advisory lock prevents
overlapping imports. Imports first populate a staging partition and build its
geography index, then check minimum coverage, geometry errors and regional pins.
Published-source rollback to an older timestamp is rejected. A production road
count decline greater than 20% requires manual review.

Publishing and pruning are transactional: the old previous partition is dropped,
current becomes previous, and staging becomes current. Failed imports remove their
staging partition and retain published data. Same-file retries are no-ops.
Historical reports store results and dataset provenance, not copies of road lines.

Steady state retains **at most two snapshots**, approximately 2.13 GB at today's
size. During an update there can temporarily be three, approximately 3.19 GB,
plus source downloads, WAL, backups and ordinary database overhead. Sizes will
change as OSM grows; this bounds retained versions, not the size of the map itself.
Partition drops reclaim their files without accumulating dead road rows weekly.

One PBF is retained in the `road_source` Docker volume, with one temporary download
during replacement. The redundant test download and disposable test database
were removed after verification.

## Running and monitoring

Start core services/migrations, then the opt-in updater profile:

```powershell
docker compose up -d
docker compose --profile roads up -d --build road-updater
```

`scripts/Start-RBC-EV-Station.ps1` starts the worker after core startup.
The worker checks every six hours and imports when the last successful import is
at least seven days old. Downloads are bounded, checked against the published MD5,
and imports time out after one hour. Failed refreshes retry after six hours without
replacing published roads. The worker has a 2 GiB memory limit and one CPU.
Health checks flag a source older than 14 days; application lookup stops using
the local source after 30 days. Docker health status is not an external alert.

```powershell
docker compose logs --tail 50 road-updater
docker exec ev-station-road-updater-1 python /work/health.py
docker exec ev-station-db-1 psql -U rbc -d rbc_ev_station -c "SELECT status,road_count,source_timestamp,imported_at FROM osm_road_datasets;"
```

To import the already downloaded source manually:

```powershell
docker compose --profile roads run --rm --no-deps --entrypoint python road-updater -u /work/import_roads.py /data/thailand-latest.osm.pbf
```

## Verification and rollback

Go tests/vet, frontend 27 tests, TypeScript/Vite and Docker builds passed.
Regression tests cover local-primary selection and remote fallback. Isolated
database tests covered failed import preservation, idempotency and exactly two
retained partitions after successive updates. Migration up/down passed in the
disposable database. Authenticated public-domain browser checks confirmed the
latest report and the visible local snapshot date without uncaught page errors.

Previous API image: `ev-station-api:before-local-roads-20261014`. Rolling the API
back does not require deleting road data. The down migration deletes imported
road partitions and metadata; it must not be used merely to revert application code.
Weekly refresh execution has been started; a full future weekly cycle has not yet
elapsed. Failure and retention behavior was verified using isolated fixtures.

Primary implementation references:
[Pyosmium](https://docs.osmcode.org/pyosmium/latest/user_manual/01-First-Steps/),
[PostGIS ST_DWithin](https://postgis.net/docs/ST_DWithin.html).
