# Task: job-pipeline-6-qa

- **Type**: qa
- **Status**: pending
- **Parallel Group**: 5
- **Branch**: feature/job-pipeline-3-web
- **Source Item**: job-pipeline-pages (plans/job-pipeline-pages.md)
- **Dependencies**: job-pipeline-5-code-review

## Description

Write and run comprehensive tests for the job-pipeline-pages feature. Cover the
store-layer `UpdateApplicationStatus` method, the new web handlers, and the
dashboard tab refactor. Run the full test suite and report results.

## Acceptance Criteria

- [ ] Store tests pass:
  - `TestUpdateApplicationStatus_SetsStatusAndTimestamp` — verifies status and the correct `*_at` column are set
  - `TestUpdateApplicationStatus_PreservesOriginalTimestamp` — verifies calling the same status twice does not overwrite the first timestamp
  - `TestUpdateApplicationStatus_RejectsNonApprovedJob` — verifies an error is returned for a `discovered`/`notified`/`rejected` job
  - `TestListJobsByApplicationStatus` — verifies `ListJobs` with `ListJobsFilter.ApplicationStatus` set returns only matching jobs
- [ ] Handler tests pass:
  - `TestHandleApprovedJobs_RequiresAuth` — unauthenticated GET returns landing/sign-in prompt, not a 500
  - `TestHandleRejectedJobs_RequiresAuth` — same for `/jobs/rejected`
  - `TestHandleSetApplicationStatus_HTMXResponse` — POST with valid `application_status` returns a `<tr>` fragment with the updated status
  - `TestHandleSetApplicationStatus_InvalidStatus` — POST with unknown status value returns HTTP 400
  - `TestHandleSetApplicationStatus_NonApprovedJob` — POST against a `discovered` job returns HTTP 403 or 404
- [ ] Dashboard handler test confirms `/?status=approved` does not appear as a tab (or returns gracefully)
- [ ] `go test ./...` exits 0 with no new failures
- [ ] Any bugs found are logged in `.workflow/BUGS.md` with file, line, severity, and reproduction steps

## Interface Contracts

```go
// Store method under test:
UpdateApplicationStatus(ctx context.Context, userID int64, jobID int64, status models.ApplicationStatus) error

// Handler endpoints under test:
GET  /jobs/approved
GET  /jobs/rejected
POST /api/jobs/{id}/application-status  (body: application_status=applied|interviewing|lost|won)
GET  /partials/approved-job-table
```

`POST /api/jobs/{id}/application-status` returns a replacement `<tr id="job-row-{id}">` on success.

## Context

- Test files: add store tests in `internal/store/application_status_test.go` (or
  `internal/store/store_test.go`), web tests in `internal/web/server_test.go` or
  a new `internal/web/approved_jobs_test.go`.
- Use the existing test helpers in `internal/store/store_test.go` and
  `internal/web/server_test.go` to set up test databases and HTTP test servers.
- The testing command from `agent.yaml` is `go test ./...`.
- If tests already exist from the coder tasks, run them and extend coverage where
  gaps exist. Do not duplicate passing tests.

## Notes

