ALTER TABLE analysis_runs
    DROP CONSTRAINT IF EXISTS analysis_runs_analysis_radius_meters_check;

ALTER TABLE analysis_runs
    ADD CONSTRAINT analysis_runs_analysis_radius_meters_check
    CHECK (analysis_radius_meters IN (1000, 3000, 5000));
