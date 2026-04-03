# Task: banned-keywords-6-qa

- **Type**: qa
- **Status**: pending
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

