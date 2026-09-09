ALTER TABLE sites
    ALTER COLUMN reference_code
    SET DEFAULT ('RBC-2569' || LPAD(nextval('site_reference_2569_seq')::TEXT, 3, '0'));

DROP FUNCTION IF EXISTS next_site_reference_code();
