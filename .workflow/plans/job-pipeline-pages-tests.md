# Test Plan: job-pipeline-pages

**Feature**: Job pipeline pages (approved/rejected views, application status updates)
**Task**: job-pipeline-6-qa
**Branch**: feature/job-pipeline-3-4-integrated

## Existing Coverage (from coder tasks)

### Store — `internal/store/application_status_test.go`
| Test | Status |
|------|--------|
| TestUpdateApplicationStatus_SetsStatusAndTimestamp | Pre-existing PASS |
| TestUpdateApplicationStatus_PreservesOriginalTimestamp | Pre-existing PASS |
| TestUpdateApplicationStatus_RejectsNonApprovedJob | Pre-existing PASS |
| TestListJobsByApplicationStatus | Pre-existing PASS |

### Web handlers — `internal/web/server_test.go`
| Test | Status |
|------|--------|
| TestHandleApprovedJobs_RequiresAuth | Pre-existing PASS |
| TestHandleRejectedJobs_RequiresAuth | Pre-existing PASS |
| TestHandleSetApplicationStatus_HTMXResponse | Pre-existing PASS |
| TestHandleSetApplicationStatus_InvalidStatus | Pre-existing PASS |
| TestHandleSetApplicationStatus_NonApprovedJob | Pre-existing PASS |

## Gaps Identified and Filled

### Store — `internal/store/application_status_test.go` (appended)
| Test | Gap addressed |
|------|--------------|
| TestUpdateApplicationStatus_UserScoping | No prior test verified cross-user update was rejected at the store layer. Store uses GetJob internally with userID scoping → attacker's update returns "not found" error. |

### Web — `internal/web/job_pipeline_qa_test.go` (new file)
| Test | Gap addressed |
|------|--------------|
| TestHandleSetApplicationStatus_MissingJob | No prior test for the 404 path (absent job ID). Handler calls GetJob → "not found" → 404. |
| TestHandleSetApplicationStatus_HTMXRowFragment | Existing test checked status 200 + Content-Type but not the `<tr id="job-row-{id}">` body requirement from the interface contract. |
| TestDashboard_NoApprovedRejectedTabs | No prior test confirmed that `?status=approved` and `?status=rejected` do not appear as tab links. Unauthenticated view renders landing page (neither tab should appear). |
| TestDashboard_TabsContainDiscoveredNotified | Complementary positive assertion: authenticated view renders `?status=discovered` and `?status=notified` tabs and still excludes approved/rejected. |

## Static Review Findings

No bugs found. All code paths were reviewed:

- `handleSetApplicationStatus`: properly validates status, guards pipeline stage
  via `pipelineStatusAllowed`, re-fetches job before rendering HTMX fragment.
- `handleApprovedJobs` / `handleRejectedJobs`: gracefully handle unauthenticated
  users (userID=0) — store returns no rows, template renders empty list.
- `handleDashboard`: uses `dashboardStatuses = [discovered, notified]` only;
  the dashboard template has hardcoded tab links (no dynamic status loop).
- Store `UpdateApplicationStatus`: uses `COALESCE` for timestamp preservation,
  scopes both GetJob and UPDATE by userID, validates pipeline stage.

## Test Infrastructure Note

Store tests require `TEST_DATABASE_URL` env var. They are skipped automatically
when the env var is not set. Web handler tests use an in-memory mock and run
without any external dependencies.

## Go Availability

`go` binary was not present in the container. All tests were written and
validated via static review of the implementation code, template output, and
mock behavior.
