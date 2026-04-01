# Task: auth-task2-login-polish

- **Type**: coder
- **Status**: pending
- **Branch**: feature/auth-task2-login-polish
- **Source Item**: Full Sign-In / Sign-Up Flow
- **Dependencies**: none

## Description

Add session-based flash messages to the auth layer and polish the login page. Errors that currently cause silent redirects (OAuth denial, code-exchange failure, userinfo failure) should set a flash message and redirect to `/login` with the message displayed. Add a loading/busy state to provider buttons on the login page.

## Acceptance Criteria

- [ ] `setFlash(w, r, msg string)` and `consumeFlash(r) string` helpers are added to `internal/web/auth.go` using the gorilla session `"flash"` key
- [ ] `handleOAuthCallback` calls `setFlash` and redirects to `/login` on: provider error, code-exchange failure, userinfo fetch failure
- [ ] `loginData` struct has a `Flash string` field
- [ ] `handleLogin` calls `consumeFlash(r)` and passes the result into `loginData`
- [ ] `login.html` renders a visible alert block when `{{.Flash}}` is non-empty
- [ ] Clicking a provider button shows an `aria-busy="true"` loading state (PicoCSS native)
- [ ] Denying OAuth consent redirects back to `/login` with a readable flash message
- [ ] OAuth misconfiguration (bad client ID) redirects back with a flash message

## Notes

From the architecture plan:

Flash messages are stored in the gorilla/sessions cookie under the `"flash"` key. The value is consumed (read and deleted) on the next render — one-shot display.

Updated `loginData` struct:
```go
type loginData struct {
    Providers []string
    Flash     string   // non-empty = show alert
}
```

The flash alert block should appear above the provider buttons in `login.html`. The `aria-busy` attribute on provider `<a>` or `<button>` elements is toggled via an `onclick` handler — PicoCSS renders this natively as a spinner.

Note: the current `login.html` does not extend `layout.html`. Provider links are GET requests so CSRF is not required. No changes to CSRF handling needed for this task.
