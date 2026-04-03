# Task: job-pipeline-1-migration

- **Type**: coder
- **Status**: pending
- **Parallel Group**: 1
- **Branch**: feature/job-pipeline-1-migration
- **Source Item**: job-pipeline-pages (plans/job-pipeline-pages.md)
- **Dependencies**: none

## Description

Create the PostgreSQL migration `011_add_application_status.sql` that adds
application-status tracking columns to the `jobs` table. This is a purely
additive migration — all new columns are nullable and have no default, so
there is no breaking change to existing rows.

## Acceptance Criteria

- [ ] File `internal/store/migrations/011_add_application_status.sql` exists
- [ ] Migration adds `application_status TEXT CHECK(...)` with allowed values `applied`, `interviewing`, `lost`, `won`
- [ ] Migration adds `applied_at TIMESTAMPTZ`, `interviewing_at TIMESTAMPTZ`, `lost_at TIMESTAMPTZ`, `won_at TIMESTAMPTZ` — all nullable
- [ ] Migration creates index `idx_jobs_application_status ON jobs(user_id, application_status) WHERE application_status IS NOT NULL`
- [ ] Migration uses `ADD COLUMN IF NOT EXISTS` and `CREATE INDEX IF NOT EXISTS` so it is re-runnable safely
- [ ] Migration runs cleanly against an existing database (verified via `go test ./internal/store/... -run TestMigrate` or equivalent)

## Interface Contracts

None — this task only touches the database schema. The column names and CHECK
constraint values defined here must match exactly what the store layer (task
`job-pipeline-2-models-store`) will reference:

```
application_status  TEXT  CHECK(application_status IN ('applied','interviewing','lost','won'))
applied_at          TIMESTAMPTZ  nullable
interviewing_at     TIMESTAMPTZ  nullable
lost_at             TIMESTAMPTZ  nullable
won_at              TIMESTAMPTZ  nullable
```

## Context

- Existing migrations live in `internal/store/migrations/`. The highest
  existing number is `010_add_banned_at_to_users.sql`. Name this file
  `011_add_application_status.sql`.
- The migration framework auto-discovers files by numeric prefix; follow
  the same naming pattern.
- Full SQL from the plan:

```sql
ALTER TABLE jobs
  ADD COLUMN IF NOT EXISTS application_status TEXT
      CHECK(application_status IN ('applied','interviewing','lost','won')),
  ADD COLUMN IF NOT EXISTS applied_at        TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS interviewing_at   TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS lost_at           TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS won_at            TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_jobs_application_status
    ON jobs(user_id, application_status)
    WHERE application_status IS NOT NULL;
```

## Notes

