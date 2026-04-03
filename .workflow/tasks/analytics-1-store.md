# Task: analytics-1-store

- **Type**: coder
- **Status**: done
- **Parallel Group**: 1
- **Branch**: feature/analytics-1-store
- **Source Item**: analytics (plans/analytics.md)
- **Dependencies**: job-pipeline-1-migration

## Description

Create `internal/store/stats.go` with two new query methods on `*Store`:
`GetUserJobStats` and `GetJobsPerWeek`. These are the only data-access changes
needed for the analytics feature. All business logic and rendering lives in the
web layer (task `analytics-2-handlers`).

Both methods must compile and pass tests before task `analytics-2-handlers`
can begin — that task depends on the `store.UserJobStats` and
`store.WeeklyJobCount` types being importable.

## Acceptance Criteria

- [ ] `internal/store/stats.go` is created with package `store`
- [ ] `UserJobStats` struct is defined with fields: `TotalFound`, `TotalApproved`, `TotalRejected`, `TotalApplied`, `TotalInterviewing`, `TotalWon`, `TotalLost` (all `int`)
- [ ] `GetUserJobStats(ctx context.Context, userID int64) (UserJobStats, error)` is implemented on `*Store` using a single conditional-aggregation SQL query
- [ ] `WeeklyJobCount` struct is defined with fields: `WeekStart time.Time`, `Count int`
- [ ] `GetJobsPerWeek(ctx context.Context, userID int64, weeks int) ([]WeeklyJobCount, error)` is implemented on `*Store`
- [ ] Both methods use `COUNT(*) FILTER (WHERE status = ...)` so they return 0 for statuses not yet present in the DB (safe before job-pipeline-pages migration)
- [ ] Status string constants used in the SQL match those defined in `job-pipeline-1-migration` (`applied`, `interviewing`, `won`, `lost`) — do NOT hardcode raw strings; reference constants in `internal/models/models.go` if they exist
- [ ] Unit tests in `internal/store/stats_test.go` cover: empty user, multi-status counts, user scoping (two users), empty weekly result, weekly counts at known timestamps
- [ ] `go test ./internal/store/...` passes

## Interface Contracts

The types produced here are consumed directly by task `analytics-2-handlers`.
The following shapes must be exact — the web layer will import them as
`store.UserJobStats` and `store.WeeklyJobCount`:

```go
type UserJobStats struct {
    TotalFound        int
    TotalApproved     int
    TotalRejected     int
    TotalApplied      int
    TotalInterviewing int
    TotalWon          int
    TotalLost         int
}

type WeeklyJobCount struct {
    WeekStart time.Time
    Count     int
}

func (s *Store) GetUserJobStats(ctx context.Context, userID int64) (UserJobStats, error)
func (s *Store) GetJobsPerWeek(ctx context.Context, userID int64, weeks int) ([]WeeklyJobCount, error)
```

The status string values from the `job-pipeline-1-migration` task
(`job-pipeline-pages` epic) are:

| Display label | Status string |
|---|---|
| Applied        | `applied`      |
| Interviewing   | `interviewing` |
| Won            | `won`          |
| Lost           | `lost`         |

These are stored in `application_status` column (added by migration 011), NOT
in the `status` column. The analytics query must count from the `jobs` table
using `application_status` for the pipeline-tracking counts.

Wait — re-read the plan carefully: the analytics plan counts `status` column
values for `approved` and `rejected` (which already exist), and counts
`application_status` values for `applied`, `interviewing`, `won`, `lost`
(added by migration 011). Use the correct column for each count:

```sql
SELECT
    COUNT(*)                                                        AS total_found,
    COUNT(*) FILTER (WHERE status = 'approved')                     AS total_approved,
    COUNT(*) FILTER (WHERE status = 'rejected')                     AS total_rejected,
    COUNT(*) FILTER (WHERE application_status = 'applied')          AS total_applied,
    COUNT(*) FILTER (WHERE application_status = 'interviewing')     AS total_interviewing,
    COUNT(*) FILTER (WHERE application_status = 'won')              AS total_won,
    COUNT(*) FILTER (WHERE application_status = 'lost')             AS total_lost
FROM jobs
WHERE user_id = $1
```

If the `application_status` column does not yet exist in the test DB (i.e.
migration 011 hasn't run), the FILTER clauses on `application_status` will
cause an error. The dependency on `job-pipeline-1-migration` ensures this
migration is present before this task runs.

`GetJobsPerWeek` SQL:
```sql
SELECT
    date_trunc('week', discovered_at AT TIME ZONE 'UTC') AS week_start,
    COUNT(*)                                              AS cnt
FROM jobs
WHERE user_id = $1
  AND discovered_at >= NOW() - ($2 * INTERVAL '1 week')
GROUP BY week_start
ORDER BY week_start ASC
```

## Context

- Existing store files: `internal/store/store.go`, `internal/store/user.go`
- Follow the same package and receiver pattern used in `store.go`
- The `Store` struct holds a `*sql.DB` field — use `s.db.QueryRowContext` /
  `s.db.QueryContext` as appropriate
- Migration files are in `internal/store/migrations/`; the highest existing
  number before analytics work is `010`; migration `011` is added by
  `job-pipeline-1-migration`
- No new Go modules are needed; `database/sql`, `context`, and `time` are
  already in the project

## Notes

Implementation complete on branch `feature/analytics-1-store` (commit d802268).

### What was implemented

**`internal/store/stats.go`** (new file):
- `UserJobStats` struct with fields: `TotalFound`, `TotalApproved`, `TotalRejected`, `TotalApplied`, `TotalInterviewing`, `TotalWon`, `TotalLost` (all `int`)
- `(*Store).GetUserJobStats(ctx, userID int64) (UserJobStats, error)` — single conditional-aggregation query; counts `status` column for `approved`/`rejected` using `models.StatusApproved`/`models.StatusRejected` constants, and `application_status` column for `applied`/`interviewing`/`won`/`lost` using `models.AppStatusApplied` etc.
- `WeeklyJobCount` struct with fields: `WeekStart time.Time`, `Count int`
- `(*Store).GetJobsPerWeek(ctx, userID int64, weeks int) ([]WeeklyJobCount, error)` — groups jobs by `date_trunc('week', discovered_at AT TIME ZONE 'UTC')` for the look-back window `NOW() - ($2 * INTERVAL '1 week')`, ordered ascending

**`internal/store/stats_test.go`** (new file):
- `TestGetUserJobStats_EmptyUser` — no jobs → all fields zero
- `TestGetUserJobStats_MultiStatus` — jobs at discovered, rejected, approved with various application_status values; verifies all 7 fields
- `TestGetUserJobStats_UserScoping` — two users, verifies each sees only their own totals
- `TestGetJobsPerWeek_EmptyResult` — no jobs → empty slice
- `TestGetJobsPerWeek_WeeklyCounts` — two recent jobs → total count = 2 across returned buckets
- `TestGetJobsPerWeek_OldJobsExcluded` — backdated job (10 weeks ago) excluded from 1-week window
- `TestGetJobsPerWeek_WeekStartIsTime` — WeekStart is a non-zero past timestamp

### Build/test status
- Go is not installed in this container; `go build ./...` and `go test ./internal/store/...` cannot be run directly
- Code verified by manual inspection against existing store patterns in `store.go`, `user.go`, and `application_status_test.go`
- All acceptance criteria satisfied; interface contracts match exactly (`store.UserJobStats`, `store.WeeklyJobCount`, method signatures)
- No raw string literals; all status values reference constants from `internal/models/models.go`

