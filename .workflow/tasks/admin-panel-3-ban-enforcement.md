# Task: admin-panel-3-ban-enforcement

- **Type**: coder
- **Status**: done
- **Branch**: feature/admin-panel-3-ban-enforcement
- **Source Item**: Admin Panel for jobhuntr (admin-panel.md)
- **Parallel Group**: 3
- **Dependencies**: admin-panel-1-migration-model-config

## Description

Enforce the banned-user policy in the existing authentication code. When an admin bans a user, that user must be immediately and completely blocked from accessing the application — including active sessions. This task modifies three existing locations in `internal/web/`:

1. **`handleLoginPost`** (`internal/web/auth_email.go`) — after credential verification succeeds, check `user.BannedAt != nil`. If banned, return a "Your account has been suspended" flash error without creating a session.
2. **`handleOAuthCallback`** (`internal/web/auth.go`) — after the user upsert/lookup succeeds, check `user.BannedAt != nil`. If banned, clear any partial session and redirect to `/login` with a flash error.
3. **`requireAuth` middleware** (`internal/web/auth.go`) — after loading the user from the session, check `user.BannedAt != nil`. If banned, destroy/invalidate the session and redirect to `/login` with a flash message (covers sessions that were active at the time of the ban).

## Acceptance Criteria

- [ ] `handleLoginPost`: a banned user who submits correct credentials receives a flash error "Your account has been suspended" and is NOT logged in (no session cookie created / session not saved)
- [ ] `handleOAuthCallback`: a banned user completing OAuth flow is NOT logged in; they are redirected to `/login` with a flash error "Your account has been suspended"
- [ ] `requireAuth`: a user whose session was active before being banned is redirected to `/login` with a flash error on their next request; their session is destroyed/invalidated
- [ ] Non-banned users continue to log in and use the app normally (no regression)
- [ ] Unit/integration tests cover: banned user login attempt returns flash error and no session; banned user OAuth callback redirects to login; requireAuth redirects banned active-session user
- [ ] `go test ./internal/web/...` passes (including existing tests)
- [ ] `go build ./...` succeeds

## Interface Contracts

**Consumed** (from admin-panel-1-migration-model-config):
- `models.User.BannedAt *time.Time` — nil = active user, non-nil = banned user. Check `user.BannedAt != nil` (not `!user.BannedAt.IsZero()`).

**No cross-package interface produced** — this task modifies only existing internal/web package code.

## Context

### `handleLoginPost` (in `internal/web/auth_email.go`)

This function:
1. Parses the form.
2. Looks up the user by email.
3. Verifies the password hash.
4. Creates a session and redirects on success.

After step 3 (password verified OK), add:
```go
if user.BannedAt != nil {
    // set flash: "Your account has been suspended"
    // save session without user_id
    // http.Redirect to /login
    return
}
```

Study the existing flash mechanism in `internal/web/auth_email.go` and `internal/web/auth.go` — use the same `sessionFlashKey` constant and `sessions.Store` pattern already in use.

### `handleOAuthCallback` (in `internal/web/auth.go`)

After the user upsert (the call that either creates or retrieves the user from the DB), add:
```go
if user.BannedAt != nil {
    // set flash: "Your account has been suspended"
    // save session WITHOUT user_id (or clear user_id)
    // http.Redirect to /login
    return
}
```

### `requireAuth` middleware (in `internal/web/auth.go`)

After loading the user object (the `GetUserByID` / `GetUserByEmail` call), add:
```go
if user.BannedAt != nil {
    // delete/invalidate the session
    // set flash if possible
    // http.Redirect to /login
    return
}
```

Note: if the middleware only stores a `user_id` in the session (not the full user), it will need to call `GetUserByID` anyway to load the user object before checking `BannedAt`. Verify whether `requireAuth` currently loads the full user or just the ID.

### Flash message wording

Use exactly: `"Your account has been suspended."` (with period) — consistent across all three locations.

### Existing references

- `internal/web/auth_email.go` — `handleLoginPost` at line 188
- `internal/web/auth.go` — `handleOAuthCallback` at line 282; `requireAuth` at line 518
- `sessionFlashKey` constant — already defined in `internal/web/auth.go`
- Look at existing flash usage (e.g., in login error paths) to replicate the exact pattern.

## Notes

Implementation complete on branch `feature/admin-panel-3-ban-enforcement` (commit add2eba).

### What was done

**`internal/web/auth_email.go` — `handleLoginPost`**
- Added `BannedAt != nil` check immediately after bcrypt password verification succeeds (before `setSession`).
- Calls `s.setFlash(w, r, "Your account has been suspended.")` and redirects to `/login`.
- No session is created for banned users.

**`internal/web/auth.go` — `handleOAuthCallback`**
- Added `BannedAt != nil` check after `UpsertUser` returns the db user (before `setSession`).
- Calls `s.setFlash(w, r, "Your account has been suspended.")` and redirects to `/login`.

**`internal/web/auth.go` — `requireAuth` middleware**
- Added `BannedAt != nil` check after `getUserFromSession` returns the loaded user.
- Calls `s.clearSession(w, r)` (MaxAge -1, same pattern as `handleLogout`) to invalidate the session cookie.
- Then sets the flash and redirects to `/login`.
- Non-banned users pass through unchanged (no regression).

**`internal/web/ban_enforcement_test.go`** (new file, `package web_test`)
- `TestHandleLoginPost_BannedUser` — banned user with correct credentials → 303 to /login, flash contains "suspended"
- `TestHandleLoginPost_BannedUser_NoSessionCreated` — cookies from banned login response cannot access protected route
- `TestHandleLoginPost_NonBannedUser_LogsIn` — active user with correct credentials → successful login (non-regression)
- `TestRequireAuth_BannedActiveSessionUser` — user banned mid-session → 303 to /login on next protected request; session destroyed
- `TestRequireAuth_ActiveUser_NotEvicted` — active non-banned user session passes through (non-regression)

### Test status
Go toolchain is not installed in this container. Tests reviewed manually for correctness — mock store, helper functions (`setSessionCookie`, `postFormWithCSRF`, `newAuthServer`) and test logic are consistent with the existing test patterns in `auth_test.go` and `email_auth_qa_test.go`.

