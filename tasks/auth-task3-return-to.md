# Task: auth-task3-return-to

- **Type**: coder
- **Status**: pending
- **Branch**: feature/auth-task3-return-to
- **Source Item**: Full Sign-In / Sign-Up Flow
- **Dependencies**: auth-task1-model, auth-task2-login-polish

## Description

Implement "return to" URL preservation so that users landing on a protected page are redirected back to their original destination after login. The URL is stored in the gorilla session before the `/login` redirect and consumed (with same-origin validation) after successful OAuth callback.

## Acceptance Criteria

- [ ] `sessionReturnTo = "return_to"` constant is defined in `internal/web/auth.go`
- [ ] `requireAuth` middleware saves `r.URL.RequestURI()` to the session as `return_to` before redirecting to `/login` (only for GET requests, only when the path is not `/login` or `/logout`)
- [ ] `consumeReturnTo(r) string` helper reads and deletes `return_to` from session; validates value starts with `/` and contains no `//` or URL scheme; returns `/` if invalid or empty
- [ ] `handleOAuthCallback` (success path) calls `consumeReturnTo` to determine the redirect target after `setSession`
- [ ] `handleLogin` (already-logged-in path) uses `consumeReturnTo` instead of a hardcoded `/`
- [ ] Unauthenticated GET to `/settings` → login → auth → lands on `/settings`
- [ ] Unauthenticated GET to `/jobs/123` → login → auth → lands on `/jobs/123`
- [ ] `return_to` containing an absolute URL (e.g. `https://evil.com`) falls back to `/`
- [ ] `return_to` starting with `//` falls back to `/`
- [ ] Already-authenticated user hitting `/login` is redirected to `return_to` or `/`

## Notes

From the architecture plan:

The same-origin guard must reject any value that contains a scheme (`://`) or starts with `//`. A simple validation:
- Value must start with `/`
- Value must not contain `://`
- Value must not start with `//`

This task depends on auth-task2-login-polish because the callback handler refactoring in Task 2 (flash + redirect on error) provides the correct structure for inserting the `consumeReturnTo` call in the success path. It depends on auth-task1-model because Task 4 (onboarding gate) builds on top of the return_to logic in the callback — coordinating with auth-task1-model ensures the `OnboardingComplete` check and return_to are added together cleanly.
