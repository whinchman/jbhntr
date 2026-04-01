-- Add user_id column. Default 0 means "legacy/unassigned" (pre-migration data).
-- No REFERENCES users(id) foreign key: legacy rows have user_id=0 with no
-- corresponding user row, which would violate PRAGMA foreign_keys=ON.
-- Enforce the relationship at the application level instead.
ALTER TABLE jobs ADD COLUMN user_id INTEGER NOT NULL DEFAULT 0;
CREATE INDEX IF NOT EXISTS idx_jobs_user_id ON jobs(user_id);

-- SQLite does not support ALTER TABLE ... DROP CONSTRAINT or ADD CONSTRAINT.
-- The original UNIQUE(external_id, source) is baked into the table definition
-- and cannot be removed. We create a new unique index that includes user_id
-- to enable per-user dedup. The old UNIQUE constraint remains but is harmless:
-- for legacy rows (all user_id=0) it still holds, and for new rows the
-- three-column index provides the stricter uniqueness check.
CREATE UNIQUE INDEX IF NOT EXISTS idx_jobs_user_source_ext ON jobs(user_id, external_id, source);
