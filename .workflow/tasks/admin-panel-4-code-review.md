# Task: admin-panel-4-code-review

- **Type**: code-reviewer
- **Status**: pending
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

