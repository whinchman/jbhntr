# Plan: Analytics — Job Search Stats Dashboard

**Author**: Architect agent
**Date**: 2026-04-03
**Status**: Draft — awaiting Manager decomposition

---

## 1. Architecture Overview

### Feature Summary

Add a `/stats` page that shows per-user aggregate metrics about their job
search pipeline. The page displays stat cards for the key lifecycle counts
plus a weekly trend chart using the existing `discovered_at` timestamp.
Stats are computed on-the-fly with SQL COUNT/GROUP BY queries — no
materialization layer is needed at this scale.

### Key Decisions

| Decision | Choice | Rationale |
|---|---|---|
| **Page location** | Dedicated `/stats` route | Keeps the dashboard uncluttered; stats are a secondary view, not primary workflow. A nav link "Stats" sits beside "Settings". |
| **Computation strategy** | On-the-fly SQL | Job counts per user are small (hundreds to low-thousands). A single multi-count query completes in <5 ms on PostgreSQL. No materialization complexity needed. |
| **Scope** | Per-user only (v1) | Each user can only see their own data. Admin aggregate stats already exist at `/admin`. No new admin view needed for v1. |
| **UI style** | Stat cards + weekly bar chart (HTML/CSS only) | Matches existing PicoCSS + indigo design system. No charting library needed for a basic bar chart — rendered as inline `<div>` bars in HTML. Keeps zero new JS deps. |
| **Trends granularity** | Jobs discovered per week (last 12 weeks) | `discovered_at` is the only reliable timestamp available now. Pipeline status dates (`applied_at`, `interviewing_at`, etc.) will come from the job-pipeline-pages epic. |

### Data Flow

```
Browser GET /stats
  → requireAuth middleware
  → handleStats handler
      → store.GetUserJobStats(ctx, userID)   [new store method]
          → single SQL query: COUNT(*) GROUP BY status
      → store.GetJobsPerWeek(ctx, userID, 12) [new store method]
          → SQL: COUNT(*) GROUP BY date_trunc('week', discovered_at)
      → render stats.html template
```

### Module Boundaries

- **`internal/store`**: two new query methods on `*Store`; new `UserJobStats` struct
- **`internal/web`**: new `StatsStore` interface; new `handleStats` handler; new `statsTmpl`; nav link added to `layout.html`
- **`internal/web/templates`**: new `stats.html` template + stat card CSS additions to `app.css`
- **`internal/models`**: no changes needed in v1 (new statuses come from job-pipeline-pages)

---

## 2. Dependency on job-pipeline-pages

The following statuses are **not yet in the schema** and will be added by the
concurrent `job-pipeline-pages` epic:

- `applied`
- `interviewing`
- `won`
- `lost`

### What this means for analytics

The `jobs.status` column CHECK constraint in `store.go` (baseline schema)
currently only allows:
`discovered | notified | approved | rejected | generating | complete | failed`

The job-pipeline-pages migration will extend this constraint to include the
four new statuses. Until that migration exists, the stats query must not
assume those values are present — it must use `COUNT(*) FILTER (WHERE status = ...)` so
that statuses with zero rows return `0` rather than being absent from results.

### Interface Contract

The analytics coder **must read** the job-pipeline-pages plan for the exact
status string values to use. Assuming the following (confirm before coding):

| Display label | Status string |
|---|---|
| Applied | `applied` |
| Interviewing | `interviewing` |
| Won | `won` |
| Lost | `lost` |

If job-pipeline-pages uses different strings, update the constants in
`internal/models/models.go` to match and reference those constants in the
stats query — do NOT hardcode status strings in the stats SQL.

### Suggested sequencing

The analytics coder task should have a **dependency** on the
job-pipeline-pages migration task being `done`. If the Coordinator needs to
ship analytics before job-pipeline-pages is merged, the stat cards for
Applied / Interviewing / Won / Lost should render `—` or `0 (coming soon)`
rather than breaking.

---

## 3. Implementation Plan

### Step 1 — Store: `UserJobStats` struct and `GetUserJobStats` method

**File**: `internal/store/stats.go` (new file)

```go
// UserJobStats holds aggregate counts for a single user's job pipeline.
type UserJobStats struct {
    TotalFound       int
    TotalApproved    int
    TotalRejected    int
    TotalApplied     int
    TotalInterviewing int
    TotalWon         int
    TotalLost        int
}

// GetUserJobStats returns aggregate counts for the given user.
func (s *Store) GetUserJobStats(ctx context.Context, userID int64) (UserJobStats, error)
```

**SQL** (single query using conditional aggregation — no multiple round trips):
```sql
SELECT
    COUNT(*)                                               AS total_found,
    COUNT(*) FILTER (WHERE status = 'approved')            AS total_approved,
    COUNT(*) FILTER (WHERE status = 'rejected')            AS total_rejected,
    COUNT(*) FILTER (WHERE status = 'applied')             AS total_applied,
    COUNT(*) FILTER (WHERE status = 'interviewing')        AS total_interviewing,
    COUNT(*) FILTER (WHERE status = 'won')                 AS total_won,
    COUNT(*) FILTER (WHERE status = 'lost')                AS total_lost
FROM jobs
WHERE user_id = $1
```

Note: `COUNT(*) FILTER (WHERE status = 'applied')` returns 0 even if no rows
with `status = 'applied'` exist — safe before job-pipeline-pages migration.

### Step 2 — Store: `WeeklyJobCount` struct and `GetJobsPerWeek` method

**File**: `internal/store/stats.go` (same file, append)

```go
// WeeklyJobCount holds a week label and the count of jobs discovered that week.
type WeeklyJobCount struct {
    WeekStart time.Time
    Count     int
}

// GetJobsPerWeek returns jobs-discovered counts grouped by ISO week for the
// past n weeks (inclusive of the current partial week).
func (s *Store) GetJobsPerWeek(ctx context.Context, userID int64, weeks int) ([]WeeklyJobCount, error)
```

**SQL**:
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

The handler should backfill any missing weeks with `Count: 0` before passing
to the template so the bar chart always shows `n` bars.

### Step 3 — Web: `StatsStore` interface

**File**: `internal/web/server.go` (append to existing interface block)

```go
// StatsStore is the subset of store.Store used by the stats page.
type StatsStore interface {
    GetUserJobStats(ctx context.Context, userID int64) (store.UserJobStats, error)
    GetJobsPerWeek(ctx context.Context, userID int64, weeks int) ([]store.WeeklyJobCount, error)
}
```

Add a `statsStore StatsStore` field to the `Server` struct and a
`WithStatsStore(ss StatsStore) *Server` builder method (same pattern as
`WithAdminStore`).

In `NewServerWithConfig`, accept the concrete `*store.Store` for this field
if the existing `JobStore` argument implements `StatsStore` (it will, after
Step 1/2). Alternatively, thread it through `WithStatsStore` in `main.go`.

### Step 4 — Web: stats template data type and handler

**File**: `internal/web/server.go` (append)

```go
type statsData struct {
    Stats       store.UserJobStats
    WeeklyTrend []store.WeeklyJobCount
    MaxWeekly   int       // max Count across all weeks — used for bar scaling
    CSRFToken   string
    User        *models.User
}
```

**Handler**: `func (s *Server) handleStats(w http.ResponseWriter, r *http.Request)`

1. Resolve `user` from context (requireAuth guarantees non-nil).
2. Call `s.statsStore.GetUserJobStats(ctx, user.ID)`.
3. Call `s.statsStore.GetJobsPerWeek(ctx, user.ID, 12)`, backfill missing weeks.
4. Compute `MaxWeekly` = max(Count) across all weeks (for CSS bar height scaling).
5. Render `statsTmpl` with `statsData`.

### Step 5 — Web: route registration

**File**: `internal/web/server.go` in `Handler()`

Inside the `requireAuth` group:
```go
r.Get("/stats", s.handleStats)
```

Also parse `statsTmpl` in `NewServerWithConfig`:
```go
statsTmpl := template.Must(template.New("layout.html").Funcs(tmplFuncs).ParseFS(templateFS,
    "templates/layout.html",
    "templates/stats.html",
))
```

Add `statsTmpl *template.Template` to the `Server` struct.

### Step 6 — Template: `stats.html`

**File**: `internal/web/templates/stats.html` (new file)

Structure:
```
{{template "layout.html" .}}
{{define "title"}}JobHuntr — Stats{{end}}
{{define "content"}}
  <h1>Job Search Stats</h1>

  <!-- Stat cards grid -->
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

  <!-- Weekly trend bar chart (pure CSS) -->
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

Guard against `MaxWeekly == 0` in the handler (set to 1 if 0 to avoid
division-by-zero in the CSS calc).

### Step 7 — CSS: stat card and bar chart styles

**File**: `internal/web/templates/static/app.css` (append new section)

Add:
- `.stats-grid` — CSS grid, `auto-fit` columns, `minmax(140px, 1fr)`, gap `var(--space-4)`
- `.stat-card` — card with surface background, border-radius, padding, centered text
- `.stat-card .stat-value` — large font (`var(--text-3xl)`), bold, accent color
- `.stat-card .stat-label` — small muted label
- Modifier variants for semantic colors (e.g. `--approved` → success green,
  `--rejected` → danger red, `--won` → teal, `--lost` → warning)
- `.bar-chart` — flexbox row, align-items flex-end, gap var(--space-2)
- `.bar-chart__col` — flex column, align center
- `.bar-chart__bar` — background accent color, min-height 2px, width 32px
- `.bar-chart__count` / `.bar-chart__label` — `var(--text-xs)`, muted

### Step 8 — Nav: add Stats link to layout

**File**: `internal/web/templates/layout.html`

Inside `.app-nav-links` (shown only when `{{if .User}}`):
```html
<a href="/stats">Stats</a>
```

Place after the Jobs link, before Settings.

### Step 9 — Wire up in `main.go`

**File**: `cmd/jobhuntr/main.go`

The existing code already passes the concrete `*store.Store` to `NewServerWithConfig`.
Add a call to `.WithStatsStore(st)` after the server is constructed. Since
`*store.Store` will implement `StatsStore` after Step 1/2, no type assertion needed.

---

## 4. Testing Plan

### Unit tests — `internal/store/stats_test.go` (new file)

- `TestGetUserJobStats_empty` — new user, no jobs; all counts == 0
- `TestGetUserJobStats_counts` — insert known jobs with various statuses; assert each counter
- `TestGetUserJobStats_userScoped` — two users, assert each only sees their own counts
- `TestGetJobsPerWeek_empty` — no jobs; returns empty slice
- `TestGetJobsPerWeek_filledWeeks` — insert jobs at known timestamps; assert counts
- `TestGetJobsPerWeek_backfill` — verify handler-level backfill produces 12 entries

### Integration tests — `internal/web/integration_test.go` (extend existing file)

- `TestStatsPage_unauthenticated` — GET /stats redirects to /login
- `TestStatsPage_authenticated` — GET /stats 200; body contains "Total Found"
- `TestStatsPage_counts` — seed jobs, assert stat values appear in HTML

### Template tests (existing pattern in `ui_minor_internal_test.go`)

- Test that `statsData` with known values renders correct HTML snippets

---

## 5. Trade-offs and Alternatives

### Option A (chosen): On-the-fly SQL with conditional aggregation

**Pros**: Simple; always up-to-date; single query; no cache invalidation.
**Cons**: Query runs on every page load.
**Why chosen**: At realistic user scale (hundreds of jobs), PostgreSQL executes
this in <1 ms. Adding a materialized view would require a second migration,
a refresh trigger, and cache-invalidation logic — significant complexity for
no measurable benefit.

### Option B: Dashboard widget (stats inline on `/`)

**Pros**: Users see counts without navigating.
**Cons**: Clutters the primary workflow page; the dashboard already has tab
filtering and search; stats would be redundant with tab counts.
**Why rejected**: Stats are a secondary concern. A dedicated `/stats` route
is cleaner and follows the existing pattern of separate pages for settings,
profile, and onboarding.

### Option C: Redis/materialized cache

**Pros**: Fastest query path.
**Cons**: Introduces a new infrastructure dependency (Redis); no Redis in the
existing stack (only PostgreSQL). Overkill for per-user aggregate counts.
**Why rejected**: The project has no caching layer. Adding one for stats alone
is not justified.

### Option D: Chart.js / D3 bar chart

**Pros**: Richer interactivity.
**Cons**: New JS dependency (bundle size, CDN trust); project currently uses
zero client-side JS libraries beyond htmx.
**Why rejected**: A pure-CSS bar chart is sufficient for the "jobs per week"
view and is consistent with the project's minimal-JS philosophy.

---

## 6. Acceptance Criteria

- [ ] `GET /stats` requires authentication; unauthenticated requests redirect to `/login`
- [ ] Stats page shows 7 stat cards: Total Found, Approved, Rejected, Applied, Interviewing, Won, Lost
- [ ] All counts are per-user (user A cannot see user B's stats)
- [ ] Applied / Interviewing / Won / Lost cards show `0` gracefully before job-pipeline-pages statuses exist in the DB
- [ ] Weekly trend chart shows up to 12 bars; weeks with no jobs show a zero-height bar (or `0` label)
- [ ] Nav bar shows "Stats" link between "Jobs" and "Settings" for authenticated users
- [ ] New store methods have unit tests with ≥80% line coverage of `stats.go`
- [ ] Integration test verifies the page renders for an authenticated user
- [ ] No new external dependencies (no charting library, no caching layer)
- [ ] All existing tests continue to pass

---

## 7. File Summary

| Path | Action | Owner |
|---|---|---|
| `internal/store/stats.go` | Create | Coder |
| `internal/store/stats_test.go` | Create | Coder / QA |
| `internal/web/server.go` | Modify — add `StatsStore` interface, `statsStore` field, `WithStatsStore`, `statsData`, `handleStats`, register route, parse template | Coder |
| `internal/web/templates/stats.html` | Create | Coder |
| `internal/web/templates/layout.html` | Modify — add Stats nav link | Coder |
| `internal/web/templates/static/app.css` | Modify — add stat card + bar chart CSS | Coder |
| `cmd/jobhuntr/main.go` | Modify — wire `WithStatsStore` | Coder |
| `internal/web/integration_test.go` | Modify — add stats page tests | QA |

---

## 8. Dependencies and Prerequisites

- **job-pipeline-pages epic**: must add `applied`, `interviewing`, `won`, `lost`
  to the `jobs.status` CHECK constraint via a migration. The analytics coder
  should treat this as a soft dependency — the stats query is safe before this
  migration (returns 0 for those statuses), but the counts will only be
  meaningful after job-pipeline-pages is shipped.
- **No new Go modules required.** All needed packages (`database/sql`,
  `context`, `time`, `html/template`, `net/http`) are already in the project.
- **No schema migration required** for analytics itself. The stats query reads
  the existing `jobs` table. The new `applied`/`interviewing`/`won`/`lost`
  statuses are the responsibility of job-pipeline-pages.
