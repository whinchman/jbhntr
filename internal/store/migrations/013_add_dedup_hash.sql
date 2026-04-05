-- Migration 013: add dedup_hash column and partial unique index for cross-source deduplication.

ALTER TABLE jobs ADD COLUMN IF NOT EXISTS dedup_hash TEXT NOT NULL DEFAULT '';

CREATE UNIQUE INDEX IF NOT EXISTS idx_jobs_user_dedup ON jobs(user_id, dedup_hash) WHERE dedup_hash != '';
