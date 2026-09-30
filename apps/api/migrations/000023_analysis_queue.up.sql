ALTER TABLE analysis_runs ADD COLUMN lease_expires_at TIMESTAMPTZ;
UPDATE analysis_runs SET lease_expires_at = now() WHERE status = 'running';
CREATE INDEX analysis_runs_queue_idx ON analysis_runs (created_at) WHERE status IN ('pending', 'running');
