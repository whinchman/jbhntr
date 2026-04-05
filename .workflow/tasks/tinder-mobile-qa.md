# Task: tinder-mobile-qa

- **Type**: qa
- **Status**: pending
- **Parallel Group**: 3
- **Branch**: feature/tinder-mobile-backend (tests can be added to either branch; coordinate with the coder who owns server_test.go)
- **Source Item**: Tinder-Style Mobile Job Review UI
- **Dependencies**: tinder-mobile-backend, tinder-mobile-frontend, tinder-mobile-code-review

## Description

Write and run Go tests for the new swipe-card backend endpoints. Tests go in `internal/web/` — either in a new file `internal/web/job_cards_partial_test.go` or appended to the appropriate existing test file. Run the full test suite (`go test ./...`) and confirm all tests pass.

## Acceptance Criteria

- [ ] Test: `GET /partials/job-cards` with HTMX headers (`HX-Request: true`) and authenticated user returns 200 with HTML containing `job-card-active` class when jobs exist
- [ ] Test: `GET /partials/job-cards` returns empty-state HTML (containing `job-card-empty` class) when no jobs exist for the user
- [ ] Test: `GET /partials/job-cards` unauthenticated returns 200 with empty body (not a redirect)
- [ ] Test: `POST /api/jobs/{id}/approve` with `HX-Request: true` and `HX-Target: job-card-deck` renders `job_cards` template (response contains `job-card` class, not table rows)
- [ ] Test: `POST /api/jobs/{id}/reject` with `HX-Request: true` and `HX-Target: job-card-deck` renders `job_cards` template
- [ ] Test: `POST /api/jobs/{id}/approve` with `HX-Request: true` and `HX-Target: job-table-body` still renders `job_rows` template (regression guard — response contains table row markup, not card markup)
- [ ] All 6 tests pass
- [ ] Full test suite `go test ./...` passes with no regressions
- [ ] Any bugs found are logged in `.workflow/BUGS.md`

## Interface Contracts

No cross-repo contracts. Tests are within `internal/web/`.

The test helper pattern for this codebase is in `internal/web/server_test.go` — read it before writing tests to match the existing style (look for how `newTestServer`, mock stores, and CSRF are handled in existing tests).

Key headers to set in test requests:
- `HX-Request: true` — required for HTMX-path code in handlers
- `HX-Target: job-card-deck` — triggers `job_cards` render in `respondJobAction`
- `HX-Target: job-table-body` — triggers `job_rows` render (existing behaviour, regression test)

Expected response content to assert:
- Card deck response contains `"job-card-active"` (class in template) when jobs present
- Card deck response contains `"job-card-empty"` when no jobs
- Table response contains `<tr` or `job-row` (existing table row markup) not `"job-card"`

## Context

Test file location (new file recommended to keep concerns separate):
```
internal/web/job_cards_partial_test.go
```

Existing test files to reference for patterns:
- `internal/web/server_test.go` — main server test setup
- `internal/web/htmx_respond_job_action_test.go` — tests for `respondJobAction` (extend or reference this file for the card-deck regression tests)
- `internal/web/remove_rejected_dashboard_qa_test.go` — example QA test style

The testing command is: `go test ./...` (run from `/workspace`).

Plan reference for test cases: `.workflow/plans/tinder-style-mobile.md` §9 Step 8.

## Notes

<!-- QA agent fills in test results and coverage summary when complete -->
