-- Add user_id column. Default 0 means "legacy/unassigned" (pre-migration data).
-- No REFERENCES users(id) foreign key: legacy rows have user_id=0 with no
-- corresponding user row. Enforce the relationship at the application level instead.
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS user_id BIGINT NOT NULL DEFAULT 0;
CREATE INDEX IF NOT EXISTS idx_jobs_user_id ON jobs(user_id);

-- Add the per-user unique constraint (user_id, external_id, source).
CREATE UNIQUE INDEX IF NOT EXISTS idx_jobs_user_source_ext ON jobs(user_id, external_id, source);
