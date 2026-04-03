# Task: email-auth-1-migration-models

- **Type**: coder
- **Status**: pending
- **Repo**: .
- **Parallel Group**: 1
- **Branch**: feature/email-auth-1-migration-models
- **Source Item**: email-auth (email/password authentication)
- **Dependencies**: none

## Description

Add the database migration and model/store changes required for email+password authentication. This is foundational — all other email-auth tasks depend on this completing first.

Three files to touch: `internal/store/migrations/009_add_email_auth.sql`, `internal/models/user.go`, `internal/store/user.go`. Also update the `UserStore` interface in `internal/web/server.go` with new method signatures.

## Acceptance Criteria

- [ ] `internal/store/migrations/009_add_email_auth.sql` exists and is correct
- [ ] `models.User` has all 6 new pointer fields
- [ ] `scanUser` scans all new nullable columns without panicking
- [ ] `CreateUserWithPassword` implemented and tested
- [ ] `GetUserByEmail` implemented and tested
- [ ] `SetResetToken` / `ConsumeResetToken` implemented and tested
- [ ] `SetEmailVerifyToken` / `ConsumeVerifyToken` implemented and tested
- [ ] `UserStore` interface in `server.go` extended with all new methods
- [ ] `go test ./internal/store/...` passes

## Interface Contracts

No cross-repo contracts — this is a single-repo project. However the `UserStore` interface in `internal/web/server.go` must be updated so the server wiring task (Group 2) can compile. The exact method signatures to add are:

```go
// Add to UserStore interface in internal/web/server.go:
CreateUserWithPassword(ctx context.Context, email, displayName, passwordHash, verifyToken string, verifyExpiresAt time.Time) (*models.User, error)
GetUserByEmail(ctx context.Context, email string) (*models.User, error)
SetResetToken(ctx context.Context, userID int64, token string, expiresAt time.Time) error
ConsumeResetToken(ctx context.Context, token string) (*models.User, error)
SetEmailVerifyToken(ctx context.Context, userID int64, token string, expiresAt time.Time) error
ConsumeVerifyToken(ctx context.Context, token string) (*models.User, error)
```

## Context

### Migration file

Create `internal/store/migrations/009_add_email_auth.sql`:

```sql
-- Password hash (bcrypt). NULL means the account was created via OAuth only.
ALTER TABLE users ADD COLUMN IF NOT EXISTS password_hash TEXT;

-- Email verification. Default true for existing OAuth users (their email was
-- verified by the provider).
ALTER TABLE users ADD COLUMN IF NOT EXISTS email_verified INTEGER NOT NULL DEFAULT 1;

-- One-time email verification token and its expiry.
ALTER TABLE users ADD COLUMN IF NOT EXISTS email_verify_token TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS email_verify_expires_at TIMESTAMPTZ;

-- Password-reset token and its expiry.
ALTER TABLE users ADD COLUMN IF NOT EXISTS reset_token TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS reset_expires_at TIMESTAMPTZ;

-- Unique index on email for GetUserByEmail lookups.
-- Allow multiple rows with empty email (legacy OAuth rows without email).
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email_unique
    ON users (email)
    WHERE email != '';
```

### Model changes

Add to `models.User` struct in `internal/models/user.go`:

```go
PasswordHash         *string    // nil = OAuth-only account
EmailVerified        bool
EmailVerifyToken     *string
EmailVerifyExpiresAt *time.Time
ResetToken           *string
ResetExpiresAt       *time.Time
```

Use `*string` (pointer) so NULL vs empty string is distinguishable on DB scan. Use `sql.NullString` and `sql.NullTime` in `scanUser`.

New `email_verified` default is `1` (true) for legacy OAuth rows. New email/password registrations must pass `email_verified = 0` explicitly.

### Store method implementations (`internal/store/user.go`)

**`CreateUserWithPassword`**: INSERT INTO users with email, display_name, password_hash, email_verified=0, email_verify_token, email_verify_expires_at. Return the created `*models.User`. Handle unique-constraint violation on email (return a typed sentinel error or a wrapped `ErrEmailTaken` so the handler can flash "An account with that email already exists.").

**`GetUserByEmail`**: `SELECT ... FROM users WHERE email = $1`. Use the same scan path as `GetUser`. Return `nil, nil` (not an error) when not found, so the handler can time-equalize before responding.

**`SetResetToken`**: `UPDATE users SET reset_token=$2, reset_expires_at=$3 WHERE id=$1`.

**`ConsumeResetToken`**: In a single UPDATE ... RETURNING:
```sql
UPDATE users
SET password_hash = $2,
    reset_token = NULL,
    reset_expires_at = NULL
WHERE reset_token = $1
  AND reset_expires_at > NOW()
RETURNING ...
```
Caller passes the new bcrypt hash. Returns the updated user, or `nil, nil` if token not found/expired.

**`SetEmailVerifyToken`**: `UPDATE users SET email_verify_token=$2, email_verify_expires_at=$3 WHERE id=$1`.

**`ConsumeVerifyToken`**:
```sql
UPDATE users
SET email_verified = 1,
    email_verify_token = NULL,
    email_verify_expires_at = NULL
WHERE email_verify_token = $1
  AND email_verify_expires_at > NOW()
RETURNING ...
```
Returns the updated user, or `nil, nil` if token not found/expired.

### Existing patterns to follow

- Look at `internal/store/user.go` — existing `scanUser` function and `UpsertUser` for style.
- Look at `internal/store/user_test.go` — existing table-driven tests for style.
- The store uses `pgx` or `database/sql` — check the existing import to match.
- Migration runner is in `internal/store/migrate.go` — it runs all `.sql` files in `migrations/` in filename order. No changes needed there; just adding the file is sufficient.

### Dependency to add

Run `go get golang.org/x/crypto` to add bcrypt. This package will be used by the auth handlers (Group 3), but adding it here ensures `go.mod` is consistent for all subsequent tasks.

## Notes

<!-- implementing agent fills this in -->
