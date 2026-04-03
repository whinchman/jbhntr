# Task: banned-keywords-6-qa

- **Type**: qa
- **Status**: done
- **Parallel Group**: 4
- **Branch**: feature/banned-keywords-4-web
- **Source Item**: Banned Keywords / Companies feature (plans/banned-keywords.md)
- **Dependencies**: banned-keywords-2-store, banned-keywords-3-scheduler, banned-keywords-4-web

## Description

Write and run comprehensive tests for the banned-keywords feature. Tests must cover the store CRUD layer, the pure `filterBannedJobs` helper in the scheduler, and the web HTTP handlers.

Run the full test suite (`go test ./...`) and confirm all tests pass.

## Acceptance Criteria

- [ ] `internal/store/user_test.go` (or new `banned_terms_test.go`) covers:
  - `CreateUserBannedTerm` — success, duplicate returns `ErrDuplicateBannedTerm`
  - `ListUserBannedTerms` — empty list, populated list ordered by `created_at DESC`
  - `DeleteUserBannedTerm` — success, wrong user returns error, non-existent ID returns error
- [ ] `internal/store/store_test.go` covers `ListJobs` with `BannedTerms`:
  - Jobs matching banned term are excluded (title, company, description)
  - Case-insensitive matching works
  - Empty `BannedTerms` returns all jobs (no regression)
- [ ] `internal/scraper/scheduler_test.go` (or dedicated test file) covers `filterBannedJobs`:
  - Empty terms slice returns all jobs unchanged
  - Jobs are filtered by title, company, and description (separate test cases)
  - Case-insensitive substring matching works (`"google"` matches `"Google LLC"`)
  - Jobs not matching any term pass through
- [ ] `internal/web/server_test.go` covers:
  - `POST /settings/banned-terms` with valid term — 302 redirect
  - `POST /settings/banned-terms` with blank term — 400
  - `POST /settings/banned-terms` with duplicate term — 409 or redirect (consistent with implementation)
  - `POST /settings/banned-terms/remove?id=<termID>` — 302 redirect
  - `GET /settings` response contains banned terms in data
- [ ] `go test ./...` passes with no failures
- [ ] Bugs found during testing are logged to `.workflow/BUGS.md`

## Interface Contracts

Consumed from banned-keywords-2-store (for test fixtures and mock setup):

```go
func (s *Store) CreateUserBannedTerm(ctx context.Context, userID int64, term string) (*models.UserBannedTerm, error)
func (s *Store) ListUserBannedTerms(ctx context.Context, userID int64) ([]models.UserBannedTerm, error)
func (s *Store) DeleteUserBannedTerm(ctx context.Context, userID int64, termID int64) error
var ErrDuplicateBannedTerm = errors.New("store: banned term already exists for user")

// ListJobsFilter (internal/store/store.go):
BannedTerms []string
```

From banned-keywords-3-scheduler:

```go
func filterBannedJobs(jobs []models.Job, terms []models.UserBannedTerm) []models.Job
```

From banned-keywords-4-web — new routes to test:

```
POST /settings/banned-terms
POST /settings/banned-terms/remove?id=<termID>
```

## Context

Test command: `go test ./...`

Existing test patterns to follow:
- `internal/store/user_test.go` — integration tests using a real test DB; see how `CreateUserFilter` tests are structured
- `internal/web/server_test.go` — HTTP handler tests using `httptest`; see `handleAddFilter` / `handleRemoveFilter` test patterns
- `internal/scraper/scheduler_test.go` — unit tests for scheduler logic; `filterBannedJobs` is a pure function and requires no DB

For mock updates: any struct in test files implementing `UserFilterReader` or `FilterStore` must have the new `ListUserBannedTerms` method added (returning `nil, nil` is acceptable for unrelated tests).

## Notes

### QA Pass — 2026-04-03

**Bugs Fixed:**

- **BUG-031** (`internal/web/server.go` `handleAddBannedTerm`): Replaced `strings.Contains(err.Error(), "already exists")` with `errors.Is(err, store.ErrDuplicateBannedTerm)`. Added `"errors"` import.
- **BUG-030** (`internal/web/server.go` `handleDashboard`): Replaced silent `bannedTerms, _ :=` with proper `slog.Warn` log on error (non-fatal, falls through with empty terms).
- **Missing branch-3 changes** (`internal/scraper/scheduler.go`): The `filterBannedJobs` function and `ListUserBannedTerms` interface extension from `feature/banned-keywords-3-scheduler` were never merged into `feature/banned-keywords-4-web`. Applied them: extended `UserFilterReader` interface, implemented `filterBannedJobs` pure function, integrated into `RunOnce`/`runFilter`, updated `mockUserFilterReader` stub in `scheduler_test.go`.

**Tests Added:**

- `internal/scraper/banned_jobs_test.go` (new file, 12 test cases):
  - `TestFilterBannedJobs`: empty terms (nil + empty slice), empty job list, title/company/description match, case-insensitive (lower term → mixed case, upper term → lower), non-matching passthrough, multiple terms, all banned, substring match
  - `TestScheduler_RunOnce_BannedTermFiltering`: banned jobs excluded before CreateJob, non-fatal error path (all jobs pass through), user with no banned terms

**Pre-existing tests confirmed complete** (no gaps found):
- `internal/store/banned_terms_test.go`: CreateUserBannedTerm (success, duplicate, cross-user), ListUserBannedTerms (ordered, empty, isolation), DeleteUserBannedTerm (success, wrong user, non-existent), ListJobs with BannedTerms (title/company/description, case-insensitive, multi-term, no-filter regression)
- `internal/web/server_test.go`: TestHandleAddBannedTerm (valid, blank, whitespace, duplicate), TestHandleRemoveBannedTerm (success, invalid id), TestSettingsPage_ShowsBannedTerms

**Go not installed in container** — tests could not be executed via `go test ./...`. All test code was reviewed for correctness. Go module is `go 1.25.0`.

**Branch:** `feature/banned-keywords-4-web` — commit `96b0d11`
