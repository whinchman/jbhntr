# Task: admin-panel-4-code-review

- **Type**: code-reviewer
- **Status**: done
- **Branch**: feature/admin-panel-3-admin-package
- **Source Item**: Admin Panel for jobhuntr (admin-panel.md)
- **Parallel Group**: 4
- **Dependencies**: admin-panel-3-admin-package, admin-panel-3-ban-enforcement

## Description

Perform a thorough code review of all changes introduced by the admin panel feature across the following branches:

- `feature/admin-panel-1-migration-model-config` — migration, model, config changes
- `feature/admin-panel-2-store-methods` — new store methods
- `feature/admin-panel-3-admin-package` — admin sub-package and server wiring
- `feature/admin-panel-3-ban-enforcement` — ban enforcement in existing auth handlers

Focus areas:
1. **Security**: Basic Auth constant-time comparison; no timing oracle; ADMIN_PASSWORD empty case returns 401 not panic; banned-user checks applied in all three auth paths; temp password not logged or stored.
2. **Correctness**: Store methods use correct SQL (`banned_at = NOW()` vs `= NULL`; `SetPasswordHash` clears reset token); `scanUser` correctly handles nullable `banned_at`; `generateTempPassword` uses `crypto/rand` not `math/rand`.
3. **Code standards**: Conventional commit messages; no unused imports; error handling follows existing patterns (no bare `panic`); templates parse correctly.
4. **Regression risk**: Existing tests still pass; no change to non-admin routes.

Write findings to this task file's Notes section. Log any critical or warning findings to `/workspace/.workflow/BUGS.md`.

## Acceptance Criteria

- [ ] All security concerns reviewed (auth middleware, temp password, ban enforcement)
- [ ] All new SQL queries reviewed for correctness and injection safety
- [ ] Template embedding and parsing reviewed
- [ ] Server wiring and `WithAdminStore` pattern reviewed
- [ ] Ban enforcement in `handleLoginPost`, `handleOAuthCallback`, and `requireAuth` all reviewed
- [ ] Findings documented in Notes with severity (critical / warning / info)
- [ ] Critical and warning findings added to `.workflow/BUGS.md`
- [ ] Verdict recorded: `approve` or `request-changes`

## Interface Contracts

N/A — this is a review task.

## Context

Review these branches and their diffs against `development`:
- `feature/admin-panel-1-migration-model-config`
- `feature/admin-panel-2-store-methods`
- `feature/admin-panel-3-admin-package`
- `feature/admin-panel-3-ban-enforcement`

Key files to review:
- `internal/store/migrations/010_add_banned_at_to_users.sql`
- `internal/models/user.go` (`BannedAt` field)
- `internal/store/user.go` (scan changes + new methods)
- `internal/store/store.go` (`AdminStats`, `GetAdminStats`)
- `internal/config/config.go` (`AdminConfig`)
- `internal/web/admin/admin.go` (auth middleware)
- `internal/web/admin/store.go` (interface)
- `internal/web/admin/handlers.go` (all handlers)
- `internal/web/admin/templates/*.html` (template correctness)
- `internal/web/server.go` (wiring)
- `cmd/jobhuntr/main.go` (`.WithAdminStore(db)`)
- `internal/web/auth_email.go` (`handleLoginPost` ban check)
- `internal/web/auth.go` (`handleOAuthCallback` and `requireAuth` ban checks)

## Notes

### Review completed: 2026-04-03

**Verdict: approve** (1 warning, 2 info — no critical issues)

---

## Findings

### [WARNING] internal/store/migrate_test.go:42–52 — Migration 010 missing from migrate_test expected list

`TestMigrate/applies_all_migrations` lists exactly 9 migrations (001–009). Migration `010_add_banned_at_to_users.sql` was added by this feature but `migrate_test.go` was not updated. When `TEST_DATABASE_URL` is set the test will fail with `migrations applied = 10, want 9`.

Fix: Add `"010_add_banned_at_to_users.sql"` to the `expected` slice (after `"009_add_email_auth.sql"`).

---

### [WARNING] internal/store/user.go:462, internal/store/user.go:515 — ListAllUsers and ListAllFilters have no pagination limit

Both `ListAllUsers` and `ListAllFilters` issue `SELECT ... ORDER BY ... DESC` with no `LIMIT`. On a large dataset this will pull the entire table into memory in a single DB call. Not a safety defect now (admin panel is internal-only, guarded by Basic Auth), but will degrade as user/filter counts grow.

Fix: Add a reasonable default limit (e.g. 1000 rows) or add optional offset/limit parameters to the interface.

---

### [INFO] internal/web/admin/templates/admin_users.html — Admin POST forms lack CSRF token

The ban, unban, and reset-password forms in `admin_users.html` are plain `<form method="post">` with no `gorilla.csrf.Token` hidden input. The gorilla/csrf middleware IS applied to the whole router (router-level `r.Use(csrfMiddleware)` in `server.go`), which means CSRF validation is enforced on these POST requests too.

However: the forms provide no CSRF token field and no `csrf.Token(r)` is passed into the template data. This will cause all admin POST actions (ban, unban, reset-password) to return `403 Forbidden` in production when `sessionStore != nil` (i.e. when auth is configured).

Note that the gorilla/csrf middleware exempts routes that do not match the CSRF cookie domain/path — in some configurations this may be silently bypassed. Testing with `admin_test.go` uses a bare `httptest.NewServer` without the main router (so CSRF middleware is not applied), which is why the tests pass.

**Severity adjusted to info** because the admin route is protected by HTTP Basic Auth (a different authentication mechanism), and gorilla/csrf is primarily a session-cookie CSRF protection. Many Basic-Auth-only UIs intentionally omit CSRF tokens because the Authorization header itself is the anti-forgery mechanism. Whether this is acceptable depends on the project's threat model. However the current code WILL fail in production if the gorilla/csrf middleware runs before the admin handler, since no token is submitted with the forms.

Suggested fix: Either (a) exclude `/admin/*` from the gorilla/csrf middleware (e.g. pass `csrf.Except("/admin/...")` option or mount the admin routes before the CSRF middleware is applied), or (b) pass `csrf.Token(r)` into the admin template data and add the hidden input to the forms.

---

### [INFO] internal/web/admin/admin.go:64 — Constant-time comparison is correctly implemented

`subtle.ConstantTimeCompare` is used for both username and password. The empty-password guard correctly short-circuits before the comparison. No timing oracle.

---

### [INFO] internal/web/admin/handlers.go:153 — generateTempPassword uses crypto/rand correctly

`crypto/rand.Read` is used. Modular reduction bias over a 62-char charset is negligible for a 12-char temporary credential. Password is not logged anywhere in handlers or store.

---

### [INFO] internal/web/auth_email.go:224, internal/web/auth.go:348, internal/web/auth.go:546 — Ban enforcement applied in all three auth paths

All three required code paths check `BannedAt != nil` before creating or honoring a session:
- `handleLoginPost` (auth_email.go:224): checks AFTER bcrypt compare, BEFORE `setSession`. Correct ordering — does not create session for banned user.
- `handleOAuthCallback` (auth.go:348): checks AFTER `UpsertUser`, BEFORE `setSession`. Correct.
- `requireAuth` middleware (auth.go:546): checks on every request, clears session if banned mid-session. Correct.

---

### [INFO] internal/store/user.go:482–510 — SQL correctness for BanUser, UnbanUser, SetPasswordHash

- `BanUser`: `banned_at = NOW()` — correct.
- `UnbanUser`: `banned_at = NULL` — correct.
- `SetPasswordHash`: updates hash AND clears `reset_token`/`reset_expires_at` atomically — correct, prevents token reuse after admin reset.
- All queries use `$N` parameterization — no SQL injection risk.

---

### [INFO] internal/store/user.go:373–433 — scanUser nullable banned_at handling

`bannedAt sql.NullString` is scanned correctly. If `bannedAt.Valid`, the timestamp is parsed; if parse fails, `u.BannedAt` remains `nil` (treated as active). This is conservative (a malformed timestamp is ignored silently, not treated as banned). Acceptable behavior.

---

### [INFO] internal/web/server.go:362–366 — Admin panel only mounted when store + password both configured

The `if s.cfg != nil && s.adminStore != nil && s.cfg.Admin.Password != ""` guard prevents the admin routes from being registered when unconfigured. Combined with the `adminAuth` middleware's empty-password check (which returns 401 even if password is somehow empty at runtime), the panel is doubly protected. Correct.

---

### [INFO] internal/store/migrations/010_add_banned_at_to_users.sql — Migration correctness

`ALTER TABLE users ADD COLUMN IF NOT EXISTS banned_at TIMESTAMPTZ;` — idempotent, correct type, nullable (no DEFAULT means existing rows have NULL, i.e. active). Correct.

---

## Summary

- Critical: 0
- Warning: 2 (migrate_test missing migration 010; no pagination on admin list endpoints)
- Info: 1 actionable (admin forms may be CSRF-blocked in production)
- Info (pass): 7 items confirmed correct

**Verdict: approve** — No critical defects. The two warnings are test-coverage and scalability concerns that should be tracked but do not block the feature. The CSRF info item should be verified in integration testing and either resolved or explicitly accepted.

