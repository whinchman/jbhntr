# Task: job-pipeline-4-templates

- **Type**: coder
- **Status**: done
- **Parallel Group**: 3
- **Branch**: feature/job-pipeline-4-templates
- **Source Item**: job-pipeline-pages (plans/job-pipeline-pages.md)
- **Dependencies**: job-pipeline-2-models-store

## Description

Create the two new page templates (`approved_jobs.html`, `rejected_jobs.html`),
the `approved_job_rows.html` HTMX partial, update `layout.html` to add the nav
links, update `dashboard.html` to remove non-triage status tabs, and add
application-status badge CSS to `app.css`.

## Acceptance Criteria

- [ ] `internal/web/templates/approved_jobs.html` exists and renders the Approved Jobs page with a tab bar for `approved`, `generating`, `complete`, `failed` statuses plus an `all` tab
- [ ] `internal/web/templates/rejected_jobs.html` exists and renders the Rejected Jobs archive as a read-only table
- [ ] `internal/web/templates/partials/approved_job_rows.html` exists with the `{{define "approved_job_rows"}}` template block that renders `<tr id="job-row-{{.ID}}">` rows
- [ ] Each row in `approved_job_rows.html` includes a `<select>` for `application_status` wired with `hx-post="/api/jobs/{{.ID}}/application-status"`, `hx-target="#job-row-{{.ID}}"`, `hx-swap="outerHTML"`, `hx-include="this"`
- [ ] The `<select>` options are: `— set status —` (value ""), `Applied` (value "applied"), `Interviewing` (value "interviewing"), `Lost` (value "lost"), `Won` (value "won"); the currently-set value is pre-selected
- [ ] Each row shows the `applicationStatusDate` output (e.g. "Applied Jan 2, 2026" or "—") in a dedicated column
- [ ] `internal/web/templates/layout.html` nav bar includes links "Approved" → `/jobs/approved` and "Rejected" → `/jobs/rejected`
- [ ] `internal/web/templates/dashboard.html` tab bar shows only `all`, `discovered`, `notified` (removes `approved`, `rejected`, `generating`, `complete`, `failed` tabs)
- [ ] `internal/web/templates/static/app.css` includes badge classes `.status-applied`, `.status-interviewing`, `.status-won`, `.status-lost` using the existing CSS variable system
- [ ] Pages are visually consistent with the existing dashboard and job-detail pages

## Interface Contracts

The `approved_job_rows.html` partial is rendered by two different handlers:
`handleApprovedJobTablePartial` (returns a full `<tbody>`) and
`handleSetApplicationStatus` (returns a single replacement `<tr>`). The partial
must support both use cases — the handler passes either `[]models.Job` (for the
table) or a single `models.Job` wrapped in a slice (for the row replacement).

Template data structs (defined in task `job-pipeline-3-web`, `server.go`):

```go
type approvedPageData struct {
    Jobs                []models.Job
    Statuses            []models.JobStatus       // approved, generating, complete, failed
    ActiveStatus        string
    Search              string
    Sort                string
    Order               string
    Columns             []columnDef
    CSRFToken           string
    User                *models.User
    ApplicationStatuses []models.ApplicationStatus  // for <select> option list
}

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

Template function available for use in templates:
```
applicationStatusDate(job models.Job) string
```
Returns a human-readable status date string like "Applied Jan 2, 2026" or "—".

## Context

- Study `internal/web/templates/dashboard.html` and `internal/web/templates/partials/job_rows.html`
  as reference for HTMX patterns, tab bar markup, and table structure. Mirror
  the same PicoCSS/class conventions.
- The approved page's tab bar should use `hx-vals` (as seen on the dashboard)
  to switch between status filters without full page reloads.
- `approved_job_rows.html` key structure (from plan):
  ```html
  {{define "approved_job_rows"}}
  {{range .}}
  <tr id="job-row-{{.ID}}">
    <td><a href="/jobs/{{.ID}}">{{.Title}}</a></td>
    <td>{{.Company}}</td>
    <td>{{.Location}}</td>
    <td>... salary ...</td>
    <td><span class="status-badge status-{{.Status}}">{{.Status}}</span></td>
    <td>{{applicationStatusDate .}}</td>
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
  <tr class="job-summary-row"><td colspan="8">{{.Summary}}</td></tr>
  {{end}}
  {{end}}
  {{end}}
  ```
- CSS badge classes to add to `internal/web/templates/static/app.css`:
  ```css
  .status-applied      { background: var(--color-info-bg);    color: var(--color-info);    }
  .status-interviewing { background: var(--color-warning-bg); color: var(--color-warning); }
  .status-won          { background: var(--color-success-bg); color: var(--color-success); }
  .status-lost         { background: var(--color-danger-bg);  color: var(--color-danger);  }
  ```

## Notes

Implementation complete on branch `feature/job-pipeline-4-templates` (worktree at `/workspace/worktrees/job-pipeline-4-templates`), based off `feature/job-pipeline-2-models-store`.

### What was implemented

- `internal/web/templates/approved_jobs.html` — Approved Jobs page with tab bar for `approved`, `generating`, `complete`, `failed`, and `all` statuses. Uses HTMX to swap `#approved-job-table-body` on tab/search changes with 30s auto-refresh.
- `internal/web/templates/rejected_jobs.html` — Rejected Jobs archive as a read-only table with search input and sortable headers via HTMX.
- `internal/web/templates/partials/approved_job_rows.html` — `{{define "approved_job_rows"}}` partial that renders `<tr id="job-row-{{.ID}}>` rows. Each row includes the `<select>` for `application_status` wired with `hx-post="/api/jobs/{{.ID}}/application-status"`, `hx-target`, `hx-swap="outerHTML"`, `hx-include="this"`. The `applicationStatusDate` template function is called per row. Compatible with both `[]models.Job` (table render) and a single-item slice (row replacement).
- `internal/web/templates/layout.html` — Added "Approved" (`/jobs/approved`) and "Rejected" (`/jobs/rejected`) nav links.
- `internal/web/templates/dashboard.html` — Tab bar replaced with hardcoded `all`, `discovered`, `notified` tabs only (removed `approved`, `rejected`, `generating`, `complete`, `failed`).
- `internal/web/templates/static/app.css` — Added `.status-applied`, `.status-interviewing`, `.status-won`, `.status-lost` badge classes using existing CSS variable tokens.

### Test results

Go is not installed in this container; `go test ./...` could not be executed. Template syntax was verified by manual review. No Go source files were modified.
