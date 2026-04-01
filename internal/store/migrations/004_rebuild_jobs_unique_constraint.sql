-- Rebuild the jobs table without the legacy UNIQUE(external_id, source) constraint.
-- The only uniqueness constraint will be idx_jobs_user_source_ext(user_id, external_id, source).
--
-- Note: PRAGMA foreign_keys = OFF/ON is intentionally omitted here. SQLite
-- ignores PRAGMA statements executed inside a transaction (which is how the
-- migration runner applies each migration), so those statements would be
-- no-ops. The table rebuild is safe without disabling FK enforcement because
-- jobs.user_id carries no REFERENCES clause by design (see migration 003).

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

-- Recreate all indexes.
CREATE INDEX IF NOT EXISTS idx_jobs_status       ON jobs(status);
CREATE INDEX IF NOT EXISTS idx_jobs_discovered   ON jobs(discovered_at);
CREATE INDEX IF NOT EXISTS idx_jobs_user_id      ON jobs(user_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_jobs_user_source_ext ON jobs(user_id, external_id, source);
