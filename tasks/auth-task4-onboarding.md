# Task: auth-task4-onboarding

- **Type**: coder
- **Status**: done
- **Branch**: feature/auth-task4-onboarding
- **Source Item**: Full Sign-In / Sign-Up Flow
- **Dependencies**: auth-task1-model, auth-task3-return-to

## Description

Implement the onboarding gate and onboarding form. After a successful OAuth callback, new users (where `OnboardingComplete == false`) are redirected to `/onboarding` to set their display name and optionally paste resume markdown. After submission the flag is set to true and the user is sent to their original destination or `/`.

## Acceptance Criteria

- [ ] `handleOAuthCallback` (success path) redirects to `/onboarding` if `dbUser.OnboardingComplete == false`, otherwise to `consumeReturnTo(r)` or `/`
- [ ] `GET /onboarding` and `POST /onboarding` routes are registered under `requireAuth` in `Handler()`
- [ ] `handleOnboardingGet` renders the onboarding form with display name pre-filled from the current user
- [ ] `handleOnboardingGet` redirects to `/` if the user already has `OnboardingComplete == true`
- [ ] `handleOnboardingPost` validates `display_name` is non-empty (max 100 chars); returns inline error on failure
- [ ] `handleOnboardingPost` reads optional `resume` textarea value
- [ ] `handleOnboardingPost` calls `s.userStore.UpdateUserOnboarding(ctx, user.ID, displayName, resume)`
- [ ] `handleOnboardingPost` redirects to `consumeReturnTo(r)` or `/` on success
- [ ] `onboardingTmpl` is added to `Server` and parsed from `templates/onboarding.html` + `templates/layout.html`
- [ ] `templates/onboarding.html` is created with display name input, optional resume textarea, CSRF token, and submit button
- [ ] First-time OAuth login redirects to `/onboarding`
- [ ] Submitting onboarding with a valid name sets `onboarding_complete=true` and redirects correctly
- [ ] Submitting with a blank name shows inline validation error, does not redirect
- [ ] User with `OnboardingComplete=true` navigating to `/onboarding` is redirected to `/`
- [ ] Resume textarea is pre-populated with existing value on re-visit

## Notes

### Completion Summary (2026-04-01)

Branch: `feature/auth-task4-onboarding`

**Files changed:**
- `internal/web/auth.go`: Modified `handleOAuthCallback` success path — after `setSession`, branches on `dbUser.OnboardingComplete`: new users redirect to `/onboarding` (preserving `return_to` in session), returning users call `consumeReturnTo` as before.
- `internal/web/server.go`: Added `onboardingTmpl *template.Template` field; parses `layout.html` + `onboarding.html`; registers `GET /onboarding` and `POST /onboarding` in the `requireAuth` group; added `onboardingData` struct; implemented `handleOnboardingGet` (redirects to `/` if already onboarded, else renders form with pre-filled displayName/resume) and `handleOnboardingPost` (validates display_name non-empty and ≤100 chars, calls `s.userStore.UpdateUserOnboarding`, redirects via `consumeReturnTo`).
- `internal/web/templates/onboarding.html`: New template extending layout with welcoming header, inline error display, display name input (pre-filled, required, maxlength=100), optional resume textarea (pre-filled), CSRF hidden field (`gorilla.csrf.Token`), and submit button.

All acceptance criteria satisfied. Go toolchain not available in this environment — build verification skipped.

### Original Notes

From the architecture plan:

Template data struct:
```go
type onboardingData struct {
    User        *models.User
    CSRFToken   string
    DisplayName string
    Error       string
}
```

The onboarding form uses `multipart/form-data` or standard `application/x-www-form-urlencoded` POST. Resume is stored in `users.resume_markdown` (existing column) via `UpdateUserOnboarding`. No file upload — textarea only.

CSRF token must be included in the form: `<input type="hidden" name="csrf_token" value="{{.CSRFToken}}">` (or whichever pattern is used by the existing settings/profile forms).

The `return_to` session value must be preserved through the onboarding redirect. Because `consumeReturnTo` clears the session on read, the callback should NOT consume it before redirecting to `/onboarding` — the onboarding POST handler consumes it after completion. Ensure the session save occurs before any redirect.
