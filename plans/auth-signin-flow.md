# Architecture Plan: Full Sign-In / Sign-Up Flow

**Feature**: Full Sign-In / Sign-Up Flow for jobhuntr  
**Date**: 2026-04-01  
**Status**: Draft

---

## Architecture Overview

The feature builds on the existing OAuth2 infrastructure (Google + GitHub via `golang.org/x/oauth2`, gorilla/sessions, gorilla/csrf, chi router). No new external dependencies are needed. All UI is server-rendered HTML via Go's `html/template` package with HTMX for lightweight interactivity, consistent with the existing codebase style.

### Core Design Decisions

1. **Flash messages via session**: store error/info strings in the session cookie (key `flash`) and consume them on the next render. This avoids a separate flash package — the gorilla/sessions store already handles it.

2. **`onboarding_complete` flag on the User model** (not `is_new_user`): a boolean column is more durable than inferring new-ness from `created_at` timestamps. The flag is set to `false` on first INSERT and `true` after the user submits the onboarding form.

3. **"Return to" URL via session**: store the originally-requested URL in the session before redirecting to `/login`. Consume and clear it in the callback handler. The value is validated against an allowlist (same-origin only) to prevent open-redirect attacks.

4. **Onboarding as a separate route** (`/onboarding`), not a modal: simpler routing, no JS complexity, back-button safe. After onboarding the user is redirected to `/`.

5. **Account/profile page at `/profile`**: separate from `/settings` (which is for search filters and resume). The profile page handles display name editing and shows the connected provider.

6. **Dashboard auth-awareness via optional auth middleware**: replace the blanket `requireAuth` on `GET /` with a new `optionalAuth` middleware that injects the user if present but does not redirect. The dashboard template checks `.User` and shows a hero/CTA to unauthenticated visitors.

7. **Resume upload on onboarding**: POST to `/onboarding` with `multipart/form-data`. The resume text is stored in `users.resume_markdown` (already a column). No file storage needed — the existing textarea approach is reused.

### Data Flow for a New User

```
Browser → GET /settings  →  requireAuth redirects to /login
                             (saves "return_to=/settings" in session)
       → GET /login      →  renders login page
       → GET /auth/google →  OAuth start (state saved in session)
       → GET /auth/google/callback
                          →  code exchange
                          →  UpsertUser (onboarding_complete=false on INSERT)
                          →  setSession
                          →  if !onboarding_complete → redirect /onboarding
                          →  else → redirect return_to or /
       → GET /onboarding  →  render onboarding form
       → POST /onboarding →  save display name + optional resume
                          →  set onboarding_complete=true
                          →  redirect to return_to or /
```

### Data Flow for a Returning User

```
Browser → GET /           →  optionalAuth: user injected, shows job table
       → POST /auth/google →  UpsertUser (ON CONFLICT: updates last_login_at only)
                           →  onboarding_complete=true → redirect return_to or /
```

---

## Data Model Changes

### 1. Add `onboarding_complete` to `users` table

New migration file: `internal/store/migrations/005_add_onboarding_complete.sql`

```sql
ALTER TABLE users ADD COLUMN onboarding_complete INTEGER NOT NULL DEFAULT 0;
```

SQLite uses INTEGER for booleans (0/1). Default 0 means existing users will be treated as needing onboarding — **this requires a data backfill** for existing users. See implementation note below.

**Backfill approach**: add to the same migration:
```sql
UPDATE users SET onboarding_complete = 1 WHERE display_name != '' AND display_name IS NOT NULL;
```

Existing users who have a `display_name` from their OAuth profile are considered already onboarded.

### 2. Update `models.User` struct

Add field to `internal/models/user.go`:
```go
OnboardingComplete bool
```

### 3. Update `store.UpsertUser`

The existing `ON CONFLICT DO UPDATE` must NOT overwrite `onboarding_complete` (same as it does not overwrite `resume_markdown`). The INSERT sets it to `0` (false) for new rows; conflicts leave it unchanged.

### 4. New store method: `UpdateUserOnboarding`

```go
func (s *Store) UpdateUserOnboarding(ctx context.Context, userID int64, displayName string, resume string) error
```

Sets `display_name = ?`, `resume_markdown = ?`, `onboarding_complete = 1` for the given user ID.

Expose this via a new `UserStore` interface method so the web layer can call it without a full store import.

### 5. New store method: `UpdateUserDisplayName`

```go
func (s *Store) UpdateUserDisplayName(ctx context.Context, userID int64, displayName string) error
```

Used by the `/profile` edit handler.

### 6. Update `scanUser`

Add `onboarding_complete` to the SELECT and Scan in `GetUser` and `GetUserByProvider`.

---

## Implementation Plan

### Task 1 — Data model + migration (coder)

**Files**:
- `internal/store/migrations/005_add_onboarding_complete.sql` — new file
- `internal/models/user.go` — add `OnboardingComplete bool`
- `internal/store/user.go` — update `scanUser` (add column to SELECT + Scan); update `UpsertUser` INSERT column list; add `UpdateUserOnboarding`; add `UpdateUserDisplayName`

**Steps**:
1. Write `005_add_onboarding_complete.sql` (ALTER TABLE + UPDATE backfill).
2. Add `OnboardingComplete bool` to `models.User`.
3. Update `scanUser`: add `onboarding_complete` to SELECT and `&u.OnboardingComplete` to Scan.
4. Update `UpsertUser` INSERT to include `onboarding_complete = 0` explicitly (not relying on DEFAULT) and ensure ON CONFLICT does NOT include `onboarding_complete`.
5. Add `UpdateUserOnboarding(ctx, userID, displayName, resume)` — single UPDATE setting all three columns.
6. Add `UpdateUserDisplayName(ctx, userID, displayName)` — UPDATE display_name only.
7. Extend `UserStore` interface in `internal/web/server.go` with `UpdateUserOnboarding` and `UpdateUserDisplayName`.

**Acceptance Criteria**:
- [ ] Migration file applies cleanly on a fresh DB and on an existing DB with users
- [ ] `models.User.OnboardingComplete` is populated correctly by `GetUser`
- [ ] New users inserted by `UpsertUser` have `onboarding_complete = false`
- [ ] Existing users with a display_name are backfilled to `onboarding_complete = true`
- [ ] `UpdateUserOnboarding` sets the flag to true and persists name + resume

---

### Task 2 — Flash messages + login page polish (coder)

**Files**:
- `internal/web/auth.go` — flash write helper, flash consume in `handleLogin`, propagate error in callback
- `internal/web/templates/login.html` — flash alert section, loading state on buttons

**Steps**:
1. Add `setFlash(w, r, msg string)` and `consumeFlash(r) string` helpers in `auth.go`. These read/write a `"flash"` key in the gorilla session.
2. In `handleOAuthCallback`: on provider error, code-exchange failure, or userinfo failure, call `setFlash(...)` then redirect to `/login` (currently these all silently redirect).
3. Update `loginData` struct:
   ```go
   type loginData struct {
       Providers []string
       Flash     string   // non-empty = show alert
   }
   ```
4. In `handleLogin`, call `consumeFlash(r)` and pass it into `loginData`.
5. Update `login.html`:
   - Add flash alert block above the provider buttons.
   - Add `aria-busy="true"` toggling via `onclick` on each provider button (PicoCSS supports this natively with `aria-busy`).
   - Use the full layout so the flash message styles are consistent.

**Acceptance Criteria**:
- [ ] Denying consent on the OAuth provider redirects back to `/login` with a readable error message visible
- [ ] OAuth misconfiguration (bad client ID) redirects back with a flash message
- [ ] Clicking a provider button shows a loading/busy state

---

### Task 3 — "Return to" redirect (coder)

**Files**:
- `internal/web/auth.go` — save/consume `return_to` in session; validate it is same-origin

**Steps**:
1. Add constants `sessionReturnTo = "return_to"`.
2. Update `requireAuth` middleware: before redirecting to `/login`, save `r.URL.RequestURI()` into the session as `return_to`. Only save if the request method is GET and the path is not `/login` or `/logout`.
3. Add helper `consumeReturnTo(r) string` — reads and deletes `return_to` from session; validates the value starts with `/` and does not contain `//` or scheme (same-origin guard); returns `/` if invalid or empty.
4. In `handleOAuthCallback` (success path, after `setSession`): call `consumeReturnTo` to get the redirect target.
5. In `handleLogin` (already-logged-in redirect): also use `consumeReturnTo` instead of hardcoded `/`.

**Acceptance Criteria**:
- [ ] Unauthenticated GET to `/settings` → login → auth → lands on `/settings`
- [ ] Unauthenticated GET to `/jobs/123` → login → auth → lands on `/jobs/123`
- [ ] `return_to` with absolute URL or `//evil.com` is ignored, falls back to `/`
- [ ] Already-authenticated user hitting `/login` is redirected to `return_to` or `/`

---

### Task 4 — Post-login routing: onboarding gate (coder)

**Files**:
- `internal/web/auth.go` — modify callback to branch on `OnboardingComplete`
- `internal/web/server.go` — register `/onboarding` routes; add `onboardingTmpl`; new handler functions; extend `UserStore` interface
- `internal/web/templates/onboarding.html` — new template

**Steps**:
1. In `handleOAuthCallback` (success path), after `setSession`:
   - If `dbUser.OnboardingComplete == false` → redirect to `/onboarding`
   - Else → redirect to `consumeReturnTo(r)` or `/`
2. Register routes in `Handler()`:
   ```
   r.Get("/onboarding", s.handleOnboardingGet)
   r.Post("/onboarding", s.handleOnboardingPost)
   ```
   Both wrapped with `requireAuth`. Also add `/onboarding` to the exclusion list in the `requireAuth` middleware so that already-complete users who somehow land there are sent to `/` instead of creating a redirect loop.

   Actually, a simpler guard: in `handleOnboardingGet`, check `user.OnboardingComplete` — if true, redirect to `/`.
3. Add `onboardingTmpl` field to `Server`; parse `templates/onboarding.html` + `templates/layout.html` in `NewServerWithConfig`.
4. `handleOnboardingGet`: render onboarding form with current display name pre-filled.
5. `handleOnboardingPost`:
   - Validate `display_name` is non-empty (max 100 chars).
   - Read optional `resume` textarea.
   - Call `s.userStore.UpdateUserOnboarding(ctx, user.ID, displayName, resume)`.
   - Redirect to `consumeReturnTo(r)` or `/`.
6. Write `templates/onboarding.html`: simple form with display name input, optional resume textarea, submit button. Extend `layout.html`.

**Onboarding template data struct**:
```go
type onboardingData struct {
    User        *models.User
    CSRFToken   string
    DisplayName string
    Error       string
}
```

**Acceptance Criteria**:
- [ ] First-time OAuth login redirects to `/onboarding`
- [ ] Submitting onboarding with a valid name sets `onboarding_complete=true` and redirects to `/` (or return_to)
- [ ] Submitting with blank name shows inline error
- [ ] User who has completed onboarding and navigates to `/onboarding` is redirected to `/`
- [ ] Resume textarea is pre-populated with existing value on re-visit

---

### Task 5 — Account/Profile page (coder)

**Files**:
- `internal/web/server.go` — add `profileTmpl`, register `GET /profile` and `POST /profile`
- `internal/web/templates/profile.html` — new template

**Steps**:
1. Add `profileTmpl` to `Server`; parse `templates/layout.html` + `templates/profile.html`.
2. Register in `Handler()` protected group:
   ```
   r.Get("/profile", s.handleProfileGet)
   r.Post("/profile", s.handleProfileUpdate)
   ```
3. `handleProfileGet`: build `profileData`, render.
4. `handleProfileUpdate`:
   - Validate `display_name` (non-empty, max 100 chars).
   - Call `s.userStore.UpdateUserDisplayName(ctx, user.ID, displayName)`.
   - Redirect to `/profile?saved=1`.
5. Write `templates/profile.html`:
   - Display name edit form (text input + submit).
   - "Connected via [google|github]" read-only section.
   - Sign out button (HTMX POST to `/logout` matching existing pattern).
   - Success flash from `?saved=1` query param (consistent with `/settings`).

**Profile template data struct**:
```go
type profileData struct {
    User      *models.User
    CSRFToken string
    Saved     bool
    Error     string
}
```

**Acceptance Criteria**:
- [ ] `/profile` renders current display name and provider
- [ ] Updating display name persists and shows "Saved" confirmation
- [ ] Blank display name shows inline validation error
- [ ] Sign Out button on profile page clears session and redirects to `/login`

---

### Task 6 — Dashboard auth-awareness + layout nav (coder)

**Files**:
- `internal/web/server.go` — new `optionalAuth` middleware; change `GET /` to use it; update `handleDashboard`
- `internal/web/templates/dashboard.html` — hero/CTA section for unauthenticated visitors
- `internal/web/templates/layout.html` — add "Sign In" link for logged-out state

**Steps**:
1. Add `optionalAuth` middleware to `auth.go`:
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
2. In `Handler()`, move `GET /` and `GET /partials/job-table` out of the `requireAuth` group into their own group that uses `optionalAuth` instead (when auth is configured). Keep other protected routes (settings, job detail, etc.) under `requireAuth`.
3. In `handleDashboard`: if `user == nil`, render the hero section (no job table). If `user != nil`, render the job table as today.
4. Update `dashboard.html` to conditionally show either a hero section or the job table based on `{{if .User}}`.
5. Update `layout.html` nav to show "Sign In" link when `.User` is nil:
   ```html
   {{if .User}}
     <!-- existing avatar + name + sign out -->
   {{else}}
     <span style="float:right;"><a href="/login">Sign In</a></span>
   {{end}}
   ```
6. Add `/profile` link to the logged-in nav section alongside "Sign out".

**Hero section content** (minimal, fits existing Pico CSS style):
- Headline: "Track your job search, automatically."
- Sub-text: brief description of what jobhuntr does.
- CTA button: "Sign In to Get Started" → links to `/login`.

**Acceptance Criteria**:
- [ ] Unauthenticated visitor to `/` sees hero section with Sign In CTA, not 302 redirect
- [ ] Authenticated user visiting `/` sees job table as before
- [ ] Layout nav shows "Sign In" link when logged out
- [ ] Layout nav shows user avatar, display name, "Profile", and "Sign out" when logged in
- [ ] `/partials/job-table` returns an empty fragment (or redirect) for unauthenticated requests (no jobs to show)

---

## File Change Summary

| File | Change Type | Task |
|------|-------------|------|
| `internal/store/migrations/005_add_onboarding_complete.sql` | New | 1 |
| `internal/models/user.go` | Modify | 1 |
| `internal/store/user.go` | Modify | 1 |
| `internal/web/auth.go` | Modify | 2, 3, 4, 6 |
| `internal/web/server.go` | Modify | 4, 5, 6 |
| `internal/web/templates/login.html` | Modify | 2 |
| `internal/web/templates/layout.html` | Modify | 6 |
| `internal/web/templates/dashboard.html` | Modify | 6 |
| `internal/web/templates/onboarding.html` | New | 4 |
| `internal/web/templates/profile.html` | New | 5 |

---

## Trade-offs and Alternatives

### Flash messages: session vs. query param

**Chosen**: session-based flash (gorilla/sessions). No URL pollution, works for POST→redirect→GET.  
**Alternative**: `?error=denied` query param. Simpler, but leaks error codes into browser history and referrer headers. Not chosen.

### Onboarding: separate route vs. modal

**Chosen**: separate `/onboarding` route.  
**Alternative**: JS modal on first login. Requires more JavaScript, breaks back-button, harder to test. Not chosen.

### `onboarding_complete` flag vs. inferring from `created_at`

**Chosen**: explicit boolean column.  
**Alternative**: if `created_at == last_login_at` then new user. Fragile — clock skew, same-second logins. Not chosen.

### `optionalAuth` vs. public dashboard + redirect

**Chosen**: `optionalAuth` middleware — serves dashboard content conditionally, single handler.  
**Alternative**: separate `/` (public) and `/dashboard` (protected) routes. Would break existing links. Not chosen.

---

## Dependencies and Prerequisites

- No new Go packages required.
- SQLite migration `005` must run before any web handlers start — already guaranteed by the startup sequence calling `store.Migrate()` before `server.Handler()`.
- The `UserStore` interface in `server.go` must be extended before Task 4/5 coders begin.

---

## Risks and Open Questions

1. **Existing users without `display_name`**: the backfill migration marks them as `onboarding_complete=true` only if `display_name != ''`. Some users may have an empty display name from the provider. These will be routed through onboarding on next login. This is the correct behavior — they should set a name.

2. **Resume upload on onboarding**: The onboarding form uses a textarea for resume markdown (same as `/settings`). If the team wants binary file upload (PDF), that is out of scope for this feature and belongs in a separate backfill.

3. **CSRF token on `login.html`**: The current `login.html` does not extend `layout.html` and does not include the CSRF meta tag or HTMX headers script. The provider links are GET requests so CSRF is not required for them. The onboarding and profile pages use POST and must include `{{.CSRFToken}}` in their forms.

4. **`/partials/job-table` under `optionalAuth`**: unauthenticated HTMX polling from the dashboard would receive empty rows (no user = userID 0 = no jobs). The 30-second auto-poll in `dashboard.html` should be conditional on `{{if .User}}` to avoid unnecessary requests.
