# Task: task2-auth-oauth

**Type:** coder
**Status:** done (verified)
**Priority:** 1
**Epic:** oauth-multi-user
**Depends On:** task1-schema-migration

## Description

Add OAuth 2.0 authentication with Google and GitHub providers, session management, and route protection middleware.

### What to build:

1. **Auth config** — Add `AuthConfig` to `internal/config/config.go` with session secret, provider client IDs/secrets (env var interpolation)

2. **Dependencies** — Add `golang.org/x/oauth2`, `gorilla/sessions`, `gorilla/csrf` to `go.mod`

3. **OAuth handlers** (`internal/web/auth.go`):
   - OAuth provider configuration (Google + GitHub)
   - `handleLogin` — render login page with provider buttons
   - `handleOAuthCallback` — exchange code for token, fetch user info, upsert user in DB, create session
   - `handleLogout` — clear session
   - `requireAuth` middleware — check session for valid user_id, redirect to /login if missing, inject *models.User into context
   - Session helper functions: `getUserFromSession`, `setSession`, `clearSession`

4. **Login template** (`internal/web/templates/login.html`):
   - Clean page with Pico CSS
   - "Sign in with Google" and "Sign in with GitHub" buttons
   - Links to `/auth/{provider}/callback` to start OAuth flow

5. **Router updates** (`internal/web/server.go`):
   - Public routes: `/login`, `/auth/{provider}/callback`, `/health`
   - Protected group: all existing routes wrapped in `requireAuth`
   - CSRF middleware with meta tag for HTMX

## Acceptance Criteria

- [x] `AuthConfig` struct exists in config with provider settings and session secret
- [x] `golang.org/x/oauth2`, `gorilla/sessions`, `gorilla/csrf` in go.mod
- [x] `internal/web/auth.go` implements login, callback, logout handlers
- [x] `requireAuth` middleware checks session and injects user into context
- [x] `/login` renders a login page with Google and GitHub buttons
- [x] `/auth/{provider}/callback` handles OAuth code exchange and session creation
- [x] `/logout` clears the session
- [x] All existing routes are protected behind `requireAuth`
- [x] `/login`, `/auth/*`, `/health` are public
- [x] CSRF protection is active with HTMX integration
- [x] `go build ./...` succeeds
- [x] `go test ./...` passes

## Context

- Web stack: Chi router + HTMX + Pico CSS, Go html/template with //go:embed
- Existing server: `internal/web/server.go`
- User model and store methods from task1: `internal/models/user.go`, `internal/store/user.go`
- See full plan: `plans/oauth-multi-user.md` (sections 1, 3, 5, 7)

## Design

Full technical design document: `plans/task2-auth-oauth-design.md`

### Files to create
- `internal/web/auth.go` — OAuth handlers, session helpers, requireAuth middleware
- `internal/web/templates/login.html` — standalone login page with provider buttons

### Files to modify
- `internal/config/config.go` — add `AuthConfig`, `ProvidersConfig`, `OAuthProviderConfig` structs; add `Auth AuthConfig` field to `Config`
- `internal/web/server.go` — add `UserStore` interface, new Server fields (`userStore`, `sessionStore`, `oauthProviders`, `baseURL`, `loginTmpl`), update `NewServerWithConfig` signature, restructure `Handler()` router into public/protected groups with CSRF middleware
- `internal/web/templates/layout.html` — add CSRF meta tag, HTMX CSRF header injection script, user info + sign-out in nav
- `cmd/jobhuntr/main.go` — pass `db` as both JobStore and UserStore to `NewServerWithConfig`
- `go.mod` — add `golang.org/x/oauth2`, `github.com/gorilla/sessions`, `github.com/gorilla/csrf`

### Key structs

```go
// internal/config/config.go
type AuthConfig struct {
    SessionSecret string          `yaml:"session_secret"`
    Providers     ProvidersConfig `yaml:"providers"`
}
type ProvidersConfig struct {
    Google OAuthProviderConfig `yaml:"google"`
    GitHub OAuthProviderConfig `yaml:"github"`
}
type OAuthProviderConfig struct {
    ClientID     string `yaml:"client_id"`
    ClientSecret string `yaml:"client_secret"`
}
```

```go
// internal/web/auth.go
type contextKey string
const userContextKey contextKey = "user"

const (
    sessionName    = "jobhuntr_session"
    sessionUserID  = "user_id"
    sessionMaxAge  = 30 * 24 * 60 * 60
    oauthStateName = "oauth_state"
)
```

```go
// internal/web/server.go — new interface
type UserStore interface {
    GetUser(ctx context.Context, id int64) (*models.User, error)
    UpsertUser(ctx context.Context, user *models.User) (*models.User, error)
}
```

### Method signatures (auth.go)

- `func UserFromContext(ctx context.Context) *models.User` — extract user from context
- `func oauthProviders(authCfg config.AuthConfig, baseURL string) map[string]*oauth2.Config` — build provider configs
- `func (s *Server) getUserFromSession(r *http.Request) (*models.User, bool)` — load user from session cookie
- `func (s *Server) setSession(w http.ResponseWriter, r *http.Request, user *models.User) error` — create session
- `func (s *Server) clearSession(w http.ResponseWriter, r *http.Request)` — destroy session
- `func generateState() (string, error)` — random OAuth state parameter
- `func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request)` — render login page
- `func (s *Server) handleOAuthStart(w http.ResponseWriter, r *http.Request)` — redirect to provider
- `func (s *Server) handleOAuthCallback(w http.ResponseWriter, r *http.Request)` — exchange code, upsert user, set session
- `func (s *Server) fetchProviderUser(ctx, provider, cfg, token) (*models.User, error)` — dispatch to provider-specific fetch
- `func fetchGoogleUser(ctx, cfg, token) (*models.User, error)` — GET googleapis userinfo
- `func fetchGitHubUser(ctx, cfg, token) (*models.User, error)` — GET api.github.com/user
- `func fetchGitHubPrimaryEmail(ctx, client) (string, error)` — fallback for private emails
- `func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request)` — clear session, redirect
- `func (s *Server) requireAuth(next http.Handler) http.Handler` — middleware

### OAuth flow
1. User visits `/login`, sees provider buttons
2. Clicks "Sign in with Google" → GET `/auth/google`
3. `handleOAuthStart` generates random state, stores in session, 302 redirects to Google auth URL
4. User consents at Google
5. Google 302 redirects to `/auth/google/callback?code=X&state=Y`
6. `handleOAuthCallback` verifies state, exchanges code for token, fetches userinfo, upserts user, sets session cookie, 302 redirects to `/`

### Router structure
- **Public**: `/login`, `/auth/{provider}`, `/auth/{provider}/callback`, `/health`
- **Protected** (wrapped in `requireAuth`): `/`, `/partials/job-table`, `/jobs/{id}`, `/output/*`, `/settings*`, `/logout`, `/api/jobs/*`
- CSRF middleware applied globally via `csrf.Protect()`, guarded by `if s.sessionStore != nil` for backward compat with tests

### CSRF + HTMX
- `<meta name="csrf-token">` in layout.html head
- DOMContentLoaded script reads meta, sets `hx-headers` on body with `X-CSRF-Token`
- `gorilla/csrf` checks `X-CSRF-Token` header on POST requests
- Template data structs (`dashboardData`, `jobDetailData`, `settingsData`) gain `CSRFToken string` and `User *models.User` fields

### Backward compatibility
- `NewServer(st JobStore)` continues to work with nil auth — no sessionStore means no CSRF, no requireAuth
- All existing tests pass without modification

## Notes

Implementation complete. All items from the design doc at `plans/task2-auth-oauth-design.md` have been implemented:

- Added `AuthConfig`, `ProvidersConfig`, `OAuthProviderConfig` to `internal/config/config.go`
- Added `golang.org/x/oauth2`, `github.com/gorilla/sessions`, `github.com/gorilla/csrf` dependencies
- Created `internal/web/auth.go` with OAuth handlers (login, start, callback, logout), session helpers, requireAuth middleware, and provider-specific user info fetchers (Google + GitHub)
- Created `internal/web/templates/login.html` as standalone Pico CSS login page
- Updated `internal/web/server.go`: added `UserStore` interface, new Server fields, updated `NewServerWithConfig` signature to accept UserStore, restructured router into public/protected groups with CSRF middleware
- Updated `internal/web/templates/layout.html` with CSRF meta tag, HTMX CSRF header injection script, user info + sign-out in nav
- Updated `cmd/jobhuntr/main.go` to pass `db` as both JobStore and UserStore
- Created `internal/web/auth_test.go` with 8 test cases covering requireAuth, login, logout, OAuth start, callback state mismatch, and UserFromContext
- All existing tests pass without modification (backward compatible via `NewServer(st)` convenience constructor)
- `go build ./...`, `go test ./...`, and `go vet ./...` all pass

## Review

**Reviewer:** code-reviewer agent
**Commit:** 1f2c560 (fix commit on top of 24865d0)

### Verdict: Approved with fixes applied

The implementation is solid overall. The OAuth flow, session management, route
protection, and CSRF integration are all correctly implemented and match the
design doc. Three issues were found and fixed in commit 1f2c560.

### Bugs found and fixed

1. **Flaky TestHandleLogout (HIGH severity)**
   - Root cause: `Body.Read(buf)` does not guarantee it reads all bytes. When
     the partial read cut through the CSRF token, the test extracted a truncated
     or incorrect token. Additionally, `html/template` HTML-encodes `+` as
     `&#43;` in attribute values, so base64 tokens containing `+` were
     corrupted when extracted via raw string matching.
   - Fix: Use `io.ReadAll` for complete body reads and `html.UnescapeString` to
     decode HTML entities from the CSRF meta tag value.

2. **Nil pointer panic on /login without auth configured (LOW severity)**
   - `getUserFromSession` called `s.sessionStore.Get()` without checking if
     `sessionStore` is nil. If auth is not configured but `/login` is accessed,
     this would panic.
   - Fix: Added nil guard at the top of `getUserFromSession`.

3. **HTMX logout swaps login page into anchor element (MEDIUM severity)**
   - The sign-out link uses `hx-post="/logout"` which sends an AJAX POST. The
     handler returned a 303 redirect, which HTMX follows and swaps the
     response HTML into the triggering element (the anchor tag). This results
     in a broken UI.
   - Fix: Detect `HX-Request` header and return `HX-Redirect: /login` header
     with 200 status, causing HTMX to perform a full-page navigation.

### Security review

- **OAuth state parameter**: Properly generated with crypto/rand (32 bytes),
  stored in session cookie, validated in callback, deleted after use. No reuse
  possible since OAuth authorization codes are single-use.
- **Session cookies**: HttpOnly, SameSite=Lax, Secure when baseURL is HTTPS,
  Path="/". MaxAge=30 days.
- **CSRF protection**: gorilla/csrf applied globally when auth is configured.
  Meta tag approach for HTMX works correctly (browser decodes HTML entities
  from meta content attribute). Token required on all POST routes.
- **No token leakage in logs**: slog messages do not log tokens, codes, or
  session data. Only provider name and error messages are logged.
- **OAuth code exchange**: Uses standard oauth2 library, no code reuse risk.
- **Provider error handling**: Errors from provider (user denied consent) are
  handled gracefully with redirect to /login.

### Code quality

- Error wrapping with `%w` used throughout (e.g., `fmt.Errorf("google userinfo request: %w", err)`).
- All functions under 50 lines.
- slog used for all logging.
- Table-driven tests with `t.Run` subtests.
- No global state; all dependencies passed via Server struct fields.
- `UserStore` interface properly abstracts the store dependency.
- Backward compatibility maintained: `NewServer(st)` still works, all
  pre-existing tests pass without modification.

### Notes for future tasks

- Several handlers still pass `userID=0` to store methods with
  `// TODO(task3): extract userID from session context` comments. These need
  to be updated when task3 wires up per-user data access.
- The login page always shows both Google and GitHub buttons regardless of
  which providers are configured. A future enhancement could conditionally
  render only configured providers.

## QA

**Verified by:** QA agent
**Commit:** 26aad20 (edge-case tests on top of 1f2c560)

### Acceptance Criteria Verification

- [x] `AuthConfig` struct exists in `internal/config/config.go` with `SessionSecret`, `Providers` (Google + GitHub)
- [x] `golang.org/x/oauth2` v0.36.0, `gorilla/sessions` v1.4.0, `gorilla/csrf` v1.7.2 in go.mod
- [x] `internal/web/auth.go` implements login, OAuth start, callback, logout handlers plus session helpers
- [x] `requireAuth` middleware checks session, redirects to `/login` if missing, injects `*models.User` into context
- [x] `/login` renders HTML login page with Google and GitHub sign-in buttons (Pico CSS)
- [x] `/auth/{provider}/callback` handles OAuth code exchange and session creation
- [x] `/logout` clears session; HTMX-aware (returns `HX-Redirect` header for AJAX requests)
- [x] All existing routes (`/`, `/jobs/*`, `/settings*`, `/api/jobs/*`, `/partials/*`, `/output/*`) protected behind `requireAuth`
- [x] `/login`, `/auth/{provider}`, `/auth/{provider}/callback`, `/health` are public
- [x] CSRF protection active via `gorilla/csrf` with meta tag + HTMX `X-CSRF-Token` header injection
- [x] `go build ./...` succeeds
- [x] `go test ./...` passes (all packages, 0 failures)
- [x] `go vet ./...` clean

### Backward Compatibility

- [x] `NewServer(st JobStore)` with nil auth still works — no CSRF, no requireAuth applied
- [x] All 15 pre-existing tests in `server_test.go` pass without modification

### Edge-Case Tests Added (7 new tests)

| Test | Scenario | Result |
|------|----------|--------|
| `TestHandleOAuthCallback_UnknownProvider` | `/auth/unknown/callback` returns 400 | PASS |
| `TestHandleOAuthCallback_ProviderError` | Provider returns `?error=access_denied` — redirects to `/login` | PASS |
| `TestRequireAuth_DeletedUser` | Session references user ID not in store — redirects to `/login` | PASS |
| `TestDoubleLogout` | POST `/logout` without session — blocked by CSRF (403) | PASS |
| `TestHandleLogout_HTMX` | HTMX POST `/logout` returns `HX-Redirect: /login` with 200 | PASS |
| `TestProtectedRoutes_Unauthenticated` | 6 protected routes all redirect to `/login` without auth | PASS |
| `TestPublicRoutes_NoAuth` | `/login` and `/health` accessible without auth | PASS |

### Bugs Found

None. Implementation is solid. All edge cases handled correctly:
- Invalid providers rejected at both start and callback
- OAuth state verified with proper cookie round-trip
- Provider errors (user denied consent) handled gracefully
- Deleted/missing users in session handled with redirect, not panic
- HTMX logout uses `HX-Redirect` for correct full-page navigation

### Final Test Count

26 tests in `internal/web/` (15 pre-existing + 8 original auth + 7 new QA edge-case tests). All pass.
