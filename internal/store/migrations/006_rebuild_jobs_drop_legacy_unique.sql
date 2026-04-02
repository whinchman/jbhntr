-- Migration 006: On Postgres there is no legacy UNIQUE(external_id, source)
-- two-column constraint to drop — the baseline schema never had one and
-- migration 003 created only the correct three-column unique index
-- (user_id, external_id, source). This migration is a no-op on Postgres.
SELECT 1;
