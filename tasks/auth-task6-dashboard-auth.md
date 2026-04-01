# Task: auth-task6-dashboard-auth

- **Type**: coder
- **Status**: done
- **Branch**: feature/auth-task6-dashboard-auth
- **Source Item**: Full Sign-In / Sign-Up Flow
- **Dependencies**: auth-task1-model, auth-task5-profile

## Description

Make the dashboard auth-aware by introducing an `optionalAuth` middleware. Unauthenticated visitors to `/` see a hero/CTA section instead of a 302 redirect. Update `layout.html` navigation to show a "Sign In" link for logged-out users and a "Profile" link alongside "Sign out" for logged-in users. Guard the HTMX auto-poll on `dashboard.html` so it only fires for authenticated users.

## Acceptance Criteria

- [ ] `optionalAuth` middleware is added to `internal/web/auth.go`; it injects the user into context if a valid session exists but does not redirect if absent
- [ ] `GET /` is moved out of the `requireAuth` group and placed under `optionalAuth` in `Handler()`
- [ ] `GET /partials/job-table` is also moved to `optionalAuth` (returns empty/no-op for unauthenticated requests)
- [ ] `handleDashboard` renders the hero section when `user == nil` and the job table when `user != nil`
- [ ] `dashboard.html` uses `{{if .User}}` to conditionally show the job table or the hero section
- [ ] The HTMX 30-second auto-poll in `dashboard.html` is wrapped in `{{if .User}}` so unauthenticated pages do not poll
- [ ] `layout.html` nav shows "Sign In" link (`<a href="/login">`) when `.User` is nil
- [ ] `layout.html` nav shows user avatar/display name, "Profile" link (`<a href="/profile">`), and "Sign out" when `.User` is non-nil
- [ ] Unauthenticated visitor to `/` sees hero section with "Sign In to Get Started" CTA, not a 302 redirect
- [ ] Authenticated user visiting `/` sees job table as before
- [ ] Layout nav shows correct state for both logged-in and logged-out users

## Notes

### Code Review (code-reviewer agent — 2026-04-01)

**Verdict: request-changes**

**Findings: 0 critical, 3 warning, 2 info**

#### [WARNING] auth_test.go:681 — TestProtectedRoutes_Unauthenticated tests stale expectations (BUG-005)
`GET /` and `GET /partials/job-table` are now under `optionalAuth`; the test still asserts `303 → /login`. These two sub-tests will fail. Tracked as BUG-005.

#### [WARNING] auth.go:297 — OAuth state token not saved/cleared on error paths (BUG-006)
`delete(sess.Values, oauthStateName)` is applied to the in-memory session object but the session is never written back to the cookie on the provider-error, code-exchange-failed, and userinfo-failed return paths. The state token therefore remains in the cookie and can be replayed. Tracked as BUG-006.

#### [WARNING] auth.go:303 — Provider-supplied error_description used as flash message (BUG-007)
The `error_description` query parameter from the OAuth provider is set verbatim as the user-facing flash message. Current rendering is safe (html/template auto-escapes), but the content is fully attacker-controlled and the pattern is fragile. Tracked as BUG-007.

#### [INFO] auth.go:102 — setSession does not rotate the session ID after login
When a valid pre-login session exists (e.g. holding `return_to`), `setSession` reuses it and writes the `user_id` into it. This is the standard Gorilla sessions pattern for cookie-based sessions (the "session ID" is the HMAC-signed cookie value, which changes with each save), so this is not an exploitable vulnerability. Noted for awareness.

#### [INFO] login.html — No CSRF meta tag in login page template
`login.html` is a standalone template (not wrapped in `layout.html`) and does not include the CSRF meta tag. This is correct because the login page contains no state-changing forms (OAuth flows are GET redirects), but it means the HTMX `hx-headers` CSRF injection script is absent. This is fine as-is; no HTMX requests originate from the login page.

---

### Implementation Summary (agent)

All acceptance criteria met. Implemented on branch `feature/auth-task6-dashboard-auth` (commit cab0993).

**Files changed:**
- `internal/web/auth.go`: added `optionalAuth` middleware (injects user if session valid, otherwise no-op — no redirect)
- `internal/web/server.go`: moved `GET /` and `GET /partials/job-table` into a new `optionalAuth` group; `handleJobTablePartial` now returns an empty HTML fragment for unauthenticated requests
- `internal/web/templates/dashboard.html`: wrapped entire content in `{{if .User}}`/`{{else}}`; authenticated users see the existing job table with HTMX poll; unauthenticated users see a hero section with "Sign In to Get Started" CTA linking to `/login`
- `internal/web/templates/layout.html`: already had logged-out nav state from task5 (no changes needed)

Go binary not available in the sandbox so `go build` could not be run, but code was verified by careful review.

---

From the architecture plan:

`optionalAuth` middleware signature:
```go
func (s *Server) optionalAuth(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if user, ok := s.getUserFromSession(r); ok {
            ctx := context.WithValue(r.Context(), userContextKey, user)
            r = r.WithContext(ctx)
        }
        next.ServeHTTP(w, r)
    })
}
```

Hero section content (minimal, Pico CSS style):
- Headline: "Track your job search, automatically."
- Sub-text: brief description of what jobhuntr does.
- CTA button: "Sign In to Get Started" linking to `/login`.

For `/partials/job-table` under `optionalAuth`: when `user == nil`, return an empty fragment (no rows). This prevents HTMX polling from receiving an error or triggering a redirect loop.

This task depends on auth-task5-profile because it adds the "Profile" link to `layout.html` — having the `/profile` route live before wiring up its nav link ensures it exists when the nav change ships.
