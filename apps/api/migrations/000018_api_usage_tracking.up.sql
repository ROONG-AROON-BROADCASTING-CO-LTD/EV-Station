CREATE TABLE api_usage_plans (
    provider_id TEXT PRIMARY KEY,
    display_name TEXT NOT NULL,
    unit_label TEXT NOT NULL,
    included_units BIGINT,
    overage_price_thb NUMERIC(14,4),
    CHECK (included_units IS NULL OR included_units >= 0),
    CHECK (overage_price_thb IS NULL OR overage_price_thb >= 0)
);

CREATE TABLE api_usage_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider_id TEXT NOT NULL REFERENCES api_usage_plans(provider_id) ON DELETE RESTRICT,
    units BIGINT NOT NULL CHECK (units > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX api_usage_events_provider_created_at_idx ON api_usage_events(provider_id, created_at);

INSERT INTO api_usage_plans (provider_id, display_name, unit_label, included_units, overage_price_thb) VALUES
  ('google-maps-js', 'Google Maps JavaScript API', 'map loads', NULL, NULL),
  ('google-places', 'Google Places API (New)', 'nearby-search requests', NULL, NULL),
  ('gemini-advisory', 'Gemini AI', 'generations', NULL, NULL),
  ('gistda-elevation', 'GISTDA Sphere Elevation API', 'elevation requests', 200, NULL);
