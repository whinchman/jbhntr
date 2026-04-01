-- Add user_id column. Default 0 means "legacy/unassigned" (pre-migration data).
-- No REFERENCES users(id) foreign key: legacy rows have user_id=0 with no
-- corresponding user row, which would violate PRAGMA foreign_keys=ON.
-- Enforce the relationship at the application level instead.
ALTER TABLE jobs ADD COLUMN user_id INTEGER NOT NULL DEFAULT 0;
CREATE INDEX IF NOT EXISTS idx_jobs_user_id ON jobs(user_id);

-- SQLite does not support ALTER TABLE ... DROP CONSTRAINT or ADD CONSTRAINT.
-- The original UNIQUE(external_id, source) is baked into the table definition
-- and cannot be removed in-place. We create a new unique index that includes
-- user_id to enable per-user dedup. The old UNIQUE constraint remains in
-- effect until migration 004 rebuilds the table and removes it, and it can
-- still block some inserts (two different users cannot share the same
-- external_id+source pair until 004 runs). The three-column index provides
-- the correct per-user uniqueness semantics; running migration 004 is required
-- to fully fix BUG-001 (legacy UNIQUE constraint blocks per-user dedup).
CREATE UNIQUE INDEX IF NOT EXISTS idx_jobs_user_source_ext ON jobs(user_id, external_id, source);
