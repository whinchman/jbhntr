# Architecture Plan: job-pipeline-pages

**Date**: 2026-04-03
**Author**: Architect Agent

---

## Overview

Reorganize the job UI into distinct pages based on pipeline stage, and add
application-status tracking for approved jobs. The dashboard becomes a
"pending triage" view; two new pages handle Approved and Rejected jobs
respectively. Status tracking (Applied / Interviewing / Lost / Won) with
date recording lives exclusively on the Approved Jobs page.

---

## Key Decisions

### 1. Routes / Pages

| Route | Purpose | Auth |
|---|---|---|
| `/` (existing) | Dashboard — pending triage (discovered + notified) | optional |
| `/jobs/approved` | Approved jobs + application status tracking | required |
| `/jobs/rejected` | Rejected jobs archive | required |
| `/jobs/{id}` | Job detail (unchanged) | required |

**Rationale**: separate routes give users bookmarkable, shareable URLs and
keep each page's template data model simple. A single shared dashboard with
status tabs was getting crowded and mixed concerns (triage vs. post-approval
tracking).

**Dashboard change**: The `allStatuses` list in `server.go` currently drives
tab rendering for ALL statuses. After this change the dashboard shows only
`discovered` and `notified` tabs (i.e. jobs awaiting the user's approve/reject
decision). The generating/complete/failed tabs move to the Approved page since
those statuses are downstream of approval. The rejected tab moves to `/jobs/rejected`.

Concretely, the dashboard tabs will be:
- `all` (default) — discovered + notified combined
- `discovered`
- `notified`

And the Approved page tabs will be:
- `all` — all approved+ statuses (approved, generating, complete, failed)
- `approved` — waiting for generation
- `generating`
- `complete`
- `failed`

### 2. Application Status Tracking

A new `application_status` concept is layered on top of the existing
`JobStatus` pipeline. These are **user-facing labels** for jobs that have
entered the approval pipeline — they represent the user's real-world
application progress.

**Application statuses**:
- `applied` — user has submitted their application
- `interviewing` — actively in the interview process
- `lost` — job search ended (rejected by employer, offer declined, etc.)
- `won` — job offer accepted

**Decision: current status + per-transition timestamps, not a full log**

A full event-log table is more flexible but adds join complexity for a simple
read path. Instead we store the current `application_status` value and four
nullable timestamp columns (one per transition). This is fully queryable,
renders trivially in templates, and a migration can add the log table later
if the product demands it.

**Columns added to `jobs` table** (migration 011):
```sql
application_status  TEXT   CHECK(application_status IN ('applied','interviewing','lost','won')),
applied_at          TIMESTAMPTZ,
interviewing_at     TIMESTAMPTZ,
lost_at             TIMESTAMPTZ,
won_at              TIMESTAMPTZ
```

All nullable — a job with no `application_status` simply hasn't been acted on
yet.

### 3. Status Transitions (UI Mechanism)

Use **HTMX inline update** — a `<select>` on each row of the Approved page
posts to a new endpoint. The endpoint updates the DB and returns a replacement
row fragment.

**Why inline select over full-page action or separate modal**: consistent with
the existing approve/reject pattern in `job_rows.html`, zero page reloads,
minimal JS, simple server implementation. A full-page form submit would be
jarring for a list of many jobs.

**Why not a dropdown (custom JS)**: the native `<select>` element with
`hx-trigger="change"` is simpler and accessible without custom JS.

**Endpoint**:
```
POST /api/jobs/{id}/application-status
Body: application_status=applied|interviewing|lost|won
```
Returns: a replacement `<tr>` fragment (same `job_rows_approved` partial).

**Transition rules**: any `application_status` value can be set from any
other (or from NULL) by the user. No server-enforced ordering — users may
jump directly from NULL to `won`, or switch `lost` back to `interviewing`.
The timestamp for each state is set the first time that state is entered and
is never overwritten on subsequent transitions through the same state.

### 4. Dashboard Nav Changes

The nav link "Jobs" at `/` stays. Two new nav links are added:
- "Approved" → `/jobs/approved`
- "Rejected" → `/jobs/rejected`

This is a small template change to `layout.html`.

---

## Data Model

### New columns on `jobs` (migration 011)

```sql
ALTER TABLE jobs
  ADD COLUMN IF NOT EXISTS application_status TEXT
      CHECK(application_status IN ('applied','interviewing','lost','won')),
  ADD COLUMN IF NOT EXISTS applied_at        TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS interviewing_at   TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS lost_at           TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS won_at            TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_jobs_application_status
    ON jobs(user_id, application_status);
```

### Updated `models.Job` struct

Add five fields:
```go
ApplicationStatus string     // "", "applied", "interviewing", "lost", "won"
AppliedAt         *time.Time
InterviewingAt    *time.Time
LostAt            *time.Time
WonAt             *time.Time
```

Use `*time.Time` (pointer) so nil serializes cleanly and scans correctly from
nullable DB columns.

### New constants in `models`

```go
type ApplicationStatus string

const (
    AppStatusApplied      ApplicationStatus = "applied"
    AppStatusInterviewing ApplicationStatus = "interviewing"
    AppStatusLost         ApplicationStatus = "lost"
    AppStatusWon          ApplicationStatus = "won"
)

func (s ApplicationStatus) Valid() bool { ... }
```

---

## Store Layer

### New method: `UpdateApplicationStatus`

```go
// UpdateApplicationStatus sets application_status on a job and stamps the
// corresponding *_at column if it has not been set yet.
// The job must belong to userID and have status approved, generating, complete,
// or failed. Returns an error if the job is not in an approved pipeline stage.
func (s *Store) UpdateApplicationStatus(
    ctx context.Context,
    userID int64,
    jobID int64,
    status models.ApplicationStatus,
) error
```

Implementation:
1. Fetch the job (scoped by userID).
2. Validate that the job's `Status` is one of `approved|generating|complete|failed`.
3. Build an `UPDATE` that sets `application_status = $1` and conditionally sets
   the relevant `*_at` column using `COALESCE(existing_at, NOW())` so repeated
   calls to the same state don't overwrite the original timestamp.
4. Single `UPDATE` query — no transaction needed (single-row idempotent update).

SQL sketch:
```sql
UPDATE jobs SET
  application_status  = $1,
  applied_at          = CASE WHEN $1 = 'applied'      THEN COALESCE(applied_at, NOW())      ELSE applied_at      END,
  interviewing_at     = CASE WHEN $1 = 'interviewing' THEN COALESCE(interviewing_at, NOW()) ELSE interviewing_at END,
  lost_at             = CASE WHEN $1 = 'lost'         THEN COALESCE(lost_at, NOW())         ELSE lost_at         END,
  won_at              = CASE WHEN $1 = 'won'          THEN COALESCE(won_at, NOW())          ELSE won_at          END,
  updated_at          = NOW()
WHERE id = $2 AND user_id = $3
```

### Updated `scanJob`

Add scanning of the five new columns.

### Updated `ListJobsFilter`

Add `ApplicationStatus models.ApplicationStatus` field to allow filtering by
application status on the Approved page.

### `GetJob` and `ListJobs`

No interface changes needed — both already accept `userID` and return `*models.Job` /
`[]models.Job`; the new columns will be populated by `scanJob`.

---

## Web Layer

### `JobStore` interface additions

```go
// Add to JobStore interface in server.go:
UpdateApplicationStatus(ctx context.Context, userID int64, jobID int64, status models.ApplicationStatus) error
```

### New template data structs

```go
// approvedPageData mirrors dashboardData but for /jobs/approved
type approvedPageData struct {
    Jobs              []models.Job
    Statuses          []models.JobStatus      // approved, generating, complete, failed
    ActiveStatus      string
    Search            string
    Sort              string
    Order             string
    Columns           []columnDef
    CSRFToken         string
    User              *models.User
    ApplicationStatuses []models.ApplicationStatus // for <select> options
}

// rejectedPageData for /jobs/rejected
type rejectedPageData struct {
    Jobs      []models.Job
    Search    string
    Sort      string
    Order     string
    Columns   []columnDef
    CSRFToken string
    User      *models.User
}
```

### New handlers

```go
// handleApprovedJobs serves GET /jobs/approved
func (s *Server) handleApprovedJobs(w http.ResponseWriter, r *http.Request)

// handleRejectedJobs serves GET /jobs/rejected
func (s *Server) handleRejectedJobs(w http.ResponseWriter, r *http.Request)

// handleApprovedJobTablePartial serves GET /partials/approved-job-table
// Returns the tbody rows for the approved page (HTMX partial).
func (s *Server) handleApprovedJobTablePartial(w http.ResponseWriter, r *http.Request)

// handleSetApplicationStatus serves POST /api/jobs/{id}/application-status
// Reads form value "application_status", updates DB, returns updated tr fragment.
func (s *Server) handleSetApplicationStatus(w http.ResponseWriter, r *http.Request)
```

### Route additions in `Handler()`

```go
// Under optional-auth group:
r.Get("/jobs/approved", s.handleApprovedJobs)
r.Get("/jobs/rejected", s.handleRejectedJobs)
r.Get("/partials/approved-job-table", s.handleApprovedJobTablePartial)

// Under protected (requireAuth) group, inside /api/jobs route:
r.Post("/{id}/application-status", s.handleSetApplicationStatus)
```

**Note**: `/jobs/approved` and `/jobs/rejected` use `optionalAuth` (matching
the dashboard pattern) but the actual job listing is only rendered when a user
is present; anonymous visitors see the hero/landing block or a prompt to sign in.

### Dashboard `allStatuses` change

The existing `allStatuses` slice in `server.go` is used to build tabs.
Refactor into two separate slices:

```go
// dashboardStatuses are shown on the main dashboard (triage view).
var dashboardStatuses = []models.JobStatus{
    models.StatusDiscovered, models.StatusNotified,
}

// approvedPageStatuses are shown on the Approved Jobs page.
var approvedPageStatuses = []models.JobStatus{
    models.StatusApproved, models.StatusGenerating,
    models.StatusComplete, models.StatusFailed,
}
```

The `handleDashboard` and `handleJobTablePartial` handlers are updated to use
`dashboardStatuses`. The new approved-page handlers use `approvedPageStatuses`.

---

## Templates

### New templates

| File | Description |
|---|---|
| `internal/web/templates/approved_jobs.html` | Approved Jobs page — table with status selector column |
| `internal/web/templates/rejected_jobs.html` | Rejected Jobs page — read-only archive table |
| `internal/web/templates/partials/approved_job_rows.html` | HTMX row partial for approved page |

### Template changes

| File | Change |
|---|---|
| `internal/web/templates/layout.html` | Add "Approved" and "Rejected" links to nav |
| `internal/web/templates/dashboard.html` | Remove rejected/approved/generating/complete/failed tabs; keep only discovered/notified |

### `approved_job_rows.html` partial

Renders `<tr>` rows for the Approved page. Key difference from `job_rows.html`:
adds a `<select>` for `application_status` in the Actions column:

```html
{{define "approved_job_rows"}}
...
{{range .}}
<tr id="job-row-{{.ID}}">
  <td><a href="/jobs/{{.ID}}">{{.Title}}</a></td>
  <td>{{.Company}}</td>
  <td>{{.Location}}</td>
  <td>{{if .ExtractedSalary}}{{.ExtractedSalary}}{{else if .Salary}}{{.Salary}}{{else}}—{{end}}</td>
  <td><span class="status-badge status-{{.Status}}">{{.Status}}</span></td>
  <td>{{applicationStatusDate .}}</td>  <!-- e.g. "Applied Jan 2" or "—" -->
  <td>{{.DiscoveredAt.Format "Jan 2, 2006"}}</td>
  <td>
    <select name="application_status"
            hx-post="/api/jobs/{{.ID}}/application-status"
            hx-target="#job-row-{{.ID}}"
            hx-swap="outerHTML"
            hx-include="this">
      <option value="">— set status —</option>
      <option value="applied"      {{if eq .ApplicationStatus "applied"}}selected{{end}}>Applied</option>
      <option value="interviewing" {{if eq .ApplicationStatus "interviewing"}}selected{{end}}>Interviewing</option>
      <option value="lost"         {{if eq .ApplicationStatus "lost"}}selected{{end}}>Lost</option>
      <option value="won"          {{if eq .ApplicationStatus "won"}}selected{{end}}>Won</option>
    </select>
    {{if eq .Status "complete"}}
    <a href="/jobs/{{.ID}}" role="button" class="outline btn-sm">View</a>
    {{end}}
  </td>
</tr>
{{if .Summary}}
<tr class="job-summary-row">
  <td colspan="8">{{.Summary}}</td>
</tr>
{{end}}
{{end}}
{{end}}
```

A template function `applicationStatusDate` returns a short date string for the
most recent status transition (e.g. "Applied Jan 2, 2026" or "—").

```go
// In tmplFuncs:
"applicationStatusDate": func(job models.Job) string {
    switch models.ApplicationStatus(job.ApplicationStatus) {
    case models.AppStatusWon:
        if job.WonAt != nil { return "Won " + job.WonAt.Format("Jan 2, 2006") }
    case models.AppStatusLost:
        if job.LostAt != nil { return "Lost " + job.LostAt.Format("Jan 2, 2006") }
    case models.AppStatusInterviewing:
        if job.InterviewingAt != nil { return "Interviewing since " + job.InterviewingAt.Format("Jan 2, 2006") }
    case models.AppStatusApplied:
        if job.AppliedAt != nil { return "Applied " + job.AppliedAt.Format("Jan 2, 2006") }
    }
    return "—"
},
```

### CSS additions to `app.css`

Add status badge classes for application statuses:
```css
.status-applied      { background: var(--color-info-bg);    color: var(--color-info);    }
.status-interviewing { background: var(--color-warning-bg);  color: var(--color-warning); }
.status-won          { background: var(--color-success-bg);  color: var(--color-success); }
.status-lost         { background: var(--color-danger-bg);   color: var(--color-danger);  }
```

---

## Template Registration in `NewServerWithConfig`

New templates must be parsed and assigned:

```go
approvedJobsTmpl := template.Must(template.New("layout.html").Funcs(tmplFuncs).ParseFS(templateFS,
    "templates/layout.html",
    "templates/approved_jobs.html",
    "templates/partials/approved_job_rows.html",
))
rejectedJobsTmpl := template.Must(template.New("layout.html").Funcs(tmplFuncs).ParseFS(templateFS,
    "templates/layout.html",
    "templates/rejected_jobs.html",
    "templates/partials/job_rows.html", // rejected page reuses existing job_rows partial
))
```

Add `approvedJobsTmpl` and `rejectedJobsTmpl` fields to the `Server` struct.

---

## Implementation Steps

### Step 1 — DB Migration (migration 011)

**File**: `internal/store/migrations/011_add_application_status.sql`

```sql
ALTER TABLE jobs
  ADD COLUMN IF NOT EXISTS application_status TEXT
      CHECK(application_status IN ('applied','interviewing','lost','won')),
  ADD COLUMN IF NOT EXISTS applied_at        TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS interviewing_at   TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS lost_at           TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS won_at            TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_jobs_application_status
    ON jobs(user_id, application_status)
    WHERE application_status IS NOT NULL;
```

### Step 2 — Models

**File**: `internal/models/models.go`

1. Add `ApplicationStatus` type and constants.
2. Add five new fields to `Job` struct.

### Step 3 — Store

**Files**: `internal/store/store.go`

1. Update `scanJob` to scan the five new columns.
2. Add `UpdateApplicationStatus` method (see signature above).
3. Update `ListJobsFilter` with `ApplicationStatus` field.
4. Update `ListJobs` to filter by `application_status` when set.

### Step 4 — Web: handlers + routes

**File**: `internal/web/server.go`

1. Add `UpdateApplicationStatus` to `JobStore` interface.
2. Add `approvedJobsTmpl` and `rejectedJobsTmpl` fields to `Server` struct.
3. Add `applicationStatusDate` template function.
4. Split `allStatuses` into `dashboardStatuses` and `approvedPageStatuses`.
5. Update `dashboardData` and `handleDashboard` to use `dashboardStatuses`.
6. Add `approvedPageData` and `rejectedPageData` structs.
7. Add `handleApprovedJobs`, `handleRejectedJobs`,
   `handleApprovedJobTablePartial`, `handleSetApplicationStatus` handlers.
8. Register routes.

### Step 5 — Templates

1. Create `internal/web/templates/approved_jobs.html`
2. Create `internal/web/templates/rejected_jobs.html`
3. Create `internal/web/templates/partials/approved_job_rows.html`
4. Update `internal/web/templates/layout.html` — add nav links.
5. Update `internal/web/templates/dashboard.html` — remove non-triage status tabs.
6. Update `internal/web/templates/partials/job_rows.html` — remove approve/reject
   buttons for statuses that won't appear on the dashboard anymore (optional
   cleanup; the button logic already guards by status).

### Step 6 — CSS

**File**: `internal/web/templates/static/app.css`

Add `application_status` badge styles (section 4 additions).

### Step 7 — Tests

**Store tests** (`internal/store/store_test.go` or new `application_status_test.go`):
- `TestUpdateApplicationStatus_SetsStatusAndTimestamp`
- `TestUpdateApplicationStatus_PreservesOriginalTimestamp`
- `TestUpdateApplicationStatus_RejectsNonApprovedJob`
- `TestListJobsByApplicationStatus`

**Web/handler tests** (`internal/web/server_test.go` or new file):
- `TestHandleApprovedJobs_RequiresAuth`
- `TestHandleRejectedJobs_RequiresAuth`
- `TestHandleSetApplicationStatus_HTMXResponse`
- `TestHandleSetApplicationStatus_InvalidStatus`
- `TestHandleSetApplicationStatus_NonApprovedJob`

---

## Trade-offs and Alternatives

### A. Current status + timestamps (chosen) vs. Event log table

**Chosen**: Five nullable columns on `jobs`.
- Pro: no join needed to render the page; migration is additive; simple.
- Pro: idempotent — calling `applied` twice doesn't duplicate data.
- Con: no history; if a user cycles through states, only the first entry of
  each state is preserved (by design via COALESCE).
- Con: can't audit "how many times did they re-enter interviewing?"

**Alternative**: `job_application_events(id, job_id, status, occurred_at)` log table.
- Pro: full history.
- Con: requires a join or subquery to get the current state; more complex.
- Decision: current-state model is sufficient for the stated requirements
  (track "when did X happen"). The log can be added later as migration 012 if
  product requests it without changing the existing columns.

### B. HTMX `<select>` vs. per-status buttons vs. modal

**Chosen**: HTMX `<select>`.
- Pro: matches existing approve/reject HTMX pattern; no custom JS.
- Pro: accessible natively; keyboard navigable.
- Con: visually less prominent than a button.

**Alternative**: Individual action buttons (Applied / Interviewing / Lost / Won).
- Pro: more discoverable.
- Con: clutters the row with 4 buttons when only 1 is relevant.

**Alternative**: Modal/drawer with a form.
- Pro: can show all dates and add notes.
- Con: requires more JS, doesn't match current no-JS-framework convention.

### C. Dashboard scope: remove all non-triage tabs vs. keep them

**Chosen**: Dashboard shows only `discovered` + `notified`.
- Pro: clear mental model — dashboard = inbox; approved/rejected = separate pages.
- Con: users who bookmarked `/?status=complete` will need to update bookmarks.

**Alternative**: Keep all tabs on dashboard, duplicate approved/rejected pages.
- Pro: backward compatible.
- Con: confusing duplication; application status would need to appear on both
  pages or only one.

---

## Acceptance Criteria

- [ ] `/jobs/approved` shows all jobs with status `approved`, `generating`,
  `complete`, or `failed`, filterable by those sub-statuses via tab bar.
- [ ] `/jobs/rejected` shows all jobs with status `rejected`.
- [ ] The main dashboard `/` shows only `discovered` and `notified` jobs.
- [ ] Each row on the Approved page has a `<select>` for application status
  (applied / interviewing / lost / won) that submits inline via HTMX without
  a page reload.
- [ ] Selecting an application status records the current timestamp in the
  corresponding `*_at` column the first time that status is entered; re-selecting
  the same status does not overwrite the timestamp.
- [ ] Application status dates are displayed in the Approved page row
  (e.g. "Applied Jan 2, 2026").
- [ ] The nav bar has links to "Approved" and "Rejected" pages.
- [ ] All existing tests pass. New store and handler tests cover the new
  `UpdateApplicationStatus` path and the new route handlers.
- [ ] No JS changes beyond what HTMX already provides.

---

## Dependencies / Prerequisites

- No new Go modules required.
- PostgreSQL migration 011 is additive (nullable columns, no breaking changes).
- HTMX 1.9.12 already loaded via CDN — the `hx-include="this"` pattern for
  `<select>` is supported in 1.9.x.
- The `hx-vals` pattern already used in `dashboard.html` can be reused on the
  Approved page for tab switching.
