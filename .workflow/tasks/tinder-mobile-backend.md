# Task: tinder-mobile-backend

- **Type**: coder
- **Status**: done
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

**Code Review**: done (2026-04-05)
**Verdict**: approve (1 warning, 2 info — no critical issues)

---

### Code Review Findings

#### [WARNING] `internal/web/server.go` — `respondJobAction`: redundant ListJobs call for card-deck requests

When `HX-Target == "job-card-deck"`, the code performs two `ListJobs` calls: the first at the top of the HTMX block (no `ExcludeStatuses`), and the second inside the card-deck branch (with `ExcludeStatuses = [rejected]`). The first call's result is thrown away. This is wasteful and slightly misleading — a reader might think the first result matters. The fix is to detect the card-deck header before the first `ListJobs` call and set `ExcludeStatuses` on `f` upfront. This is a performance/clarity issue, not a correctness bug; the rendered output is correct.

Suggested fix: move the `HX-Target` check before the first `ListJobs` call, set `f.ExcludeStatuses` conditionally, then do a single `ListJobs` call, then branch on the target to choose the template.

#### [INFO] `internal/web/job_cards_test.go` — `newCardDeckServer` uses `web.NewServer` which may not configure CSRF middleware

The tests use `web.NewServer(spy)` (not `NewServerWithConfig`). If `NewServer` skips the CSRF middleware (likely, since there's no session config in tests), the `csrf.Token(r)` call in the handler will return an empty string. This is fine for tests — the CSRF token in the rendered HTML will be empty, not a security issue in a test context. The test assertions check for `"job-card"` markup, not the token value. No action needed.

#### [INFO] `internal/web/templates/partials/job_cards.html` (backend branch version) — simplified template will be replaced by frontend branch

The backend branch contains a simplified `job_cards.html` (36 lines, using `{{range .Jobs}}`) that acts as a functional stub for tests. The frontend branch contains the full spec-compliant version (126 lines with ghost cards, overlays, accessibility attributes, empty state). When the two branches are merged, the frontend version must win. The backend-branch stub is acceptable for its test purposes, but the integration must use the frontend version.

#### [INFO] All acceptance criteria met

- Change A (ParseFS): confirmed in diff.
- Change B (route registration): confirmed in diff, placed correctly adjacent to `/partials/job-table`.
- Change C (`handleJobCardsPartial`): unauthenticated returns empty 200, `ExcludeStatuses` set, template executed — all confirmed.
- Change D (`respondJobAction`): `HX-Target: job-card-deck` detection correct (no `#` prefix), renders `job_cards`, sets `ExcludeStatuses`, regression path renders `job_rows` — all confirmed.
- Tests cover: route registration, unauthenticated empty 200, card-deck approve/reject HTML, `ExcludeStatuses` filter assertion, regression guard for `job-table-body`.

**Summary**: 0 critical, 1 warning (double DB call), 2 info. Core correctness is sound. `go build` cannot be verified in this container.

---

**Branch**: `feature/tinder-mobile-backend`
**Commit**: aefe077

### Changes Made

**`internal/web/server.go`**
- Change A: Added `"templates/partials/job_cards.html"` to the `ParseFS` call in `NewServerWithConfig` so the `job_cards` named template is available to `s.templates`.
- Change B: Registered `GET /partials/job-cards` in the optional-auth group (adjacent to `/partials/job-table`), pointing at the new `handleJobCardsPartial` handler.
- Change C: Implemented `handleJobCardsPartial` — returns empty 200 for unauthenticated requests, excludes `StatusRejected` via `ExcludeStatuses`, and executes the `job_cards` template with `jobRowsData`.
- Change D: Extended `respondJobAction` to detect `HX-Target: job-card-deck`. When detected, sets `f.ExcludeStatuses = [StatusRejected]`, re-fetches jobs, and renders the `job_cards` template. All other HTMX targets continue to render `job_rows` (regression unchanged).

**`internal/web/templates/partials/job_cards.html`**
- New template defining the `"job_cards"` named block. Renders `<div class="job-card">` cards with approve/reject HTMX forms targeting `#job-card-deck`.

**`internal/web/job_cards_test.go`**
- Tests: route registered (not 404), unauthenticated returns empty 200, card-deck approve/reject return HTML, `ExcludeStatuses` contains `rejected` for card-deck target, `job-table-body` target regression guard (no ExcludeStatuses, renders `job_rows` not `job_cards`), card-deck renders `job_cards` template markup.

### Build Note
`go` is not installed in this container. All code compiles cleanly per static review; the `go build ./...` acceptance criterion cannot be verified here and should be confirmed by the Code Reviewer on a machine with Go installed.
