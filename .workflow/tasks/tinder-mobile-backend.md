# Task: tinder-mobile-backend

- **Type**: coder
- **Status**: pending
- **Parallel Group**: 1
- **Branch**: feature/tinder-mobile-backend
- **Source Item**: Tinder-Style Mobile Job Review UI
- **Dependencies**: none

## Description

Add the backend Go changes required for the swipe-card mobile job review interface. This task covers all changes to `internal/web/server.go`: registering the new template, adding a new HTTP route and handler for the card-deck partial, and extending the existing `respondJobAction` handler to detect card-deck HTMX requests and render the correct template.

No new data models or database queries are needed — the existing `jobRowsData` struct and `store.ListJobs` are reused.

## Acceptance Criteria

- [ ] `NewServerWithConfig` parses `"templates/partials/job_cards.html"` alongside `layout.html`, `dashboard.html`, and `job_rows.html` in the `ParseFS` call
- [ ] `Handler()` registers `GET /partials/job-cards` in the optional-auth group (same group as `/partials/job-table`)
- [ ] `handleJobCardsPartial` method exists on `*Server` with the signature `func (s *Server) handleJobCardsPartial(w http.ResponseWriter, r *http.Request)`
- [ ] `handleJobCardsPartial` returns 200 with empty body for unauthenticated requests (no redirect)
- [ ] `handleJobCardsPartial` excludes `StatusRejected` jobs via `f.ExcludeStatuses`
- [ ] `handleJobCardsPartial` executes template `"job_cards"` with `jobRowsData`
- [ ] `respondJobAction` detects `HX-Target: job-card-deck` and renders `"job_cards"` template instead of `"job_rows"`
- [ ] `respondJobAction` sets `f.ExcludeStatuses = []models.JobStatus{models.StatusRejected}` for card-deck requests
- [ ] `respondJobAction` continues to render `"job_rows"` for all other HTMX requests (regression: `HX-Target: job-table-body` still works)
- [ ] `go build ./...` passes with no errors

## Interface Contracts

This is a single-repo project. No cross-repo contracts.

The `job_cards.html` template (created by the frontend task) must define a named template `"job_cards"` accepting `jobRowsData` (defined in `server.go`):

```go
type jobRowsData struct {
    Jobs      []models.Job
    CSRFToken string
}
```

The new route contract:

| Property | Value |
|----------|-------|
| Method | GET |
| Path | `/partials/job-cards` |
| Auth | Optional (returns empty fragment for unauthenticated) |
| Query params | `status`, `q`, `sort`, `order` (same as `/partials/job-table`) |
| Handler | `func (s *Server) handleJobCardsPartial(w http.ResponseWriter, r *http.Request)` |
| Template executed | `"job_cards"` |
| Data struct | `jobRowsData{Jobs: []models.Job, CSRFToken: string}` |
| Response | `text/html; charset=utf-8` |

`respondJobAction` card-deck detection: check `r.Header.Get("HX-Target") == "job-card-deck"`.

## Context

### File: `internal/web/server.go`

**Change A — Add `job_cards.html` to the template parse call in `NewServerWithConfig`:**

Current call:
```go
template.Must(template.New("layout.html").Funcs(tmplFuncs).ParseFS(templateFS,
    "templates/layout.html",
    "templates/dashboard.html",
    "templates/partials/job_rows.html",
))
```

New call:
```go
template.Must(template.New("layout.html").Funcs(tmplFuncs).ParseFS(templateFS,
    "templates/layout.html",
    "templates/dashboard.html",
    "templates/partials/job_rows.html",
    "templates/partials/job_cards.html",
))
```

**Change B — Register route in `Handler()` optional-auth group:**
```go
r.Get("/partials/job-cards", s.handleJobCardsPartial)
```
Place this adjacent to the existing `r.Get("/partials/job-table", ...)` registration.

**Change C — New handler (full implementation):**
```go
func (s *Server) handleJobCardsPartial(w http.ResponseWriter, r *http.Request) {
    user := UserFromContext(r.Context())
    if user == nil {
        w.Header().Set("Content-Type", "text/html; charset=utf-8")
        return
    }
    var bannedTermStrings []string
    if s.filterStore != nil {
        bannedTerms, err := s.filterStore.ListUserBannedTerms(r.Context(), user.ID)
        if err != nil {
            slog.Warn("failed to load banned terms", "error", err)
        } else {
            bannedTermStrings = bannedTermsToStrings(bannedTerms)
        }
    }
    q := r.URL.Query()
    sort, order := parseSortParams(q)
    f := store.ListJobsFilter{
        Status:          models.JobStatus(q.Get("status")),
        ExcludeStatuses: []models.JobStatus{models.StatusRejected},
        Search:          q.Get("q"),
        Sort:            sort,
        Order:           order,
        BannedTerms:     bannedTermStrings,
    }
    jobs, err := s.store.ListJobs(r.Context(), user.ID, f)
    if err != nil {
        http.Error(w, "failed to list jobs", http.StatusInternalServerError)
        return
    }
    if jobs == nil {
        jobs = []models.Job{}
    }
    w.Header().Set("Content-Type", "text/html; charset=utf-8")
    if err := s.templates.ExecuteTemplate(w, "job_cards", jobRowsData{Jobs: jobs, CSRFToken: csrf.Token(r)}); err != nil {
        slog.Error("template render error", "error", err)
    }
}
```

**Change D — Extend `respondJobAction`:**

Inside the `if r.Header.Get("HX-Request") == "true"` block, after the `ListJobs` call and before the existing `ExecuteTemplate("job_rows", ...)` call, add:

```go
w.Header().Set("Content-Type", "text/html; charset=utf-8")
data := jobRowsData{Jobs: jobs, CSRFToken: csrf.Token(r)}

if r.Header.Get("HX-Target") == "job-card-deck" {
    f.ExcludeStatuses = []models.JobStatus{models.StatusRejected}
    // Re-fetch jobs with exclude filter applied for card deck
    jobs, err = s.store.ListJobs(r.Context(), userID, f)
    if err != nil {
        http.Error(w, "failed to list jobs", http.StatusInternalServerError)
        return
    }
    if jobs == nil {
        jobs = []models.Job{}
    }
    data = jobRowsData{Jobs: jobs, CSRFToken: csrf.Token(r)}
    if err := s.templates.ExecuteTemplate(w, "job_cards", data); err != nil {
        slog.Error("template render error", "error", err)
    }
    return
}
if err := s.templates.ExecuteTemplate(w, "job_rows", data); err != nil {
    slog.Error("template render error", "error", err)
}
```

Note: read the existing `respondJobAction` implementation carefully. The plan notes that `ExcludeStatuses` should be set for card-deck requests. The exact placement of the branch depends on how `respondJobAction` currently builds the filter `f`. Adjust accordingly, preserving all existing logic for the non-card-deck path.

## Notes

<!-- implementing agent fills in when complete -->
