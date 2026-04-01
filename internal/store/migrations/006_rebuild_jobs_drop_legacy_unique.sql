-- Migration 006: ensure the legacy UNIQUE(external_id, source) two-column
-- constraint is absent from the jobs table.
--
-- Context: the original baseline schema (store.go) contained an implicit
-- UNIQUE(external_id, source) constraint baked into the CREATE TABLE
-- definition. Migration 003 added the correct per-user three-column index
-- UNIQUE(user_id, external_id, source), but SQLite does not support
-- ALTER TABLE ... DROP CONSTRAINT, so the old constraint remained in effect.
-- With INSERT OR IGNORE, the old two-column constraint fires first, silently
-- dropping inserts for any second user who discovers the same job listing.
--
-- Migration 004 was a first attempt at this rebuild, introduced alongside
-- the multi-user scheduler feature. This migration (006) is the authoritative
-- fix for BUG-001, ensuring the correct constraint is present on all
-- database instances regardless of their upgrade path.
--
-- The standard SQLite table-rebuild pattern is used:
--   1. Create replacement table (no legacy UNIQUE constraint)
--   2. Copy all rows
--   3. Drop original table
--   4. Rename replacement table
--   5. Recreate indexes
--
-- No PRAGMA foreign_keys is included here: the pragma is a no-op inside a
-- transaction (which is how migrate.go executes migrations), and no foreign
-- keys reference the jobs table by design (see migration 003 comments).

CREATE TABLE jobs_new (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id          INTEGER NOT NULL DEFAULT 0,
    external_id      TEXT    NOT NULL,
    source           TEXT    NOT NULL,
    title            TEXT    NOT NULL DEFAULT '',
    company          TEXT    NOT NULL DEFAULT '',
    location         TEXT    NOT NULL DEFAULT '',
    description      TEXT    NOT NULL DEFAULT '',
    salary           TEXT    NOT NULL DEFAULT '',
    apply_url        TEXT    NOT NULL DEFAULT '',
    status           TEXT    NOT NULL DEFAULT 'discovered'
                     CHECK(status IN ('discovered','notified','approved','rejected','generating','complete','failed')),
    summary          TEXT    NOT NULL DEFAULT '',
    extracted_salary TEXT    NOT NULL DEFAULT '',
    resume_html      TEXT    NOT NULL DEFAULT '',
    cover_html       TEXT    NOT NULL DEFAULT '',
    resume_pdf       TEXT    NOT NULL DEFAULT '',
    cover_pdf        TEXT    NOT NULL DEFAULT '',
    error_msg        TEXT    NOT NULL DEFAULT '',
    discovered_at    DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
    updated_at       DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);

INSERT INTO jobs_new SELECT id, user_id, external_id, source, title, company, location, description, salary, apply_url, status, summary, extracted_salary, resume_html, cover_html, resume_pdf, cover_pdf, error_msg, discovered_at, updated_at FROM jobs;

DROP TABLE jobs;

ALTER TABLE jobs_new RENAME TO jobs;

-- Recreate all indexes that should exist on the jobs table.
CREATE INDEX IF NOT EXISTS idx_jobs_status       ON jobs(status);
CREATE INDEX IF NOT EXISTS idx_jobs_discovered   ON jobs(discovered_at);
CREATE INDEX IF NOT EXISTS idx_jobs_user_id      ON jobs(user_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_jobs_user_source_ext ON jobs(user_id, external_id, source);
