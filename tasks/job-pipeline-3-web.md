# Task: job-pipeline-3-web

- **Type**: coder
- **Status**: pending
- **Parallel Group**: 3
- **Branch**: feature/job-pipeline-3-web
- **Source Item**: job-pipeline-pages (plans/job-pipeline-pages.md)
- **Dependencies**: job-pipeline-2-models-store

## Description

Extend `internal/web/server.go` with the two new page handlers (`/jobs/approved`,
`/jobs/rejected`), the approved-table partial handler, and the
`POST /api/jobs/{id}/application-status` HTMX endpoint. Refactor the dashboard
status list, add new page data structs, wire all routes, and register the new
templates.

## Acceptance Criteria

- [ ] `JobStore` interface in `server.go` includes `UpdateApplicationStatus(ctx context.Context, userID int64, jobID int64, status models.ApplicationStatus) error`
- [ ] `allStatuses` is replaced by `dashboardStatuses` (discovered, notified) and `approvedPageStatuses` (approved, generating, complete, failed)
- [ ] `handleDashboard` and `handleJobTablePartial` use `dashboardStatuses` (dashboard no longer shows approved/rejected/generating/complete/failed tabs)
- [ ] `approvedPageData` and `rejectedPageData` structs are defined with all fields per the plan
- [ ] `handleApprovedJobs` serves `GET /jobs/approved`, queries with `approvedPageStatuses`, populates `approvedPageData`, renders `approvedJobsTmpl`
- [ ] `handleRejectedJobs` serves `GET /jobs/rejected`, queries with `StatusRejected`, populates `rejectedPageData`, renders `rejectedJobsTmpl`
- [ ] `handleApprovedJobTablePartial` serves `GET /partials/approved-job-table`, returns tbody HTML for HTMX refresh on the Approved page
- [ ] `handleSetApplicationStatus` serves `POST /api/jobs/{id}/application-status`, reads form value `application_status`, calls `store.UpdateApplicationStatus`, returns replacement `<tr>` fragment via the `approved_job_rows` partial
- [ ] All four handlers are registered in `Handler()` at the correct paths
- [ ] `approvedJobsTmpl` and `rejectedJobsTmpl` are parsed in `NewServerWithConfig` and stored as `Server` struct fields
- [ ] `applicationStatusDate` template function is registered in `tmplFuncs`
- [ ] `handleSetApplicationStatus` returns HTTP 400 for an invalid `application_status` value
- [ ] `handleSetApplicationStatus` returns HTTP 403/404 when the job does not belong to the authenticated user or is not in an approved-pipeline stage
- [ ] All existing handler tests continue to pass

## Interface Contracts

Store method added in task `job-pipeline-2-models-store`:
```go
// On JobStore interface:
UpdateApplicationStatus(ctx context.Context, userID int64, jobID int64, status models.ApplicationStatus) error
```

Models from `internal/models/models.go` (task `job-pipeline-2-models-store`):
```go
type ApplicationStatus string

const (
    AppStatusApplied      ApplicationStatus = "applied"
    AppStatusInterviewing ApplicationStatus = "interviewing"
    AppStatusLost         ApplicationStatus = "lost"
    AppStatusWon          ApplicationStatus = "won"
)

// On models.Job — five new fields available for template data:
ApplicationStatus string
AppliedAt         *time.Time
InterviewingAt    *time.Time
LostAt            *time.Time
WonAt             *time.Time
```

Route layout additions:
```
GET  /jobs/approved                       → handleApprovedJobs          (optionalAuth)
GET  /jobs/rejected                       → handleRejectedJobs          (optionalAuth)
GET  /partials/approved-job-table         → handleApprovedJobTablePartial (optionalAuth)
POST /api/jobs/{id}/application-status    → handleSetApplicationStatus  (requireAuth)
```

`handleSetApplicationStatus` reads form body field `application_status` and
returns a replacement `<tr id="job-row-{id}">` fragment (rendered via the
`approved_job_rows` partial template defined in task `job-pipeline-4-templates`).

## Context

- Primary file: `internal/web/server.go`
- Template parsing: `approvedJobsTmpl` parses `layout.html` + `approved_jobs.html` +
  `partials/approved_job_rows.html`. `rejectedJobsTmpl` parses `layout.html` +
  `rejected_jobs.html` + `partials/job_rows.html`.
- `applicationStatusDate` template function logic (register in `tmplFuncs` map):
  ```go
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
  }
  ```
- New handler tests go in `internal/web/server_test.go` or a new file. Required:
  - `TestHandleApprovedJobs_RequiresAuth`
  - `TestHandleRejectedJobs_RequiresAuth`
  - `TestHandleSetApplicationStatus_HTMXResponse`
  - `TestHandleSetApplicationStatus_InvalidStatus`
  - `TestHandleSetApplicationStatus_NonApprovedJob`

## Notes

