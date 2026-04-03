# Task: job-pipeline-5-code-review

- **Type**: code-reviewer
- **Status**: pending
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

