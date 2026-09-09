CREATE SEQUENCE site_reference_2569_seq START WITH 1;

ALTER TABLE sites
    ADD COLUMN reference_code VARCHAR(24);

ALTER TABLE sites
    ALTER COLUMN reference_code
    SET DEFAULT ('RBC-2569' || LPAD(nextval('site_reference_2569_seq')::TEXT, 3, '0'));

CREATE UNIQUE INDEX sites_reference_code_unique_idx
    ON sites (reference_code)
    WHERE reference_code IS NOT NULL;
