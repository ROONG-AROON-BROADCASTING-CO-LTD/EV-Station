DROP INDEX IF EXISTS sites_reference_code_unique_idx;
ALTER TABLE sites DROP COLUMN IF EXISTS reference_code;
DROP SEQUENCE IF EXISTS site_reference_2569_seq;
