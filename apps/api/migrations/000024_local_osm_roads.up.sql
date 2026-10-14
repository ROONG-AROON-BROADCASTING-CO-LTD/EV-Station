CREATE TABLE osm_road_datasets (
    id UUID PRIMARY KEY,
    status TEXT NOT NULL CHECK (status IN ('staging','current','previous')),
    source_url TEXT NOT NULL,
    source_timestamp TIMESTAMPTZ NOT NULL,
    imported_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    sha256 TEXT NOT NULL,
    road_count BIGINT NOT NULL DEFAULT 0,
    skipped_count BIGINT NOT NULL DEFAULT 0
);
CREATE UNIQUE INDEX osm_roads_one_current ON osm_road_datasets(status) WHERE status='current';
CREATE UNIQUE INDEX osm_roads_one_previous ON osm_road_datasets(status) WHERE status='previous';
CREATE TABLE osm_roads (
    dataset_id UUID NOT NULL REFERENCES osm_road_datasets(id),
    osm_id BIGINT NOT NULL,
    road_class TEXT NOT NULL,
    name TEXT NOT NULL DEFAULT '',
    ref TEXT NOT NULL DEFAULT '',
    oneway TEXT NOT NULL DEFAULT '',
    access TEXT NOT NULL DEFAULT '',
    motor_vehicle TEXT NOT NULL DEFAULT '',
    geom geometry(LineString,4326) NOT NULL,
    PRIMARY KEY (dataset_id,osm_id)
) PARTITION BY LIST(dataset_id);
