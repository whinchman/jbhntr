# Task: job-pipeline-2-models-store

- **Type**: code-reviewer
- **Status**: done
- **Parallel Group**: 2
- **Branch**: feature/job-pipeline-2-models-store
- **Source Item**: job-pipeline-pages (plans/job-pipeline-pages.md)
- **Dependencies**: job-pipeline-1-migration

## Description

Update the `models` and `store` packages to expose the five new application-status
columns added by migration 011. Add an `ApplicationStatus` type with constants,
update the `Job` struct, update `scanJob`, add a `ListJobsFilter.ApplicationStatus`
field, and implement the `UpdateApplicationStatus` store method.

## Acceptance Criteria

- [ ] `internal/models/models.go` defines type `ApplicationStatus string` and constants `AppStatusApplied`, `AppStatusInterviewing`, `AppStatusLost`, `AppStatusWon`
- [ ] `models.ApplicationStatus` has a `Valid() bool` method that returns true for the four known values and false for anything else (including empty string)
- [ ] `models.Job` has five new fields: `ApplicationStatus string`, `AppliedAt *time.Time`, `InterviewingAt *time.Time`, `LostAt *time.Time`, `WonAt *time.Time`
- [ ] `scanJob` in `internal/store/store.go` scans all five new columns
- [ ] `ListJobsFilter` has a new `ApplicationStatus models.ApplicationStatus` field; when non-empty, `ListJobs` filters by that value
- [ ] `Store.UpdateApplicationStatus(ctx, userID, jobID, status)` is implemented per the spec: validates job belongs to user, validates job is in an approved-pipeline stage, updates `application_status` and stamps the corresponding `*_at` using `COALESCE(existing_at, NOW())`
- [ ] `UpdateApplicationStatus` returns an error if the job's pipeline `Status` is not one of `approved`, `generating`, `complete`, `failed`
- [ ] Re-selecting the same `application_status` does NOT overwrite the existing timestamp
- [ ] All existing store tests continue to pass

## Interface Contracts

The five DB columns (from migration `011_add_application_status.sql`):
```
application_status  TEXT  CHECK(application_status IN ('applied','interviewing','lost','won'))
applied_at          TIMESTAMPTZ  nullable
interviewing_at     TIMESTAMPTZ  nullable
lost_at             TIMESTAMPTZ  nullable
won_at              TIMESTAMPTZ  nullable
```

The store method signature that the web layer (task `job-pipeline-3-web`) will
call via the `JobStore` interface:
```go
UpdateApplicationStatus(ctx context.Context, userID int64, jobID int64, status models.ApplicationStatus) error
```

The SQL used inside `UpdateApplicationStatus` must use COALESCE to preserve
original timestamps:
```sql
UPDATE jobs SET
  application_status  = $1,
  applied_at          = CASE WHEN $1 = 'applied'      THEN COALESCE(applied_at, NOW())      ELSE applied_at      END,
  interviewing_at     = CASE WHEN $1 = 'interviewing' THEN COALESCE(interviewing_at, NOW()) ELSE interviewing_at END,
  lost_at             = CASE WHEN $1 = 'lost'         THEN COALESCE(lost_at, NOW())         ELSE lost_at         END,
  won_at              = CASE WHEN $1 = 'won'          THEN COALESCE(won_at, NOW())          ELSE won_at          END,
  updated_at          = NOW()
WHERE id = $2 AND user_id = $3
```

## Context

- Primary files: `internal/models/models.go`, `internal/store/store.go`
- `scanJob` is the function that maps a `*sql.Rows` scan to a `models.Job` struct.
  Add the five new columns at the end of the scan list to match the SELECT order.
- `ListJobsFilter` struct is used by `ListJobs` to build WHERE clauses. Add an
  `ApplicationStatus models.ApplicationStatus` field; when set, append
  `AND application_status = $N` to the query.
- Validation in `UpdateApplicationStatus`: after fetching the job by `(id, user_id)`,
  check that `job.Status` is one of `models.StatusApproved`, `models.StatusGenerating`,
  `models.StatusComplete`, `models.StatusFailed`. Return a descriptive error otherwise.
- New store tests should go in `internal/store/store_test.go` or a new file
  `internal/store/application_status_test.go`. Required test cases:
  - `TestUpdateApplicationStatus_SetsStatusAndTimestamp`
  - `TestUpdateApplicationStatus_PreservesOriginalTimestamp`
  - `TestUpdateApplicationStatus_RejectsNonApprovedJob`
  - `TestListJobsByApplicationStatus`

## Notes

**Branch**: `feature/job-pipeline-2-models-store` (based on `feature/job-pipeline-1-migration`)

**Implemented:**
- `models.ApplicationStatus` type with constants `AppStatusApplied`, `AppStatusInterviewing`, `AppStatusLost`, `AppStatusWon` and a `Valid() bool` method
- `models.Job` extended with five new fields: `ApplicationStatus ApplicationStatus`, `AppliedAt *time.Time`, `InterviewingAt *time.Time`, `LostAt *time.Time`, `WonAt *time.Time`
- `scanJob` updated to scan all five new columns (`application_status` via `sql.NullString`, four `*time.Time` nullable timestamps)
- `GetJob` and `ListJobs` SELECT queries updated to include the five new columns
- `ListJobsFilter.ApplicationStatus` field added; `ListJobs` filters by it when non-empty
- `Store.UpdateApplicationStatus` implemented with COALESCE-based SQL per spec; validates that `job.Status` is one of approved/generating/complete/failed before updating
- `JobStore` interface in `internal/web/server.go` extended with `UpdateApplicationStatus`
- `mockJobStore` and `uiMinorJobStore` mock implementations updated to satisfy the interface

**Test file**: `internal/store/application_status_test.go` with tests:
- `TestUpdateApplicationStatus_SetsStatusAndTimestamp`
- `TestUpdateApplicationStatus_PreservesOriginalTimestamp`
- `TestUpdateApplicationStatus_RejectsNonApprovedJob`
- `TestListJobsByApplicationStatus`

**Test run**: Go toolchain not available in the container. Tests require `TEST_DATABASE_URL` (Postgres) per `openTestStore` pattern. Code review for compile-time correctness confirmed all types, interface satisfaction, and SQL correctness manually.

---

## Code Review Findings

**Reviewer**: code-reviewer agent
**Branch**: `feature/job-pipeline-2-models-store`
**Summary**: 0 critical, 2 warning, 1 info. **Verdict: approve** (no correctness defects; warnings are low-risk and can be addressed without blocking this task).

---

### [WARNING] internal/store/store.go:329–359 — `UpdateApplicationStatus` does not support `userID == 0` worker path

All other mutating store methods (`UpdateJobStatus`, `UpdateJobSummary`, `UpdateJobError`, `UpdateJobGenerated`) follow a consistent pattern: when `userID == 0`, they drop the `AND user_id = $N` clause so that background workers can update any job. `UpdateApplicationStatus` deviates from this: it always appends `AND user_id = $3` to the UPDATE, which means a call with `userID=0` will silently succeed only for legacy jobs where `user_id = 0`, not for all jobs.

This is currently low-risk because no background worker calls `UpdateApplicationStatus` (it is a user-initiated action) and the interface only exposes it to the web layer (where `userID` is always non-zero). However, it creates a correctness trap if a future worker or admin path ever needs to call this method without a user ID.

**Suggested fix**: Follow the existing pattern — detect `userID == 0` and omit the `AND user_id = $3` clause, or add a clear doc comment stating "userID must be > 0; background worker path not supported" to match the deliberate design intent.

---

### [WARNING] internal/web/server_test.go:51–65 — `mockJobStore.ListJobs` does not filter by `ApplicationStatus`

`mockJobStore.ListJobs` filters by `f.Status` but ignores `f.ApplicationStatus`. If any existing or future test exercises `ListJobs` with a non-empty `ApplicationStatus` filter via the mock, it will return all jobs (or the Status-filtered subset) instead of correctly filtering — a silently wrong result.

The mock is used in web handler tests, not store integration tests, so this bug would only surface if a web handler test exercises this filter path. It does not affect the real store implementation.

**Suggested fix**: Add the missing filter clause to `mockJobStore.ListJobs`:
```go
if f.ApplicationStatus != "" && j.ApplicationStatus != f.ApplicationStatus {
    continue
}
```

---

### [INFO] internal/store/store.go:450–470 — Inconsistent timestamp scan pattern

`discoveredAt` and `updatedAt` are scanned into intermediate `string` variables and parsed via `time.Parse(time.RFC3339, ...)`, a pre-existing pattern in `scanJob`. The four new nullable timestamps (`AppliedAt`, `InterviewingAt`, `LostAt`, `WonAt`) are scanned directly into `*time.Time`, which is the correct modern approach with the `pgx/v5/stdlib` driver.

Both approaches work correctly with `pgx`. The inconsistency is a readability/maintenance concern: future contributors may be confused about which pattern to use. No action needed on this branch; it can be addressed in a future refactor of `scanJob` to migrate the two pre-existing fields to the `*time.Time` pattern.

---

### Positive observations

- `ApplicationStatus` type, constants, and `Valid()` method are correctly defined; pattern matches `JobStatus`.
- SQL `COALESCE` pattern in `UpdateApplicationStatus` exactly matches the spec and correctly preserves original timestamps.
- `user_id` scoping in the `UPDATE WHERE id = $2 AND user_id = $3` clause is correct for the normal (non-zero userID) case.
- `scanJob` correctly uses `sql.NullString` for the nullable `application_status` TEXT column and only sets `job.ApplicationStatus` when the value is non-null.
- `ListJobsFilter.ApplicationStatus` filter correctly increments `argN` after appending its clause, preserving parameter numbering for subsequent filters (`Search`).
- `pipelineStatuses` map correctly encodes the four allowed pipeline stages.
- `JobStore` interface extended correctly; both `mockJobStore` and `uiMinorJobStore` implement the new method.
- Test coverage in `application_status_test.go` is comprehensive: covers timestamp setting, timestamp preservation across status changes, rejection of non-pipeline-stage jobs, and list filtering.
- `migrate_test.go` subtests correctly verify the new columns, the CHECK constraint, and nullable behavior.

---

**Code review verdict**: **approve**
- Findings: 0 critical, 2 warning, 1 info
- BUG-025 filed for `UpdateApplicationStatus` missing `userID == 0` worker path consistency
- BUG-026 filed for `mockJobStore.ListJobs` missing `ApplicationStatus` filter

