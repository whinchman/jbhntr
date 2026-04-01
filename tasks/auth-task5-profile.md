# Task: auth-task5-profile

- **Type**: coder
- **Status**: done
- **Branch**: feature/auth-task5-profile
- **Source Item**: Full Sign-In / Sign-Up Flow
- **Dependencies**: auth-task1-model

## Description

Create the account/profile page at `/profile` where authenticated users can update their display name, see which OAuth provider they are connected through, and sign out. This is separate from `/settings` (which handles search filters and resume).

## Acceptance Criteria

- [ ] `profileTmpl` is added to `Server` and parsed from `templates/layout.html` + `templates/profile.html`
- [ ] `GET /profile` and `POST /profile` routes are registered in the `requireAuth` protected group in `Handler()`
- [ ] `handleProfileGet` renders the profile page with current user data
- [ ] `handleProfileUpdate` validates `display_name` (non-empty, max 100 chars); returns inline error on failure
- [ ] `handleProfileUpdate` calls `s.userStore.UpdateUserDisplayName(ctx, user.ID, displayName)` on success
- [ ] `handleProfileUpdate` redirects to `/profile?saved=1` on success
- [ ] `templates/profile.html` is created with: display name edit form, read-only "Connected via [google|github]" section, sign out button, success indicator when `?saved=1`
- [ ] `/profile` renders the current display name and provider
- [ ] Updating display name persists and shows a "Saved" confirmation
- [ ] Submitting a blank display name shows an inline validation error
- [ ] Sign Out button clears the session and redirects to `/login`

## Notes

### Completion Summary (feature/auth-task5-profile)

Implemented on branch `feature/auth-task5-profile`.

**Files changed:**
- `internal/web/server.go` — added `profileTmpl` field to `Server`, parsed `templates/profile.html` in `NewServerWithConfig`, registered `GET /profile` and `POST /profile` routes in the `requireAuth` group, added `profileData` struct, `handleProfileGet`, and `handleProfileSave` handlers
- `internal/web/templates/profile.html` — new template: display name edit form (POST /profile), connected provider info (read-only: provider name, email, provider ID, avatar), sign out button via `hx-post="/logout"`, success/error banners
- `internal/web/templates/layout.html` — added `Profile` nav link in the `{{if .User}}` block; added `Sign in` link in the new `{{else}}` block for logged-out visitors

**Validation:** `display_name` trimmed, must be non-empty and at most 100 characters; validation errors render inline without redirect. Success redirects to `/profile?saved=1`.

Go toolchain not available in the container; no build verification possible. Code reviewed manually against existing patterns.

### From the architecture plan:

Template data struct:
```go
type profileData struct {
    User      *models.User
    CSRFToken string
    Saved     bool
    Error     string
}
```

`Saved` is derived from the `?saved=1` query parameter in the GET handler (consistent with the `/settings` pattern in the existing codebase).

The Sign Out button should use the same HTMX POST pattern as the existing sign-out control (wherever it currently appears in the layout/nav). Check the existing `layout.html` or nav partial for the correct `hx-post="/logout"` pattern to replicate.

The provider field comes from `models.User` — check the existing struct for the provider/account field name (likely `Provider` or `OAuthProvider`).

This task only depends on auth-task1-model (for `UpdateUserDisplayName`). It can be worked in parallel with tasks 2, 3, and 4.
