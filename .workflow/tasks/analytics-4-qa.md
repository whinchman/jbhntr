# Task: analytics-4-qa

- **Type**: qa
- **Status**: pending
- **Parallel Group**: 3
- **Branch**: feature/analytics-2-handlers
- **Source Item**: analytics (plans/analytics.md)
- **Dependencies**: analytics-2-handlers

## Description

Write and run comprehensive tests for the analytics feature. This task extends
existing test files and creates new ones to cover the store layer, the HTTP
handler, and the HTML rendering.

Work on branch `feature/analytics-2-handlers` (the final integration branch
where both store and handler changes are present).

## Acceptance Criteria

### Store tests — `internal/store/stats_test.go`

- [ ] `TestGetUserJobStats_empty` — new user with no jobs returns all-zero `UserJobStats`
- [ ] `TestGetUserJobStats_counts` — insert jobs with known statuses; assert each counter in the returned struct matches
- [ ] `TestGetUserJobStats_userScoped` — two users with different job sets; assert each user only sees their own counts
- [ ] `TestGetJobsPerWeek_empty` — user with no jobs; method returns empty slice without error
- [ ] `TestGetJobsPerWeek_filledWeeks` — insert jobs at known `discovered_at` timestamps; assert returned counts match
- [ ] Store tests use the existing test DB helpers (follow patterns in `store_test.go` or `admin_test.go`)

### Integration tests — `internal/web/integration_test.go`

- [ ] `TestStatsPage_unauthenticated` — `GET /stats` without a session returns redirect to `/login` (3xx)
- [ ] `TestStatsPage_authenticated` — `GET /stats` with a valid session returns 200 and body contains `"Total Found"`
- [ ] `TestStatsPage_counts` — seed a known number of jobs for the test user; assert that the rendered HTML contains the expected count values

### Template / render tests

- [ ] Add a test in the existing `ui_minor_internal_test.go` or a new `stats_internal_test.go` that renders `statsData` with known values and asserts correct HTML snippets appear (e.g. stat values, "Approved", "Interviewing")

### Coverage

- [ ] `internal/store/stats.go` achieves ≥ 80% line coverage
- [ ] `go test ./...` passes with no failures

## Interface Contracts

The handler and store types to test:

```go
// store package
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
```

The `/stats` route requires authentication. Use the same test helpers that
`integration_test.go` already uses to create sessions (follow `TestApprovedJobsPage`
or similar existing authenticated tests as a pattern).

## Context

- Existing integration test file: `internal/web/integration_test.go`
- Existing store test helpers: `internal/store/store_test.go`, `internal/store/admin_test.go`
- Existing UI test patterns: `internal/web/ui_minor_internal_test.go`
- Test command: `go test ./...`
- The test DB is set up by `migrate.go` — use `testDB()` or the equivalent
  helper already present in the store tests
- If `application_status` column tests require migration 011 to be present,
  ensure the test DB is migrated fully before asserting counts for
  `applied`/`interviewing`/`won`/`lost`

## Notes

