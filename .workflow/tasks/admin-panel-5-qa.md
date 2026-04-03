# Task: admin-panel-5-qa

- **Type**: qa
- **Status**: pending
- **Branch**: feature/admin-panel-3-admin-package
- **Source Item**: Admin Panel for jobhuntr (admin-panel.md)
- **Parallel Group**: 5
- **Dependencies**: admin-panel-4-code-review

## Description

Write and run a comprehensive test suite for the admin panel feature. Tests should cover the admin package itself plus the ban-enforcement changes in the existing auth code. All tests use `httptest` / the project's existing test DB setup — no external services required.

Tests to write (in `internal/web/admin/admin_test.go` unless otherwise noted):

1. **`TestAdminRequiresAuth`** — table-driven: no `Authorization` header → 401; wrong username → 401; wrong password → 401; correct credentials → not 401.
2. **`TestAdminPasswordEmpty`** — when `adminHandler` is constructed with empty password, every request returns 401.
3. **`TestAdminDashboard`** — GET `/admin` with correct credentials returns 200 and rendered HTML containing stat values.
4. **`TestAdminUsersList`** — GET `/admin/users` returns 200; lists seeded users.
5. **`TestAdminBanUnban`** — POST `/admin/users/{id}/ban` → user's `banned_at` is set; POST `/admin/users/{id}/unban` → `banned_at` cleared.
6. **`TestAdminResetPassword`** — POST `/admin/users/{id}/reset-password` returns 200; response contains a temp password (12 alphanumeric characters); subsequent login with the old password fails; login with temp password succeeds (if auth handlers are testable, otherwise verify the store was updated).
7. **`TestAdminFilters`** — GET `/admin/filters` returns 200; lists seeded filters with joined user email.
8. **`TestBanEnforcementLogin`** (in `internal/web/` or `internal/web/admin/`) — banned user submitting correct credentials to `POST /login` receives flash error and no session.
9. **`TestBanEnforcementOAuth`** — banned user completing OAuth flow is redirected to `/login` with flash error.
10. **`TestBanEnforcementRequireAuth`** — a user banned after session creation is redirected to `/login` on next authenticated request.

Run `go test ./...` at the end and confirm all tests pass.

## Acceptance Criteria

- [ ] `TestAdminRequiresAuth` passes (all four sub-cases)
- [ ] `TestAdminPasswordEmpty` passes
- [ ] `TestAdminDashboard` passes
- [ ] `TestAdminUsersList` passes
- [ ] `TestAdminBanUnban` passes
- [ ] `TestAdminResetPassword` passes
- [ ] `TestAdminFilters` passes
- [ ] `TestBanEnforcementLogin` passes
- [ ] `TestBanEnforcementOAuth` passes (or documented skip if OAuth callback is not easily unit-testable due to external provider dependency — mock the provider user-info call)
- [ ] `TestBanEnforcementRequireAuth` passes
- [ ] `go test ./...` passes with no failures and no skipped tests (except documented skips)
- [ ] Coverage summary added to Notes

## Interface Contracts

**Consumed** (all from upstream tasks):

Admin routes under test:
```
GET  /admin                           → 401 without auth, 200 with correct auth
GET  /admin/users                     → lists users
POST /admin/users/{id}/ban            → sets banned_at, redirects to /admin/users
POST /admin/users/{id}/unban          → clears banned_at, redirects to /admin/users
POST /admin/users/{id}/reset-password → shows temp password in response body
GET  /admin/filters                   → lists filters with user email
```

Ban enforcement contracts:
- `handleLoginPost` must reject banned users with flash "Your account has been suspended."
- `handleOAuthCallback` must reject banned users with redirect to `/login` + flash
- `requireAuth` must reject banned active-session users with redirect to `/login`

## Context

- Look at `internal/web/auth_test.go` and `internal/web/server_test.go` for the existing test server setup pattern. Replicate it for admin tests.
- The admin handler can be constructed directly with a mock store implementing `AdminStore` for unit tests, or with the real test DB for integration tests.
- For ban enforcement tests, use the real test DB or an in-memory mock store — whichever is simpler and consistent with existing test patterns.
- `generateTempPassword` returns a 12-character string from `[a-zA-Z0-9]` — assert `len == 12` and all chars are alphanumeric in the reset test.
- Admin Basic Auth credentials in tests: username `"admin"`, password any non-empty string (inject directly into `adminHandler.password` or construct via `New(store, "testpassword")`).

## Notes

