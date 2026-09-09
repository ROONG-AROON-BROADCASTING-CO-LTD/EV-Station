CREATE OR REPLACE FUNCTION next_site_reference_code()
RETURNS text
LANGUAGE plpgsql
AS $$
DECLARE
    sequence_number bigint;
    sequence_text text;
BEGIN
    sequence_number := nextval('site_reference_2569_seq');
    sequence_text := sequence_number::text;
    RETURN 'RBC-2569' || LPAD(sequence_text, GREATEST(3, LENGTH(sequence_text)), '0');
END;
$$;

ALTER TABLE sites
    ALTER COLUMN reference_code
    SET DEFAULT next_site_reference_code();
