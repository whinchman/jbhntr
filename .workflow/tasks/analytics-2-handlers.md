# Task: analytics-2-handlers

- **Type**: coder
- **Status**: done
- **Parallel Group**: 2
- **Branch**: feature/analytics-2-handlers
- **Source Item**: analytics (plans/analytics.md)
- **Dependencies**: analytics-1-store

## Description

Implement the full web layer for the `/stats` page. This task covers everything
from the `StatsStore` interface through the HTTP handler, route registration,
HTML template, CSS, nav link, and wiring in `main.go`.

The store methods (`GetUserJobStats`, `GetJobsPerWeek`) and their return types
(`store.UserJobStats`, `store.WeeklyJobCount`) are provided by task
`analytics-1-store` and must be done before this task begins.

## Acceptance Criteria

- [ ] `StatsStore` interface is defined in `internal/web/server.go` with exactly the two methods from the Interface Contracts section
- [ ] `Server` struct has a `statsStore StatsStore` field
- [ ] `WithStatsStore(ss StatsStore) *Server` builder method is added to `Server` (same pattern as `WithAdminStore`)
- [ ] `statsTmpl *template.Template` field is added to `Server` and parsed in `NewServerWithConfig` using `templates/layout.html` and `templates/stats.html`
- [ ] `statsData` struct is defined with fields: `Stats store.UserJobStats`, `WeeklyTrend []store.WeeklyJobCount`, `MaxWeekly int`, `CSRFToken string`, `User *models.User`
- [ ] `handleStats` handler is implemented: resolves user from context, calls both store methods, backfills missing weeks to ensure exactly 12 entries, computes `MaxWeekly` (minimum 1 to avoid divide-by-zero), renders `statsTmpl`
- [ ] Route `GET /stats` is registered inside the `requireAuth` middleware group in `Handler()`
- [ ] `internal/web/templates/stats.html` is created: stat cards grid (7 cards: Total Found, Approved, Rejected, Applied, Interviewing, Won, Lost) and weekly bar chart section (12 bars, pure CSS `calc()` heights)
- [ ] `internal/web/templates/layout.html` has "Stats" nav link added between Jobs and Settings links (inside the `{{if .User}}` block)
- [ ] `internal/web/templates/static/app.css` has `.stats-grid`, `.stat-card` (and modifier variants), `.bar-chart`, `.bar-chart__col`, `.bar-chart__bar`, `.bar-chart__count`, `.bar-chart__label` CSS rules appended
- [ ] `cmd/jobhuntr/main.go` calls `.WithStatsStore(st)` on the server after construction
- [ ] `GET /stats` redirects unauthenticated requests to `/login`
- [ ] `GET /stats` returns 200 for authenticated requests and renders the stats page
- [ ] No new Go modules or JS libraries are introduced
- [ ] `go test ./...` passes

## Interface Contracts

The following types come from `internal/store/stats.go` (task `analytics-1-store`):

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

The `StatsStore` interface to define in `internal/web/server.go`:

```go
type StatsStore interface {
    GetUserJobStats(ctx context.Context, userID int64) (store.UserJobStats, error)
    GetJobsPerWeek(ctx context.Context, userID int64, weeks int) ([]store.WeeklyJobCount, error)
}
```

`*store.Store` will satisfy `StatsStore` after `analytics-1-store` completes —
no type assertion needed in `main.go`.

## Context

- Pattern to follow: `WithAdminStore` in `internal/web/server.go` and
  `cmd/jobhuntr/main.go`
- The `requireAuth` middleware group is already present in `Handler()` in
  `internal/web/server.go`; add the `/stats` route inside it
- Template parsing pattern — follow `adminTmpl` or equivalent; pass
  `templateFS`, `"templates/layout.html"`, `"templates/stats.html"`
- Nav link position: inside `.app-nav-links`, after the Jobs link (`<a href="/">`),
  before Settings (`<a href="/settings">`)
- Bar chart backfill: iterate from `now - 11 weeks` to `now` (week boundaries),
  filling in 0 for weeks absent from the DB result. Always pass exactly 12
  entries to the template
- `MaxWeekly` guard: `if MaxWeekly == 0 { MaxWeekly = 1 }` prevents
  `calc(N / 0 * 160px)` in the CSS
- CSS design tokens already in `app.css`: use `var(--space-4)`, `var(--text-3xl)`,
  `var(--text-xs)` for spacing/typography; use PicoCSS surface/background
  variables for the card backgrounds; use semantic color variables if present
  (e.g. success green for `--approved`, danger red for `--rejected`, teal for
  `--won`, warning for `--lost`) — fall back to inline hex if tokens don't exist

### Template skeleton for `stats.html`

```html
{{template "layout.html" .}}
{{define "title"}}JobHuntr — Stats{{end}}
{{define "content"}}
  <h1>Job Search Stats</h1>
  <div class="stats-grid">
    <div class="stat-card">
      <span class="stat-value">{{.Stats.TotalFound}}</span>
      <span class="stat-label">Total Found</span>
    </div>
    <div class="stat-card stat-card--approved">
      <span class="stat-value">{{.Stats.TotalApproved}}</span>
      <span class="stat-label">Approved</span>
    </div>
    <div class="stat-card stat-card--rejected">
      <span class="stat-value">{{.Stats.TotalRejected}}</span>
      <span class="stat-label">Rejected</span>
    </div>
    <div class="stat-card stat-card--applied">
      <span class="stat-value">{{.Stats.TotalApplied}}</span>
      <span class="stat-label">Applied</span>
    </div>
    <div class="stat-card stat-card--interviewing">
      <span class="stat-value">{{.Stats.TotalInterviewing}}</span>
      <span class="stat-label">Interviewing</span>
    </div>
    <div class="stat-card stat-card--won">
      <span class="stat-value">{{.Stats.TotalWon}}</span>
      <span class="stat-label">Won</span>
    </div>
    <div class="stat-card stat-card--lost">
      <span class="stat-value">{{.Stats.TotalLost}}</span>
      <span class="stat-label">Lost</span>
    </div>
  </div>
  <section class="stats-chart-section">
    <h2>Jobs Found per Week (Last 12 Weeks)</h2>
    <div class="bar-chart">
      {{range .WeeklyTrend}}
      <div class="bar-chart__col">
        <span class="bar-chart__count">{{.Count}}</span>
        <div class="bar-chart__bar"
             style="height: calc({{.Count}} / {{$.MaxWeekly}} * 160px)"></div>
        <span class="bar-chart__label">{{.WeekStart.Format "Jan 2"}}</span>
      </div>
      {{end}}
    </div>
  </section>
{{end}}
```

## Notes

Implemented on branch `feature/analytics-2-handlers` (based on `feature/analytics-1-store`).

### Changes made

- `internal/web/server.go`: Added `StatsStore` interface, `statsStore StatsStore` field on `Server`, `statsTmpl *template.Template` field, `WithStatsStore(ss StatsStore) *Server` builder, `statsData` struct, `handleStats` handler with 12-week backfill logic and MaxWeekly guard, `GET /stats` route in `requireAuth` group, `statsTmpl` parsed in `NewServerWithConfig`.
- `internal/web/templates/stats.html`: New template with 7-card stats grid and 12-bar pure-CSS bar chart section.
- `internal/web/templates/layout.html`: Added `{{if .User}}<a href="/stats">Stats</a>{{end}}` nav link between Rejected and Settings.
- `internal/web/templates/static/app.css`: Appended section 19 with `.stats-grid`, `.stat-card` (and 6 modifier variants), `.bar-chart`, `.bar-chart__col`, `.bar-chart__bar`, `.bar-chart__count`, `.bar-chart__label` rules.
- `cmd/jobhuntr/main.go`: Added `.WithStatsStore(db)` call in server construction chain.
- `internal/web/stats_test.go`: `TestHandleStats_Unauthenticated` (expects 303 → /login) and `TestHandleStats_Authenticated` (expects 200 + correct body content).

Note: `go build ./...` and `go test ./...` could not be run in this environment (Go runtime not installed; project uses Docker). All acceptance criteria reviewed manually against the code — no compilation errors expected.
