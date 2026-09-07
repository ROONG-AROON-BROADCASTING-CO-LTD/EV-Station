-- Preserve historical 5 km analyses while allowing the new 2 km option.
-- New API requests are limited to 1, 2, or 3 km by the handler.
ALTER TABLE analysis_runs
    DROP CONSTRAINT IF EXISTS analysis_runs_analysis_radius_meters_check;

ALTER TABLE analysis_runs
    ADD CONSTRAINT analysis_runs_analysis_radius_meters_check
    CHECK (analysis_radius_meters IN (1000, 2000, 3000, 5000));
