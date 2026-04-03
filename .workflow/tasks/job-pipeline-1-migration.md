# Task: job-pipeline-1-migration

- **Type**: coder
- **Status**: done
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

Implementation complete on branch `feature/job-pipeline-1-migration`.

- Created `internal/store/migrations/011_add_application_status.sql` with the
  exact SQL from the task spec: five ADD COLUMN IF NOT EXISTS statements and
  a CREATE INDEX IF NOT EXISTS on (user_id, application_status) WHERE
  application_status IS NOT NULL.
- Updated `internal/store/migrate_test.go`: extended the expected migration
  list to include 010 and 011, and added three new test subtests covering
  column usability, CHECK constraint rejection, and NULL-ability.
- Go and PostgreSQL are not available in this container, so tests could not
  be executed. The SQL and test code were validated by inspection against
  existing migration patterns.

---

## Code Review Findings

**Reviewer**: Code Reviewer agent
**Date**: 2026-04-03
**Verdict**: APPROVE

### Summary: 0 critical, 1 warning, 2 info

---

### [WARNING] internal/store/migrate_test.go:107-139 — New test subtests not idempotent on repeated runs

The three new subtests (`adds application_status columns to jobs`,
`application_status CHECK constraint rejects invalid values`,
`application_status columns are nullable`) all perform plain INSERT statements
with fixed `external_id` values (`'mig-appstatus'`, `'mig-badstatus'`,
`'mig-nullstatus'`) against a shared persistent PostgreSQL database and do not
use `ON CONFLICT DO NOTHING`. If the test suite is run twice against the same
DB without truncation, the second run will hit a unique constraint violation on
`(user_id, external_id, source)` and fail with `ERROR: duplicate key value
violates unique constraint`.

This is not a new pattern — the existing `adds user_id to jobs` subtest at
line 97 has the same issue with `external_id='mig-test'`. The new tests
faithfully follow the established convention, so this is an inherited weakness
rather than a new defect.

Suggested fix: Add `ON CONFLICT (user_id, external_id, source) DO NOTHING` to
all four job-INSERT subtests (existing and new) so they are safe to run
repeatedly. Or use randomised `external_id` values via `fmt.Sprintf`.

---

### [INFO] plans/job-pipeline-pages.md:135-137 vs 446-448 — Plan has two conflicting index definitions

The plan's Data Model section (line 136) defines the index without a WHERE clause:
`ON jobs(user_id, application_status)`. The implementation step 1 (line 447)
defines it with `WHERE application_status IS NOT NULL`. The migration correctly
implements the WHERE-clause (partial index) version from Step 1, which is the
better choice — it avoids indexing the majority of rows where
`application_status IS NULL`. No code change needed; the discrepancy is in the
plan document only.

---

### [INFO] 011_add_application_status.sql — No CHECK constraint name

The CHECK constraint on `application_status` is defined inline without an
explicit name:
```sql
CHECK(application_status IN ('applied','interviewing','lost','won'))
```
PostgreSQL will auto-generate a name like `jobs_application_status_check`. If a
future migration needs to drop or alter this constraint, the generated name must
be discovered at runtime. A named constraint (e.g.
`CONSTRAINT chk_application_status CHECK(...)`) would make future migrations
self-documenting. This is a minor style point and does not affect correctness.

---

### Positive findings

- **DDL correctness**: All five columns (`application_status TEXT`,
  `applied_at TIMESTAMPTZ`, `interviewing_at TIMESTAMPTZ`, `lost_at TIMESTAMPTZ`,
  `won_at TIMESTAMPTZ`) are nullable with no DEFAULT, matching the acceptance
  criteria and the interface contract exactly.
- **CHECK values**: `'applied','interviewing','lost','won'` match the plan and
  the interface contract exactly.
- **ADD COLUMN IF NOT EXISTS**: Used on all five columns — migration is
  re-runnable.
- **CREATE INDEX IF NOT EXISTS**: Present — migration is re-runnable.
- **Index definition**: `ON jobs(user_id, application_status) WHERE
  application_status IS NOT NULL` is a correct partial index; will efficiently
  support queries filtering by application status.
- **Migration numbering**: File is named `011_add_application_status.sql`,
  sequential after `010_add_banned_at_to_users.sql`.
- **migrate_test.go expected list**: Both `010_add_banned_at_to_users.sql` and
  `011_add_application_status.sql` correctly added — also resolves BUG-022.
- **Transaction safety**: Migration runs through `runMigration()` which wraps
  execution in a `BEGIN`/`COMMIT` transaction. `ALTER TABLE` and
  `CREATE INDEX` are both transactional in PostgreSQL — safe.
- **Embed**: Migration file is picked up automatically by `//go:embed migrations/*.sql`
  in `migrate.go` — no registration required.

**Overall verdict**: APPROVE. The SQL DDL is correct, complete, and matches the
plan and interface contracts. The one warning (non-idempotent test INSERTs) is
inherited from the pre-existing test pattern and does not block merging.
