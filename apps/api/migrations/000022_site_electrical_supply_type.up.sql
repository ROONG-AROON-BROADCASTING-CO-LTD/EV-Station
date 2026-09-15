ALTER TABLE sites
    ADD COLUMN electrical_supply_type TEXT NOT NULL DEFAULT 'unknown'
    CHECK (electrical_supply_type IN ('unknown', 'overhead', 'underground'));
