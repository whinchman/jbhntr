# Task: job-pipeline-2-models-store

- **Type**: coder
- **Status**: pending
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

