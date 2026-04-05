# Task: tinder-mobile-qa

- **Type**: qa
- **Status**: done
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

**QA agent — 2026-04-05**

### Go Runtime Availability

Go is not installed in this container. All tests were written and reviewed statically. Tests cannot be executed here; `go test ./...` must be run on a machine with Go installed.

### Existing Tests Reviewed (`internal/web/job_cards_test.go`)

Seven tests written by the backend coder were reviewed for correctness and coverage:

| Test | Covers | Status |
|------|--------|--------|
| `TestHandleJobCardsPartial_Unauthenticated` | AC3: unauthenticated returns 200 empty body | Correct |
| `TestHandleJobCardsPartial_RouteRegistered` | Route registered, not 404 | Correct |
| `TestRespondJobAction_CardDeck_Approve_ReturnsHTML` | AC4: approve+card-deck returns HTML | Correct (content-type only) |
| `TestRespondJobAction_CardDeck_Reject_ReturnsHTML` | AC5: reject+card-deck returns HTML | Correct (content-type only) |
| `TestRespondJobAction_CardDeck_ExcludeStatusesSetInFilter` | ExcludeStatuses=[rejected] set for card-deck | Correct (filter assertion via spy) |
| `TestRespondJobAction_JobTable_NotAffected_Regression` | AC6: table target still renders job_rows | Correct |
| `TestRespondJobAction_CardDeck_RendersJobCardsTemplate` | AC4: approve+card-deck renders job-card markup | Correct |

**Gap identified**: AC1 (authenticated user + jobs → `job-card-active` class) and AC2 (no jobs → `job-card-empty` class) were not covered. These require an internal test to inject the auth context.

### Tests Added

**New file: `internal/web/job_cards_partial_test.go`** (internal package, `package web`)

- `TestHandleJobCardsPartial_Authenticated_WithJobs_ReturnsActiveCard` — AC1: authenticated GET with jobs renders `job-card-active`
- `TestHandleJobCardsPartial_Authenticated_NoJobs_ReturnsEmptyState` — AC2: authenticated GET with no jobs renders `job-card-empty`
- `TestHandleJobCardsPartial_Authenticated_NotEmpty` — regression guard: authenticated path must not return empty body
- `TestHandleJobCardsPartial_Authenticated_ExcludesRejected` — ExcludeStatuses filter is applied end-to-end (mock honours the filter)

Uses `cardPartialJobStore` (internal mock that implements `ExcludeStatuses` filtering, mirroring `uiMinorJobStore` pattern) and auth injection via `userContextKey`.

**Extended: `internal/web/job_cards_test.go`**

- `TestRespondJobAction_CardDeck_Reject_RendersJobCardsTemplate` — AC5: reject+card-deck renders `job_cards` markup (not just content-type)
- `TestHandleJobCardsPartial_ContentType` — explicit Content-Type header assertion for unauthenticated path

### Acceptance Criteria Coverage After QA

- [x] AC1: authenticated + jobs → `job-card-active` class — covered by internal test
- [x] AC2: authenticated + no jobs → `job-card-empty` class — covered by internal test
- [x] AC3: unauthenticated → 200 empty body — covered by existing test
- [x] AC4: approve + card-deck → job_cards template — covered by existing test
- [x] AC5: reject + card-deck → job_cards template — added test
- [x] AC6: job-table-body target → job_rows (regression guard) — covered by existing test
- [ ] All 6 ACs pass — cannot execute; requires Go runtime

### JS Static Review (`swipe-cards.js`)

No critical logic bugs found. Two info-level observations filed in BUGS.md:

1. **INFO**: `commitCard` calls `submitAction(direction)` without passing `card`; `submitAction` falls back to `document.getElementById('job-card-deck')` query. Works correctly; silently fails only if deck element is absent (not a real-world scenario).
2. **INFO**: `window.matchMedia` not guarded in `commitCard`; throws on IE 9 and below (not a supported target; no action needed).

Other reviewed areas: velocity smoothing (correct), overlay opacity updates (correct), `transitionend` multi-property firing (correctly handled — listener self-removes after first fire), vertical swipe bail-out (correctly handled — `_card` null guards all subsequent calls), pointer capture lifecycle (auto-released on `pointerup`/`pointercancel` per spec).

### Bugs Filed

Two INFO-level JS findings appended to `.workflow/BUGS.md`. The WARNING-level double-ListJobs bug was already filed by the Code Reviewer.

**Branch**: `feature/tinder-mobile-backend`
**Commit**: 450189e
