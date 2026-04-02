-- Migration 004: On Postgres the baseline jobs table has no legacy
-- UNIQUE(external_id, source) two-column constraint, so no table rebuild
-- is required. The correct per-user index idx_jobs_user_source_ext was
-- already created in migration 003. This migration is a no-op on Postgres.
SELECT 1;
