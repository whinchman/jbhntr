# Task: admin-panel-1-migration-model-config

- **Type**: coder
- **Status**: pending
- **Branch**: feature/admin-panel-1-migration-model-config
- **Source Item**: Admin Panel for jobhuntr (admin-panel.md)
- **Parallel Group**: 1
- **Dependencies**: none

## Description

Lay the foundational groundwork for the admin panel feature by:

1. Adding a new database migration that introduces the `banned_at` column to the `users` table.
2. Adding a `BannedAt` field to the `models.User` struct.
3. Updating the `scanUser` function in the store layer to select and scan `banned_at`.
4. Adding the `AdminConfig` struct and `Admin` field to the application config.
5. Documenting the new config key in `config.yaml.example`.

This task touches only existing foundational files (no new packages). All subsequent tasks depend on these changes.

## Acceptance Criteria

- [ ] `internal/store/migrations/010_add_banned_at_to_users.sql` exists and contains `ALTER TABLE users ADD COLUMN IF NOT EXISTS banned_at TIMESTAMPTZ;`
- [ ] `internal/models/user.go` has a `BannedAt *time.Time` field on the `User` struct
- [ ] `internal/store/user.go` includes `banned_at` in `userSelectCols` and scans it correctly in `scanUser` (using `sql.NullTime` or `sql.NullString` + parse)
- [ ] `internal/config/config.go` has an `AdminConfig` struct with a `Password string` field and an `Admin AdminConfig` field on `Config`
- [ ] `config.yaml.example` documents `admin.password: "${ADMIN_PASSWORD}"` with a comment
- [ ] `go test ./internal/store/...` passes (existing tests unaffected — new column is nullable)
- [ ] `go test ./internal/config/...` passes
- [ ] `go test ./internal/models/...` passes
- [ ] `go build ./...` succeeds

## Interface Contracts

This task is purely foundational. It produces:

- `models.User.BannedAt *time.Time` — nil means not banned; non-nil means banned. All downstream tasks rely on this field name and type.
- `config.Config.Admin.Password string` — used by the admin package and server wiring tasks.
- Migration file `010_add_banned_at_to_users.sql` — must use `ADD COLUMN IF NOT EXISTS` for safe re-application.

## Context

- **Migration file**: create at `internal/store/migrations/010_add_banned_at_to_users.sql`. The existing highest migration is `009_add_email_auth.sql`. Use the same naming pattern.
- **Model file**: `internal/models/user.go` — add `BannedAt *time.Time` to the `User` struct. Import `"time"` if not already present.
- **Store scan**: in `internal/store/user.go`, `userSelectCols` is a string constant/var listing selected columns. Append `, banned_at` at the end. In `scanUser` (or equivalent row scan), add a variable to receive the nullable timestamp (use `sql.NullTime` if available in the Go version, otherwise `*time.Time` directly or `sql.NullString` + `time.Parse`). Set `u.BannedAt` from that variable.
- **Config**: in `internal/config/config.go`, add:
  ```go
  type AdminConfig struct {
      Password string `yaml:"password"`
  }
  ```
  And on the `Config` struct:
  ```go
  Admin AdminConfig `yaml:"admin"`
  ```
- **config.yaml.example**: add the section:
  ```yaml
  admin:
    password: "${ADMIN_PASSWORD}"  # HTTP Basic Auth password for /admin panel; username is "admin"
  ```

## Notes

