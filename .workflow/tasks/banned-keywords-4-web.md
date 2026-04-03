# Task: banned-keywords-4-web

- **Type**: coder
- **Status**: pending
- **Parallel Group**: 3
- **Branch**: feature/banned-keywords-4-web
- **Source Item**: Banned Keywords / Companies feature (plans/banned-keywords.md)
- **Dependencies**: banned-keywords-2-store

## Description

Wire the banned-terms feature into the web layer:

1. Extend the `FilterStore` interface in `internal/web/server.go` with the three banned-term store methods
2. Extend `settingsData` to carry `BannedTerms`
3. Update `handleSettings` to load and populate banned terms
4. Add `handleAddBannedTerm` and `handleRemoveBannedTerm` HTTP handlers
5. Mount the two new routes in the authenticated router block
6. Pass banned terms to `ListJobs` in the dashboard handler (query-time filtering)
7. Add the `bannedTermsToStrings` helper
8. Add the banned terms `<section>` to `internal/web/templates/settings.html`

## Acceptance Criteria

- [ ] `FilterStore` interface includes `ListUserBannedTerms`, `CreateUserBannedTerm`, `DeleteUserBannedTerm`
- [ ] `settingsData` has a `BannedTerms []models.UserBannedTerm` field
- [ ] Settings page loads and displays all banned terms for the logged-in user
- [ ] `POST /settings/banned-terms` with a valid term adds it and redirects to `/settings?saved=1`
- [ ] `POST /settings/banned-terms` with a blank term returns HTTP 400
- [ ] `POST /settings/banned-terms/remove?id=<termID>` removes the term and redirects
- [ ] Dashboard `ListJobs` call passes banned terms via `ListJobsFilter.BannedTerms`
- [ ] Settings HTML template renders the banned terms table and add-term form
- [ ] All existing tests continue to pass (update any mocks that implement `FilterStore`)

## Interface Contracts

Consumed from banned-keywords-2-store:

```go
// Store methods to add to FilterStore interface:
ListUserBannedTerms(ctx context.Context, userID int64) ([]models.UserBannedTerm, error)
CreateUserBannedTerm(ctx context.Context, userID int64, term string) (*models.UserBannedTerm, error)
DeleteUserBannedTerm(ctx context.Context, userID int64, termID int64) error

// ErrDuplicateBannedTerm sentinel (internal/store/user.go):
var ErrDuplicateBannedTerm = errors.New("store: banned term already exists for user")

// ListJobsFilter extension (internal/store/store.go):
BannedTerms []string
```

## Context

Files to modify:
- `internal/web/server.go`
- `internal/web/templates/settings.html`

### server.go changes

Extend `FilterStore` interface (around line 80):

```go
type FilterStore interface {
    // existing methods unchanged ...
    // NEW:
    ListUserBannedTerms(ctx context.Context, userID int64) ([]models.UserBannedTerm, error)
    CreateUserBannedTerm(ctx context.Context, userID int64, term string) (*models.UserBannedTerm, error)
    DeleteUserBannedTerm(ctx context.Context, userID int64, termID int64) error
}
```

Extend `settingsData` (around line 674):

```go
type settingsData struct {
    Filters     []models.UserSearchFilter
    BannedTerms []models.UserBannedTerm  // NEW
    Resume      string
    NtfyTopic   string
    Saved       bool
    CSRFToken   string
    User        *models.User
}
```

In `handleSettings`, after loading filters, load banned terms:

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
data.BannedTerms = bannedTerms
```

New handlers (follow the `handleAddFilter` / `handleRemoveFilter` pattern at line 773+):

```go
// POST /settings/banned-terms
func (s *Server) handleAddBannedTerm(w http.ResponseWriter, r *http.Request)

// POST /settings/banned-terms/remove?id=<termID>
func (s *Server) handleRemoveBannedTerm(w http.ResponseWriter, r *http.Request)
```

`handleAddBannedTerm`: trim whitespace; reject blank (400); call `CreateUserBannedTerm`; on `ErrDuplicateBannedTerm` return 409 or redirect silently (document choice in Notes); redirect to `/settings?saved=1` on success.

Mount routes in the authenticated block (around line 342, near existing filter routes):

```go
r.Post("/settings/banned-terms",        s.handleAddBannedTerm)
r.Post("/settings/banned-terms/remove", s.handleRemoveBannedTerm)
```

Dashboard integration — in `handleDashboard` (or wherever `ListJobs` is called for a logged-in user), add:

```go
bannedTerms, _ := s.filterStore.ListUserBannedTerms(r.Context(), userID)
// then include in ListJobsFilter:
lf := store.ListJobsFilter{
    // ... existing fields ...
    BannedTerms: bannedTermsToStrings(bannedTerms),
}
```

Helper function (add to `server.go` or a new `internal/web/filter_helpers.go`):

```go
func bannedTermsToStrings(terms []models.UserBannedTerm) []string {
    out := make([]string, len(terms))
    for i, t := range terms {
        out[i] = t.Term
    }
    return out
}
```

### settings.html changes

Add a new `<section>` below the existing Search Filters section:

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

## Notes

