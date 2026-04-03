# Task: email-auth-4-qa

- **Type**: qa
- **Status**: pending
- **Repo**: .
- **Parallel Group**: 4
- **Branch**: feature/email-auth-4-qa
- **Source Item**: email-auth (email/password authentication)
- **Dependencies**: email-auth-3-handlers, email-auth-3-templates

## Description

Write and run comprehensive tests for the email/password authentication feature. The handlers task (`email-auth-3-handlers`) writes basic unit tests — this QA task adds integration-level and edge-case coverage, runs the full test suite, and verifies all acceptance criteria from the architecture plan.

## Acceptance Criteria

- [ ] `go test ./...` passes with no failures
- [ ] All 8 auth handler functions have test coverage for at least happy path + primary error paths
- [ ] Rate limiter behavior tested (6th request in a minute returns 429)
- [ ] Token expiry tested: expired verify token and expired reset token both return correct error responses
- [ ] Duplicate email registration returns the correct flash message
- [ ] Login timing equalization tested (response time on "not found" is >= 200ms)
- [ ] CSRF middleware: POST requests without CSRF token return 403 (if testable without full integration setup)
- [ ] `ConsumeVerifyToken` and `ConsumeResetToken` store tests verify token is cleared after consumption (not reusable)
- [ ] Migration 009 idempotency tested: running the migration twice does not error (`IF NOT EXISTS` / `IF NOT EXISTS`)
- [ ] Test coverage summary recorded in Notes

## Interface Contracts

### Test helpers available

- `internal/store/store_test.go` — existing store test setup (check for test DB helpers)
- `internal/web/server_test.go` — existing handler test setup (check for `httptest.NewServer` patterns)
- The `NoopMailer` from `internal/mailer` is the mock mailer for all tests

### Store methods under test

All 6 new store methods from `email-auth-1-migration-models`:
- `CreateUserWithPassword` — happy path, duplicate email conflict
- `GetUserByEmail` — found, not found (returns nil/nil)
- `SetResetToken` / `ConsumeResetToken` — happy path, expired token, already consumed (token cleared)
- `SetEmailVerifyToken` / `ConsumeVerifyToken` — same as above
- `GetUserByResetToken` (if added by handlers task) — found, not found, expired

## Context

### Test scenarios by handler

#### Registration (`POST /register`)
- Empty display name → flash error, no user created
- Invalid email format → flash error
- Password < 8 chars → flash error
- Passwords don't match → flash error
- Duplicate email → flash "An account with that email already exists."
- Valid input → user created, session set, redirect to `/onboarding`
- Valid input → verify email sent (mock mailer called with correct `to` address)

#### Login (`POST /login`)
- Unknown email → 200ms+ response time, flash "Invalid email or password."
- Wrong password → flash "Invalid email or password."
- Correct credentials → session cookie set, redirect to `/onboarding` (if not onboarded) or `/` (if onboarded)
- Rate limit: 6 requests in < 60s → HTTP 429

#### Forgot password (`POST /forgot-password`)
- Unknown email → same success flash (no enumeration)
- Known email (with password) → reset token stored, email sent to correct address
- Known email (OAuth-only, no password hash) → no email sent, same success flash
- Rate limit: 6 requests → HTTP 429

#### Reset password (`GET /reset-password`)
- Valid token → form rendered with `.TokenValid = true`
- Expired token → form not rendered, "link expired" message
- Missing token → "link expired" message

#### Reset password (`POST /reset-password`)
- Expired/consumed token → flash, redirect to `/forgot-password`
- Passwords don't match → flash error
- Password < 8 chars → flash error
- Valid → password updated, session set, redirect to `/`

#### Email verification (`GET /verify-email`)
- Valid token → `email_verified = 1`, flash success, redirect `/`
- Expired token → flash error, redirect `/login`
- Already consumed token → flash error, redirect `/login`

### Store-level tests

For `ConsumeResetToken`: after a successful consume, call it again with the same token → returns `nil, nil` (token was cleared).

For `ConsumeVerifyToken`: same pattern — second call returns `nil, nil`.

For `CreateUserWithPassword` duplicate: second insert with same email → returns an error (wrapped `ErrEmailTaken` or similar).

### Running the test suite

```
go test ./...
```

If test DB is required (for store tests), check `internal/store/store_test.go` for the test DB setup pattern. The existing tests use a test Postgres instance configured via environment variables — follow the same pattern.

### Coverage report

Run `go test -coverprofile=coverage.out ./... && go tool cover -func=coverage.out` and record the summary in Notes.

## Notes

<!-- QA agent fills in test results, coverage summary, any bugs found -->
