# Task: auth-task6-dashboard-auth

- **Type**: coder
- **Status**: pending
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
