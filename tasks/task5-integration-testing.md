# Task: task5-integration-testing

**Type:** designer
**Status:** done (verified)
**Priority:** 3
**Epic:** oauth-multi-user
**Depends On:** task1-schema-migration, task2-auth-oauth, task3-peruser-routes, task4-multiuser-scraper

## Description

Final integration testing and cleanup for the OAuth multi-user epic. Write end-to-end tests that verify the complete flow across all components, remove any remaining single-user assumptions, and update config examples.

### What to build:

1. **Integration tests** (`internal/web/server_test.go` or new integration test file):
   - End-to-end OAuth login flow (mocked provider)
   - Test: unauthenticated requests redirect to `/login`
   - Test: user A cannot see user B's jobs (full HTTP flow)
   - Test: settings persist per-user (full HTTP flow)
   - Test: scraper creates jobs for correct user (store-level integration)

2. **Cleanup**:
   - Remove any remaining single-user assumptions across the codebase
   - Search for any lingering `userID=0` that should be a real user ID
   - Search for any remaining TODO comments related to multi-user
   - Ensure all branches are properly chained (task1→task2→task3→task4→task5)

3. **Config example**:
   - Update `config.yaml.example` with the new `auth` section
   - Document required environment variables for OAuth

## Acceptance Criteria

- [ ] Integration test: OAuth login flow works end-to-end (mocked provider)
- [ ] Integration test: unauthenticated requests redirect to `/login`
- [ ] Integration test: user isolation (user A cannot see user B's jobs)
- [ ] Integration test: per-user settings persistence
- [ ] Integration test: scraper creates jobs for correct user
- [ ] No remaining single-user assumptions in the codebase
- [ ] No remaining TODO comments for multi-user tasks (task1-4)
- [ ] `config.yaml.example` updated with auth section
- [ ] `go build ./...` succeeds
- [ ] `go test ./...` passes

## Context

- All prior tasks (1-4) are implemented on chained feature branches
- Branch chain: main → feature/task1-schema-migration → feature/task2-auth-oauth → feature/task3-peruser-routes → feature/task4-multiuser-scraper
- See full plan: `plans/oauth-multi-user.md` (task 5 section)

## Design

Full design document: `plans/task5-integration-testing-design.md`

### Summary

**New integration tests (4 tests across 2 new files):**

1. `internal/web/integration_test.go`:
   - `TestIntegration_OAuthLoginFlow` — complete OAuth code exchange with a mock provider server, verifying redirect chain through `/auth/google` to callback to dashboard, and user creation in DB.
   - `TestIntegration_UserIsolation_Jobs` — two users with session cookies, verifying user A's `/api/jobs` returns only A's jobs and 404 for B's job IDs (and vice versa).
   - `TestIntegration_PerUserSettings` — user A saves resume and filter via POST to `/settings/*`, user B's settings page and DB have no trace of A's data.

2. `internal/scraper/integration_test.go`:
   - `TestIntegration_SchedulerCreatesJobsForCorrectUser` — `Scheduler` wired to real in-memory SQLite store with two users and distinct filters, verifying jobs land in correct user scopes.

**Mock OAuth approach:** A local `httptest.Server` serves `/token` (returns fake access token) and `/userinfo` or `/user` + `/user/emails` (returns configurable profile). A new `WithTestOAuthProvider` method on `Server` lets tests override the `oauth2.Config` endpoint URLs after server construction.

**Cleanup audit results:**
- Zero TODO/FIXME comments remain in non-test code.
- All `userID=0` usages in `generator/worker.go` are intentional (background worker processes all users).
- `Config.SearchFilters` is unused at runtime (scheduler reads DB); add deprecation comment.
- `Config.Resume.Path` is still used by worker as fallback; add clarification comment. Future enhancement: worker should read per-user `resume_markdown`.

**Config updates:**
- `config.yaml.example`: add `auth` section with `session_secret` and `providers` (Google + GitHub), deprecation note on `search_filters`.
- `.env.example`: add `SESSION_SECRET`, `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, `GITHUB_CLIENT_ID`, `GITHUB_CLIENT_SECRET`.

**Production code changes (minimal):**
- `internal/web/server.go`: add `WithTestOAuthProvider(name string, cfg *oauth2.Config)` method (~5 lines).
- `internal/config/config.go`: add deprecation/clarification comments (~6 lines).

## Notes

Design document completed. Thorough audit of all existing tests confirmed substantial coverage already exists. Identified 4 specific integration test gaps and specified exact setup/assert steps for each. One minor production code addition (`WithTestOAuthProvider`) enables the OAuth integration test. No remaining single-user assumptions or TODO comments found. Worker per-user resume lookup logged as future backlog item.

### Implementation Notes

All 4 integration tests implemented and passing:
- `TestIntegration_OAuthLoginFlow` — full OAuth redirect chain with mock provider, user creation verified in DB
- `TestIntegration_UserIsolation_Jobs` — table-driven test with two users, cross-user access returns 404
- `TestIntegration_PerUserSettings` — resume and filter persistence per user, verified via HTTP and DB
- `TestIntegration_SchedulerCreatesJobsForCorrectUser` — scheduler with real SQLite store, jobs land in correct user scopes

Production code changes:
- `internal/web/server.go`: added `WithTestOAuthProvider` method
- `internal/config/config.go`: added deprecation comments on `SearchFilters` and `Resume`
- `config.yaml.example`: updated with `auth` section and deprecation note
- `.env.example`: added OAuth environment variables

Test infrastructure: `mockProviderTransport` intercepts Google/GitHub API URLs and redirects to local mock server for end-to-end OAuth testing without external dependencies.

`go build ./...` and `go test ./...` both pass.

## Review

**Reviewer:** Code Reviewer agent
**Verdict:** PASS (one minor fix applied)

### Summary

All 4 integration tests are well-designed, match the design doc, and exercise real stores (in-memory SQLite) rather than mocks. Build and all tests pass.

### Fix Applied

- `internal/config/config.go`: Fixed field alignment on `Resume` line (was missing padding after deprecation comment was added, breaking column alignment with other struct fields).

### Detailed Findings

**Test correctness** -- All four tests test what they claim. Assertions are meaningful and specific:
- `TestIntegration_OAuthLoginFlow`: Full redirect chain (root -> login -> provider -> callback -> dashboard) with DB verification of user creation. Uses cookie jar for stateful session tracking.
- `TestIntegration_UserIsolation_Jobs`: Table-driven with 4 cases covering both list and detail endpoints for cross-user denial. Uses `setIntegrationSessionCookie` to forge valid session cookies.
- `TestIntegration_PerUserSettings`: Exercises CSRF flow end-to-end (GET settings -> extract token -> POST with token). Verifies isolation via both HTTP response body and direct DB queries.
- `TestIntegration_SchedulerCreatesJobsForCorrectUser`: Wired to real `store.Store`, verifies per-user scoping and unscoped (userID=0) query returns all jobs.

**OAuth mock** -- Realistic: serves token exchange, Google userinfo, and GitHub user + emails endpoints. `mockProviderTransport` intercepts real Google/GitHub API URLs and redirects to local mock. Correctly checks `/user/emails` before `/user` to avoid prefix-match false positive.

**`WithTestOAuthProvider`** -- Acceptable. Exported method on `Server` with clear test-only documentation. Not gated by build tags, but requires direct `*Server` access which only bootstrap code has. The method correctly modifies `oauthProviders` map that handlers read at request time (not at handler-construction time).

**Global transport mutation** -- `newIntegrationServer` replaces `http.DefaultTransport`. This is a potential test pollution vector if tests run in parallel. However, no `t.Parallel()` is used and cleanup is properly registered. Acceptable trade-off vs. the alternative of injecting an HTTP client through production code.

**Config examples** -- Complete and correct. All YAML keys match struct tags. Env var names in `.env.example` match `${...}` placeholders in `config.yaml.example`. Deprecation comment on `search_filters` is clear.

**No bugs found** -- No out-of-scope bugs to file.

## QA

**Reviewer:** QA agent
**Verdict:** PASS -- all acceptance criteria verified

### Verification Results

| Criterion | Status | Notes |
|-----------|--------|-------|
| Integration test: OAuth login flow (mocked provider) | PASS | `TestIntegration_OAuthLoginFlow` -- 5 subtests, full redirect chain |
| Integration test: unauthenticated requests redirect to `/login` | PASS | Covered by subtest `unauthenticated_root_redirects_to_login` |
| Integration test: user isolation (user A cannot see user B's jobs) | PASS | `TestIntegration_UserIsolation_Jobs` -- 4 table-driven cases |
| Integration test: per-user settings persistence | PASS | `TestIntegration_PerUserSettings` -- 4 subtests with CSRF flow |
| Integration test: scraper creates jobs for correct user | PASS | `TestIntegration_SchedulerCreatesJobsForCorrectUser` -- 6 subtests |
| No remaining single-user assumptions | PASS | Full grep audit; all `userID=0` in worker.go are intentional |
| No remaining TODO comments for task1-4 | PASS | Zero TODO/FIXME in non-test Go files |
| `config.yaml.example` updated with auth section | PASS | Includes auth, session_secret, providers, deprecation note |
| `.env.example` updated with OAuth vars | PASS | SESSION_SECRET, GOOGLE_*, GITHUB_* all present |
| `go build ./...` succeeds | PASS | Clean build, no warnings |
| `go test ./...` passes | PASS | All packages pass (config, generator, models, notifier, scraper, store, web) |
| `go vet ./...` passes | PASS | No static analysis issues |

### Branch Chain Integrity

Full chain is clean and linear (15 commits from main):
```
main -> task1 (schema) -> task2 (auth) -> task3 (routes) -> task4 (scraper) -> task5 (integration)
```

### Codebase Audit Summary

- **TODO/FIXME scan:** zero in non-test production code
- **userID=0 scan:** 5 occurrences in `generator/worker.go`, all intentional background-worker patterns with documenting comments
- **Single-user file-config reads in web handlers:** none found
- **Config.SearchFilters runtime usage outside config.go:** none (correctly deprecated)
- **`go vet`:** passes clean

### Edge Case Analysis

1. **Both Google and GitHub configured, one fails:** Handled correctly. Each provider is independent in the `oauthProviders` map. If one provider's token exchange fails, the callback logs the error and redirects to `/login`. Other providers remain functional.
2. **session_secret not set:** Server starts without auth -- `sessionStore` is nil, `requireAuth` middleware is not applied, routes are unprotected. This is the correct no-auth development mode.
3. **Same email, different providers:** Creates two separate user accounts (unique key is `(provider, provider_id)`, not email). This is a known design choice, not a bug. Account linking across providers is a future enhancement.

### No New Bugs Found

Existing bugs BUG-001 through BUG-004 remain valid and already filed. No new issues discovered during QA verification of this task.
