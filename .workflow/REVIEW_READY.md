# Review Ready: job-pipeline-pages

**Date:** 2026-04-03
**Feature:** Job Pipeline Pages + Application Status Tracking
**Plan:** plans/job-pipeline-pages.md

## Summary

The feature is merged to `development` (local). Review the diff, then push when satisfied.

```
git -C /workspace log origin/development..development --oneline
git -C /workspace diff origin/development..development
git -C /workspace push origin development
```

## Summary of Changes

**Migration 011** (`internal/store/migrations/011_add_application_status.sql`): adds `application_status TEXT CHECK('applied','interviewing','lost','won')` (nullable) and four nullable `TIMESTAMPTZ` columns to `jobs`, plus a partial index on `(user_id, application_status)`.

**Models** (`internal/models/models.go`): `ApplicationStatus` type with constants; `Job` struct extended with 5 new fields.

**Store** (`internal/store/store.go`): `UpdateApplicationStatus` (COALESCE-based, pipeline-stage guard, user-scoped); `scanJob` extended; `ListJobsFilter.ApplicationStatus` filter.

**Web** (`internal/web/server.go`): `dashboardStatuses` replaces `allStatuses` (dashboard triage-only); four new handlers for approved/rejected pages and application status updates; `applicationStatusDate` template function; routes under correct auth groups.

**Templates**: `approved_jobs.html` (8-column table, HTMX tab bar), `rejected_jobs.html` (5-column static archive), `partials/approved_job_rows.html` (HTMX `<select>` for inline status), `layout.html` (nav links), `dashboard.html` (trimmed to 3 tabs), `app.css` (4 status badge classes).

## Validation Summary

| Check | Result |
|-------|--------|
| Code Review — migration | PASS |
| Code Review — models/store | PASS |
| Code Review — web/templates | PASS (BUGs 027/028/029 fixed in integration pass) |
| QA | PASS — 5 gap-filling tests, static review |

## Known Bugs Logged (non-blocking)

| Bug | Severity | Description |
|-----|----------|-------------|
| BUG-024 | Warning | `migrate_test.go` INSERT subtests may fail on persistent DB (duplicate key) |
| BUG-025 | Warning | `UpdateApplicationStatus` missing `userID==0` worker path |
| BUG-026 | Warning | Mock `ListJobs` ignores `ApplicationStatus` filter |

## Manual Steps Required

None.
