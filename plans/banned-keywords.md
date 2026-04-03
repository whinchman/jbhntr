# Architecture Plan: Banned Keywords / Companies

**Feature**: Per-user "Banned Keywords / Companies" setting  
**Date**: 2026-04-03  
**Status**: Draft

---

## 1. Architecture Overview

Users need a list of terms (keywords or company names) that should suppress any
job listing whose title, company name, or description contains a match.

### Key Design Decisions

| Question | Decision | Rationale |
|----------|----------|-----------|
| Where to filter | **At scrape/storage time** (inside `Scheduler.runFilter`) | Prevents banned jobs from ever entering the DB; no retroactive cleanup needed; query-time filtering would require joining every `ListJobs` call against a variable-length list which is wasteful at scale |
| Storage location | **New table `user_banned_terms`** | Keeps the list variable-length without a text blob; fits the existing one-table-per-feature migration pattern; compatible with future admin visibility |
| Matching | **Case-insensitive substring (`ILIKE`)** for retroactive SQL check; Go `strings.EqualFold` + `strings.Contains` (lowercased) in the scheduler | Substring matching catches "Google LLC" and "Google" with one rule; case-insensitive avoids "Google" vs "google" mismatches |
| Retroactive filtering | On `ListJobs` add an optional SQL `NOT (title ILIKE any OR company ILIKE any)` clause via a helper | Handles jobs scraped before the ban was added; opt-in via a new `ListJobsFilter` field |
| Scope | Per-user only; no global banned list in this iteration | Keeps the feature self-contained |

### Data Flow

```
Scheduler.runFilter
  └─ fetches user's banned terms (new StoreReader method)
  └─ filters results[]models.Job before calling CreateJob
       → jobs matching a banned term are silently dropped

ListJobs (web dashboard)
  └─ ListJobsFilter.BannedTerms []string (populated from store)
  └─ SQL WHERE clause excludes matching rows
       → handles pre-ban jobs that slipped through
```

---

## 2. Database Schema

### New Table

Migration file: `internal/store/migrations/011_add_user_banned_terms.sql`

```sql
CREATE TABLE IF NOT EXISTS user_banned_terms (
    id         BIGSERIAL PRIMARY KEY,
    user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    term       TEXT   NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, term)
);
CREATE INDEX IF NOT EXISTS idx_user_banned_terms_user ON user_banned_terms(user_id);
```

### No changes to existing tables

`users`, `jobs`, `user_search_filters` are untouched.

---

## 3. Model Changes

### `internal/models/user.go`

Add a new model alongside `UserSearchFilter`:

```go
// UserBannedTerm is one entry in a user's banned-keywords list.
type UserBannedTerm struct {
    ID        int64
    UserID    int64
    Term      string
    CreatedAt time.Time
}
```

---

## 4. Store Changes

### `internal/store/user.go`

Add four new methods:

```go
// CreateUserBannedTerm inserts a banned term for the given user.
// Returns ErrDuplicateBannedTerm if the term already exists for this user.
func (s *Store) CreateUserBannedTerm(ctx context.Context, userID int64, term string) (*models.UserBannedTerm, error)

// ListUserBannedTerms returns all banned terms for the given user, ordered by created_at DESC.
func (s *Store) ListUserBannedTerms(ctx context.Context, userID int64) ([]models.UserBannedTerm, error)

// DeleteUserBannedTerm deletes a banned term by ID scoped to the given user.
// Returns an error if the term does not exist or does not belong to the user.
func (s *Store) DeleteUserBannedTerm(ctx context.Context, userID int64, termID int64) error

// scanUserBannedTerm scans one user_banned_terms row (unexported helper).
func scanUserBannedTerm(s scanner) (*models.UserBannedTerm, error)
```

Add a sentinel error in `user.go`:

```go
var ErrDuplicateBannedTerm = errors.New("store: banned term already exists for user")
```

Detect via the existing `isUniqueViolation` helper (already present in `user.go`).

### `internal/store/store.go`

Extend `ListJobsFilter`:

```go
// BannedTerms, when non-empty, excludes jobs whose title, company,
// or description contains any of the given terms (case-insensitive).
BannedTerms []string
```

Inside `ListJobs`, after the existing `WHERE` clause construction, add:

```go
for _, t := range f.BannedTerms {
    like := "%" + t + "%"
    where = append(where, fmt.Sprintf(
        "(title NOT ILIKE $%d AND company NOT ILIKE $%d AND description NOT ILIKE $%d)",
        argN, argN+1, argN+2))
    args = append(args, like, like, like)
    argN += 3
}
```

---

## 5. Scheduler Changes

### `internal/scraper/scheduler.go`

#### 5a. Extend `UserFilterReader` interface

```go
type UserFilterReader interface {
    ListActiveUserIDs(ctx context.Context) ([]int64, error)
    ListUserFilters(ctx context.Context, userID int64) ([]models.UserSearchFilter, error)
    // NEW:
    ListUserBannedTerms(ctx context.Context, userID int64) ([]models.UserBannedTerm, error)
}
```

#### 5b. Fetch banned terms once per user in `RunOnce`

In the per-user loop (after `ListUserFilters`), call:

```go
bannedTerms, err := s.userFilters.ListUserBannedTerms(ctx, userID)
if err != nil {
    s.logger.Error("failed to load banned terms", "user_id", userID, "error", err)
    bannedTerms = nil // non-fatal; proceed without filtering
}
```

Pass `bannedTerms` through to `runFilter`:

```go
jobs, err := s.runFilter(ctx, userID, ntfyTopic, filter, bannedTerms)
```

#### 5c. `runFilter` signature and filtering logic

```go
func (s *Scheduler) runFilter(
    ctx context.Context,
    userID int64,
    ntfyTopic string,
    filter models.SearchFilter,
    bannedTerms []models.UserBannedTerm,
) ([]models.Job, error)
```

After `results, searchErr := s.source.Search(...)`, add a filter pass before
the `CreateJob` loop:

```go
results = filterBannedJobs(results, bannedTerms)
```

New pure helper (no DB access, easy to unit-test):

```go
// filterBannedJobs removes any job whose title, company, or description
// contains a banned term (case-insensitive substring match).
func filterBannedJobs(jobs []models.Job, terms []models.UserBannedTerm) []models.Job {
    if len(terms) == 0 {
        return jobs
    }
    lower := make([]string, len(terms))
    for i, t := range terms {
        lower[i] = strings.ToLower(t.Term)
    }
    out := jobs[:0]
    for _, j := range jobs {
        titleL := strings.ToLower(j.Title)
        companyL := strings.ToLower(j.Company)
        descL := strings.ToLower(j.Description)
        banned := false
        for _, t := range lower {
            if strings.Contains(titleL, t) ||
                strings.Contains(companyL, t) ||
                strings.Contains(descL, t) {
                banned = true
                break
            }
        }
        if !banned {
            out = append(out, j)
        }
    }
    return out
}
```

---

## 6. Web Layer Changes

### `internal/web/server.go`

#### 6a. Extend `FilterStore` interface

```go
type FilterStore interface {
    // existing …
    CreateUserFilter(ctx context.Context, userID int64, filter *models.UserSearchFilter) error
    ListUserFilters(ctx context.Context, userID int64) ([]models.UserSearchFilter, error)
    DeleteUserFilter(ctx context.Context, userID int64, filterID int64) error
    UpdateUserResume(ctx context.Context, userID int64, markdown string) error
    UpdateUserNtfyTopic(ctx context.Context, userID int64, topic string) error
    // NEW:
    ListUserBannedTerms(ctx context.Context, userID int64) ([]models.UserBannedTerm, error)
    CreateUserBannedTerm(ctx context.Context, userID int64, term string) (*models.UserBannedTerm, error)
    DeleteUserBannedTerm(ctx context.Context, userID int64, termID int64) error
}
```

#### 6b. Extend `settingsData`

```go
type settingsData struct {
    Filters      []models.UserSearchFilter
    BannedTerms  []models.UserBannedTerm  // NEW
    Resume       string
    NtfyTopic    string
    Saved        bool
    CSRFToken    string
    User         *models.User
}
```

#### 6c. Update `handleSettings`

Load banned terms alongside filters:

```go
bannedTerms, err := s.filterStore.ListUserBannedTerms(r.Context(), userID)
if err != nil {
    slog.Error("failed to list banned terms", "error", err)
    http.Error(w, "failed to load settings", http.StatusInternalServerError)
    return
}
if bannedTerms == nil {
    bannedTerms = []models.UserBannedTerm{}
}
// add to settingsData
data.BannedTerms = bannedTerms
```

#### 6d. New HTTP handlers

```go
// POST /settings/banned-terms
func (s *Server) handleAddBannedTerm(w http.ResponseWriter, r *http.Request)
// POST /settings/banned-terms/remove?id=<termID>
func (s *Server) handleRemoveBannedTerm(w http.ResponseWriter, r *http.Request)
```

Both follow the same pattern as `handleAddFilter` / `handleRemoveFilter`.

`handleAddBannedTerm` trims whitespace; rejects blank; redirects to
`/settings?saved=1` on success, or returns 400 on blank input.

#### 6e. Mount routes (in the authenticated router block, around line 339)

```go
r.Post("/settings/banned-terms",        s.handleAddBannedTerm)
r.Post("/settings/banned-terms/remove", s.handleRemoveBannedTerm)
```

#### 6f. Pass banned terms to `ListJobs` in dashboard handler

In `handleDashboard` (or wherever `ListJobs` is called for a logged-in user):

```go
bannedTerms, _ := s.filterStore.ListUserBannedTerms(r.Context(), userID)
lf := store.ListJobsFilter{
    Status:      jobStatus,
    Search:      search,
    BannedTerms: bannedTermsToStrings(bannedTerms), // helper: extracts []string
    Limit:       pageSize,
    Offset:      offset,
}
```

Helper (add to `server.go` or a small `filter_helpers.go`):

```go
func bannedTermsToStrings(terms []models.UserBannedTerm) []string {
    out := make([]string, len(terms))
    for i, t := range terms {
        out[i] = t.Term
    }
    return out
}
```

---

## 7. Template Changes

### `internal/web/templates/settings.html`

Add a new `<section>` for banned terms below the Search Filters section:

```html
<hr>

<section>
  <h2>Banned Keywords &amp; Companies</h2>
  <p class="text-muted">Jobs whose title, company, or description contain any of
     these terms will be hidden from your results.</p>

  {{if .BannedTerms}}
  <table>
    <thead>
      <tr><th>Term</th><th></th></tr>
    </thead>
    <tbody>
      {{range $t := .BannedTerms}}
      <tr>
        <td>{{$t.Term}}</td>
        <td>
          <button hx-post="/settings/banned-terms/remove?id={{$t.ID}}"
                  hx-target="body"
                  hx-push-url="true"
                  class="outline secondary btn-sm">Remove</button>
        </td>
      </tr>
      {{end}}
    </tbody>
  </table>
  {{else}}
  <p class="text-muted">No banned terms configured.</p>
  {{end}}

  <h3>Add Banned Term</h3>
  <form method="POST" action="/settings/banned-terms">
    <input type="hidden" name="gorilla.csrf.Token" value="{{.CSRFToken}}">
    <div class="filter-form-grid">
      <label>
        Term
        <input type="text" name="term" placeholder="e.g. Contractor, Staffing Agency" required>
      </label>
      <button type="submit">Add</button>
    </div>
  </form>
</section>
```

---

## 8. Implementation Steps (Ordered)

| Step | File(s) | Agent | Depends On |
|------|---------|-------|-----------|
| 1 | `internal/store/migrations/011_add_user_banned_terms.sql` | coder | — |
| 2 | `internal/models/user.go` — add `UserBannedTerm` struct | coder | Step 1 |
| 3 | `internal/store/user.go` — add CRUD methods + `ErrDuplicateBannedTerm` | coder | Step 2 |
| 4 | `internal/store/store.go` — extend `ListJobsFilter.BannedTerms` + query | coder | Step 2 |
| 5 | `internal/scraper/scheduler.go` — extend interface + `filterBannedJobs` | coder | Step 3 |
| 6 | `internal/web/server.go` — extend `FilterStore`, settings data/handlers, routes, dashboard filter | coder | Steps 3, 4 |
| 7 | `internal/web/templates/settings.html` — banned terms section | coder | Step 6 |
| 8 | Tests: store (`store_test.go`), scheduler (`scheduler_test.go`), web (`server_test.go`) | qa | Steps 1–7 |

Steps 3 and 4 are independent of each other (both depend only on Step 2) and
can be implemented in parallel.

---

## 9. Acceptance Criteria

- [ ] `user_banned_terms` table exists with correct schema and unique index
- [ ] `ListUserBannedTerms`, `CreateUserBannedTerm`, `DeleteUserBannedTerm` store methods pass unit tests
- [ ] `filterBannedJobs` pure function correctly drops jobs matching any banned term (case-insensitive substring)
- [ ] Scheduler's `RunOnce` loads banned terms per user and passes them through `runFilter`; no banned job is inserted into the DB
- [ ] `ListJobs` with non-empty `BannedTerms` excludes matching rows via SQL
- [ ] Settings page displays existing banned terms and allows adding/removing them
- [ ] Adding a duplicate term returns a 409 Conflict (or silently ignores, implementation choice — document it)
- [ ] Adding a blank term returns 400 Bad Request
- [ ] All existing tests continue to pass after changes to `UserFilterReader` interface

---

## 10. Trade-offs and Alternatives Considered

### Alternative A: Store banned terms as a comma-separated TEXT column on `users`

- Pro: trivial schema change, single row fetch
- Con: unbounded text field; hard to validate individual entries; awkward to
  remove one entry; no created_at per term; cannot add indexes

### Alternative B: Filter only at SQL query time (not at scrape time)

- Pro: no scheduler changes; banned jobs visible if user removes a term
- Con: every `ListJobs` call emits a potentially large WHERE clause; banned jobs
  accumulate in the DB forever and consume storage; no log visibility of filtered count

### Chosen: Separate table + dual-layer filtering (scrape-time + query-time)

The table approach is consistent with the existing `user_search_filters` pattern.
Dual-layer filtering is low-cost (one extra DB query per scrape run, amortised
over many filter rows) and provides belt-and-suspenders coverage for jobs that
were stored before a ban was added.

### Matching semantics

Case-insensitive substring matching (not word-boundary) was chosen because:
- Company names vary: "Google", "Google LLC", "Google, Inc." should all match "google"
- `strings.Contains(lower, term)` is O(n) per job per term — acceptable at the
  volumes SerpAPI returns (≤10 results per filter)
- Whole-word matching via regex adds complexity with minimal real-world benefit

One known false-positive risk: a term like "io" would match many tech companies.
Users are responsible for choosing precise terms. This is acceptable UX for a
power-user feature.

---

## 11. Dependencies and Prerequisites

- No new Go packages required; all logic uses stdlib (`strings`) and existing
  `database/sql` patterns
- Migration `011_add_user_banned_terms.sql` must be deployed before any code
  that references the new table
- The `UserFilterReader` interface change is a breaking interface change — all
  mock implementations in tests must add `ListUserBannedTerms`

---

## 12. File Index

```
internal/
  models/
    user.go                          ← add UserBannedTerm struct
  store/
    migrations/
      011_add_user_banned_terms.sql  ← NEW
    user.go                          ← CRUD methods + ErrDuplicateBannedTerm
    store.go                         ← ListJobsFilter.BannedTerms + query
  scraper/
    scheduler.go                     ← interface extension + filterBannedJobs
  web/
    server.go                        ← FilterStore extension + handlers + routes
    templates/
      settings.html                  ← banned terms UI section
```
