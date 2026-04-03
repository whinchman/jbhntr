# Task: job-pipeline-5-code-review

- **Type**: code-reviewer
- **Status**: done
- **Parallel Group**: 4
- **Branch**: feature/job-pipeline-3-web
- **Source Item**: job-pipeline-pages (plans/job-pipeline-pages.md)
- **Dependencies**: job-pipeline-3-web, job-pipeline-4-templates

## Description

Review all code changes introduced by the job-pipeline-pages feature across
branches `feature/job-pipeline-1-migration`, `feature/job-pipeline-2-models-store`,
`feature/job-pipeline-3-web`, and `feature/job-pipeline-4-templates`. Check for
correctness, security, adherence to project conventions, and completeness against
the acceptance criteria in the plan.

## Acceptance Criteria

- [ ] Migration 011 is additive and safe (no data loss, IF NOT EXISTS guards)
- [ ] `ApplicationStatus.Valid()` covers all four known values and rejects empty/unknown
- [ ] `UpdateApplicationStatus` correctly enforces job ownership (`user_id`) and pipeline stage validation
- [ ] COALESCE logic in the UPDATE SQL correctly preserves original timestamps on repeated calls
- [ ] `handleSetApplicationStatus` validates `application_status` input before calling the store
- [ ] `handleSetApplicationStatus` returns appropriate HTTP error codes (400 for bad input, 403/404 for wrong user/non-approved job)
- [ ] New routes are registered under correct auth middleware (approved/rejected pages under `optionalAuth`, the POST endpoint under `requireAuth`)
- [ ] Template HTMX wiring (`hx-post`, `hx-target`, `hx-swap`, `hx-include`) is correct and consistent with existing patterns
- [ ] Dashboard no longer shows tabs for `approved`, `rejected`, `generating`, `complete`, `failed`
- [ ] No SQL injection risks, no hardcoded credentials, no missing error handling
- [ ] All findings written to task Notes; critical/warning findings also appended to `.workflow/BUGS.md`

## Interface Contracts

Same as tasks `job-pipeline-2-models-store`, `job-pipeline-3-web`, and
`job-pipeline-4-templates`. No new contracts — this task verifies that the
contracts were honored correctly.

## Context

- Review branches in order: migration → models/store → web → templates
- Pay particular attention to:
  1. The COALESCE timestamp preservation logic in `UpdateApplicationStatus`
  2. The job ownership + pipeline-stage validation in `handleSetApplicationStatus`
  3. CSRF token presence on the HTMX form (the `<select>` posts via HTMX — verify
     CSRF protection is not bypassed)
  4. The dashboard tab removal — ensure no existing URLs (`/?status=approved` etc.)
     produce a broken/empty page rather than a redirect
- Reference: `plans/job-pipeline-pages.md` for full acceptance criteria

## Notes

### Review Summary — 2026-04-03

**Findings: 0 critical, 3 warning, 3 info. Verdict: request-changes.**

The request-changes verdict is driven entirely by three inter-branch conflicts and the
missing `/partials/rejected-job-table` endpoint (BUG-029). The handler code in branch 3
and the store/model code from branches 1–2 are correct.

---

#### Branches 1 and 2 (previously reviewed — confirming acceptance criteria still met)

- Migration 011: additive, IF NOT EXISTS guards, CHECK constraint on application_status. ✓
- `ApplicationStatus.Valid()` covers all four constants, rejects empty/unknown. ✓
- `UpdateApplicationStatus` pre-fetches job (enforcing ownership via user_id and pipeline-stage
  validation), then runs COALESCE UPDATE. Timestamp preservation is correct. ✓
- `scanJob` uses `sql.NullString` for `application_status`. ✓
- `migrate_test.go` updated with migration 010 and 011 entries. ✓

---

#### Branch 3 (feature/job-pipeline-3-web) — new findings

**[WARNING] BUG-027 — `rejected_jobs.html` uses `job_rows` partial: 7-column rows vs 5-column header**
- File: `internal/web/templates/rejected_jobs.html`, `internal/web/templates/partials/job_rows.html`
- The branch 3 rejected page borrows the general `job_rows` partial, which renders 7 columns and
  `colspan="7"` on empty state. The rejected_jobs.html header only has 5 `<th>` elements.
  Visual layout is broken (Status and Actions columns rendered without headers).
- Fix: replace `{{template "job_rows" .Jobs}}` with an inline row loop that matches the 5 headers,
  or use the branch 4 version of `rejected_jobs.html` which is self-contained and correct.

**[INFO] Test name `TestHandleApprovedJobs_RequiresAuth` is misleading**
- File: `internal/web/server_test.go`
- The test asserts that `/jobs/approved` returns 200 for an unauthenticated request (correctly
  so — the route is under optionalAuth). The name "RequiresAuth" implies the opposite of what
  is being tested. Should be renamed `TestHandleApprovedJobs_UnauthReturns200` or similar.

**[INFO] Duplicate pipeline-stage guard: handler calls `pipelineStatusAllowed(job.Status)` then
store `UpdateApplicationStatus` also checks `pipelineStatuses[job.Status]`**
- File: `internal/web/server.go:handleSetApplicationStatus`, `internal/store/store.go:UpdateApplicationStatus`
- Both layers enforce the pipeline-stage check independently. Not a bug — defence-in-depth is
  fine — but the duplication means the error message surfaces differently depending on which
  layer fires. Acceptable as-is; note for future maintainers.

**Acceptance criteria verified for branch 3:**
- `handleSetApplicationStatus` validates `application_status` before calling store. ✓
- Returns 400 for bad input, 403 for non-pipeline-stage job, 404 for missing job. ✓
- POST `/api/jobs/{id}/application-status` is under `requireAuth`. ✓
- `/jobs/approved` and `/jobs/rejected` are under `optionalAuth`. ✓
- `dashboardStatuses` replaces `allStatuses`; dashboard no longer passes
  Approved/Rejected/Generating/Complete/Failed tabs. ✓
- CSRF: HTMX `<select>` POSTs inherit `X-CSRF-Token` from layout's `hx-headers` on `document.body`.
  CSRF is correctly handled. ✓
- `approvedPageData.CSRFToken` and `rejectedPageData.CSRFToken` are populated in both handlers. ✓
- `handleApprovedJobTablePartial` correctly uses `approvedJobsTmpl.ExecuteTemplate(w, "approved_job_rows", jobs)`. ✓
- Template parsing uses `template.New("layout.html")` consistent with all other layout-bearing templates. ✓
- `approvedJobsTmpl` includes `approved_job_rows.html` partial needed for HTMX row swap. ✓
- `rejectedJobsTmpl` parses `job_rows.html` (which exists and is valid); the visual mismatch is
  flagged as BUG-027.

---

#### Branch 4 (feature/job-pipeline-4-templates) — new findings

**[WARNING] BUG-029 — `rejected_jobs.html` references `/partials/rejected-job-table` which is never registered**
- File: `internal/web/templates/rejected_jobs.html` (branch 4)
- The search input and all sortable column headers are wired to `hx-get="/partials/rejected-job-table"`.
  No handler for this route exists in either branch 3 or branch 4. All search/sort HTMX calls
  will return 404.
- Fix: add `handleRejectedJobTablePartial` and register `GET /partials/rejected-job-table` under
  optionalAuth, mirroring the approved table partial; or use the branch 3 static rejected_jobs.html
  and forgo HTMX search/sort on that page.

**[WARNING] BUG-028 — `approved_job_rows.html` renders 8 cells; `approved_jobs.html` header has 7 columns**
- File: `internal/web/templates/partials/approved_job_rows.html` (branch 4),
  `internal/web/templates/approved_jobs.html` (branch 4)
- Row template: 8 cells (Title, Company, Location, Salary, Status-badge, applicationStatusDate,
  DiscoveredAt, Application-select). Header: 6 from `{{range .Columns}}` + 1 "Application Status" = 7.
  The `applicationStatusDate` cell (col 6) has no header; "Application Status" header labels the
  `DiscoveredAt` cell instead.
- Fix: add a `<th>Status Date</th>` between the `{{range .Columns}}` block and
  `<th>Application Status</th>`, or remove the standalone `applicationStatusDate` column from rows
  and embed it in the Application-select cell.

**[INFO] Branch 4 `approved_jobs.html` starts with `{{template "layout.html" .}}` (dead code)**
- File: `internal/web/templates/approved_jobs.html` (branch 4)
- The `ExecuteTemplate(w, "layout.html", data)` call in the handler drives rendering; the
  `{{template "layout.html" .}}` line at the top of the file is outside any `{{define}}` block
  and is never directly executed. This matches the established pattern in `dashboard.html`,
  `settings.html`, etc. — it is harmless but technically dead code. The branch 3 version of
  `approved_jobs.html` (which omits this line) is the more minimal pattern.

**Accepted as canonical (branch 4 is better than branch 3 in these respects):**
- `approved_job_rows.html` (branch 4): uses `hx-include="this"` (correct — scopes the POST to
  just the select value), adds a Summary row (consistent with `job_rows.html`), more descriptive
  placeholder text "— set status —", and separates applicationStatusDate and DiscoveredAt.
  The branch 4 version is preferred for the merge **once BUG-028 is fixed**.
- `dashboard.html` (branch 4): hardcodes `discovered` and `notified` tabs instead of ranging over
  `.Statuses`, which is clearer and makes the dashboard scope explicit.
- `layout.html` (branch 4): adds "Approved" and "Rejected" nav links. Correct.
- `app.css` (branch 4): adds `.status-applied`, `.status-interviewing`, `.status-won`, `.status-lost`
  badge classes. Necessary and correct.

---

#### Template Conflict Summary (MERGE CRITICAL — Coordinator must resolve)

Three files are created by **both** branches. The Coordinator must choose one version per file when
merging. Recommended canonical versions:

| File | Use branch 3 version | Use branch 4 version | Reason |
|------|----------------------|----------------------|--------|
| `templates/approved_jobs.html` | | ✓ (after fixing BUG-028: add missing "Status Date" header `<th>`) | Full HTMX tabs, search, polling, sortable columns |
| `templates/partials/approved_job_rows.html` | | ✓ (after fixing BUG-028) | hx-include="this", summary row, better placeholder |
| `templates/rejected_jobs.html` | ✓ (static table) | | Branch 4 version has BUG-029 (missing route) |

Additionally, branch 4 adds changes to `layout.html`, `dashboard.html`, and `app.css` that are
**not present in branch 3** — these must all be carried forward from branch 4.

---

**Review findings totals: 0 critical, 3 warning, 3 info.**
**Verdict: request-changes** — BUG-027, BUG-028, and BUG-029 must be fixed before QA.
BUG-027 and BUG-028 can be fixed by patching the rejected/approved templates.
BUG-029 requires either a new handler (preferred) or substituting the branch-3 static rejected template.

