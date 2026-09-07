ALTER TABLE sites
    ADD COLUMN internet_available BOOLEAN,
    ADD COLUMN internet_supports_24ghz BOOLEAN,
    ADD COLUMN land_leveling_required BOOLEAN,
    ADD COLUMN frontage_meters DOUBLE PRECISION CHECK (frontage_meters IS NULL OR frontage_meters >= 0),
    ADD COLUMN electrical_extension_km DOUBLE PRECISION CHECK (electrical_extension_km IS NULL OR electrical_extension_km >= 0);
