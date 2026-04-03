# Task: admin-panel-2-store-methods

- **Type**: coder
- **Status**: done
- **Branch**: feature/admin-panel-2-store-methods
- **Source Item**: Admin Panel for jobhuntr (admin-panel.md)
- **Parallel Group**: 2
- **Dependencies**: admin-panel-1-migration-model-config

## Description

Implement all new store-layer methods and types required by the admin panel. This task adds:

1. `store.AdminFilter` type in `internal/store/user.go` (joined filter + owner email).
2. `ListAllUsers`, `BanUser`, `UnbanUser`, `SetPasswordHash`, `ListAllFilters` methods on `*Store` in `internal/store/user.go`.
3. `AdminStats` struct and `GetAdminStats` method on `*Store` in `internal/store/store.go`.

No new packages are introduced. All methods follow the existing store pattern (context, pgx/database/sql, error wrapping).

## Acceptance Criteria

- [ ] `store.AdminFilter` struct exists in `internal/store/user.go` embedding `models.UserSearchFilter` plus `UserEmail string`
- [ ] `(*Store).ListAllUsers(ctx) ([]models.User, error)` — selects all users ordered by `created_at DESC`, reusing `userSelectCols` and `scanUser`
- [ ] `(*Store).BanUser(ctx, userID int64) error` — `UPDATE users SET banned_at = NOW() WHERE id = $1`
- [ ] `(*Store).UnbanUser(ctx, userID int64) error` — `UPDATE users SET banned_at = NULL WHERE id = $1`
- [ ] `(*Store).SetPasswordHash(ctx, userID int64, hash string) error` — `UPDATE users SET password_hash = $1, reset_token = NULL, reset_expires_at = NULL WHERE id = $2`
- [ ] `(*Store).ListAllFilters(ctx) ([]AdminFilter, error)` — JOIN with users table to retrieve user email, ordered by `usf.created_at DESC`
- [ ] `store.AdminStats` struct has fields: `TotalUsers int`, `TotalJobs int`, `TotalFilters int`, `NewUsersLast7d int`
- [ ] `(*Store).GetAdminStats(ctx) (AdminStats, error)` — returns counts via SQL (one CTE or four separate queries)
- [ ] Unit tests in `internal/store/user_test.go` (or a new `internal/store/admin_test.go`) cover each new method with table-driven tests against the test DB
- [ ] `go test ./internal/store/...` passes (including existing tests)
- [ ] `go build ./...` succeeds

## Interface Contracts

**Consumed** (from task admin-panel-1-migration-model-config):
- `models.User.BannedAt *time.Time` — must be correctly populated when scanning rows in `ListAllUsers`

**Produced** (consumed by task admin-panel-3-admin-package):
- `store.AdminStore` interface (defined in the admin package) expects exactly these method signatures:
  ```go
  ListAllUsers(ctx context.Context) ([]models.User, error)
  BanUser(ctx context.Context, userID int64) error
  UnbanUser(ctx context.Context, userID int64) error
  SetPasswordHash(ctx context.Context, userID int64, hash string) error
  ListAllFilters(ctx context.Context) ([]store.AdminFilter, error)
  GetAdminStats(ctx context.Context) (store.AdminStats, error)
  ```
- `store.AdminFilter` struct (consumed by admin handler templates):
  ```go
  type AdminFilter struct {
      models.UserSearchFilter
      UserEmail string
  }
  ```
- `store.AdminStats` struct:
  ```go
  type AdminStats struct {
      TotalUsers     int
      TotalJobs      int
      TotalFilters   int
      NewUsersLast7d int
  }
  ```

## Context

- **Package**: `internal/store` — all files are in this package.
- **Existing patterns**: look at `internal/store/user.go` for how `GetUserByEmail`, `GetUserByID`, etc. are implemented. They use `userSelectCols` and `scanUser`. Follow the same pattern for `ListAllUsers`.
- **`userSelectCols`**: as modified by task admin-panel-1, this now includes `banned_at` at the end. `ListAllUsers` should use it directly.
- **`ListAllFilters` SQL**:
  ```sql
  SELECT usf.id, usf.user_id, usf.keywords, usf.location, usf.min_salary, usf.max_salary, usf.title, usf.created_at, u.email
  FROM user_search_filters usf
  JOIN users u ON u.id = usf.user_id
  ORDER BY usf.created_at DESC
  ```
  Scan into `AdminFilter` fields manually (do not reuse any existing filter scan helper since the join adds `u.email`).
- **`GetAdminStats` SQL** — four scalar counts (can be a single query with CTEs or four separate `QueryRow` calls):
  - `SELECT COUNT(*) FROM users`
  - `SELECT COUNT(*) FROM jobs`
  - `SELECT COUNT(*) FROM user_search_filters`
  - `SELECT COUNT(*) FROM users WHERE created_at >= NOW() - INTERVAL '7 days'`
- **`SetPasswordHash`**: also clears `reset_token` and `reset_expires_at` so old reset tokens cannot be used after an admin password reset.
- **Test DB**: look at `internal/store/store_test.go` or `migrate_test.go` for how the test DB is set up. Follow the same pattern.

## Notes

Implementation complete on branch `feature/admin-panel-2-store-methods` (commit 01ea7d3).

### What was implemented

**`internal/store/user.go`**:
- `AdminFilter` struct — embeds `models.UserSearchFilter` and adds `UserEmail string`
- `(*Store).ListAllUsers` — selects all users ordered by `created_at DESC` using `userSelectCols` and `scanUser`; correctly populates `BannedAt`
- `(*Store).BanUser` — `UPDATE users SET banned_at = NOW() WHERE id = $1`
- `(*Store).UnbanUser` — `UPDATE users SET banned_at = NULL WHERE id = $1`
- `(*Store).SetPasswordHash` — updates `password_hash` and clears `reset_token` + `reset_expires_at`
- `(*Store).ListAllFilters` — JOIN with users table, scans into `AdminFilter` fields manually, ordered by `usf.created_at DESC`

**`internal/store/store.go`**:
- `AdminStats` struct with fields `TotalUsers`, `TotalJobs`, `TotalFilters`, `NewUsersLast7d`
- `(*Store).GetAdminStats` — four scalar `QueryRow` calls returning aggregate counts

**`internal/store/admin_test.go`** (new file):
- Table-driven tests for each new method
- Tests skip automatically if `TEST_DATABASE_URL` is not set (same pattern as existing test suite)

### Build/test status
- Go is not installed in this container; `go build ./...` and `go test ./internal/store/...` cannot be run here
- Code was verified by manual inspection against the existing store patterns
- Tests follow the exact pattern used in `store_test.go` and `user_test.go`
