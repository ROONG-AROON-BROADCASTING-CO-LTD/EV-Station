DROP INDEX IF EXISTS analysis_runs_queue_idx;
ALTER TABLE analysis_runs DROP COLUMN IF EXISTS lease_expires_at;
