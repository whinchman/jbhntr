# Architecture Plan: OAuth via Google

**Date:** 2026-04-03
**Status:** Research complete — implementation already exists

---

## Current Auth State

Google OAuth is **already fully implemented** in this codebase. All core components
are in place and tested. This plan documents the existing architecture and identifies
any remaining gaps or hardening opportunities.

### What exists today

| Component | File | Status |
|-----------|------|--------|
| OAuth config struct | `internal/config/config.go` | Done |
| `google` provider wired into `oauthProviders()` | `internal/web/auth.go:46-73` | Done |
| OAuth start handler (`GET /auth/{provider}`) | `internal/web/auth.go:242-269` | Done |
| OAuth callback handler (`GET /auth/{provider}/callback`) | `internal/web/auth.go:274-356` | Done |
| Google userinfo fetch (`fetchGoogleUser`) | `internal/web/auth.go:372-401` | Done |
| CSRF state stored in session | `internal/web/auth.go:257-266` | Done |
| State verification in callback | `internal/web/auth.go:290-302` | Done |
| Session creation on successful login | `internal/web/auth.go:341-345` | Done |
| Onboarding redirect for new users | `internal/web/auth.go:351-355` | Done |
| `return_to` open-redirect protection | `internal/web/auth.go:171-196` | Done |
| `users` table schema | `internal/store/migrations/001_create_users.sql` | Done |
| `UpsertUser` store method | `internal/store/user.go:42-61` | Done |
| Login page template with Google button | `internal/web/templates/login.html` | Done |
| Config example with Google creds | `config.yaml.example:9-12` | Done |
| Auth unit tests | `internal/web/auth_test.go` | Done |
| End-to-end OAuth integration tests (mock provider) | `internal/web/integration_test.go` | Done |

### Dependencies already in go.mod

```
golang.org/x/oauth2 v0.36.0        — core OAuth2 client
github.com/gorilla/sessions v1.4.0  — cookie-based session store
github.com/gorilla/csrf v1.7.2      — CSRF protection middleware
github.com/gorilla/securecookie v1.1.2
github.com/go-chi/chi/v5 v5.2.5    — router (URL params for {provider})
```

No new dependencies are needed.

---

## Architecture Overview

The auth system is provider-agnostic via a `map[string]*oauth2.Config` keyed by
provider name. Adding Google required only populating that map, writing
`fetchGoogleUser`, and adding a button to the login template. GitHub uses the
same plumbing.

### Data Flow

```
User clicks "Sign in with Google"
  → GET /auth/google
    → generate random state, store in session cookie
    → redirect to accounts.google.com/o/oauth2/v2/auth?state=...&client_id=...
  → Google authenticates user, redirects back to:
  → GET /auth/google/callback?code=...&state=...
    → verify state matches session (CSRF check)
    → exchange code for access token (POST oauth2.googleapis.com/token)
    → GET googleapis.com/oauth2/v2/userinfo with bearer token
    → UpsertUser (INSERT … ON CONFLICT DO UPDATE)
    → setSession (gorilla/sessions cookie, 30-day MaxAge)
    → if !OnboardingComplete → redirect /onboarding
    → else → redirect consumeReturnTo() (safe same-origin validation)
```

### Key Security Decisions

1. **CSRF on OAuth state**: State is a 32-byte random base64 value stored in
   the session cookie and compared on callback. Cleared after use.
2. **Open-redirect protection**: `consumeReturnTo` validates the `return_to`
   value must start with `/` and not start with `//` or contain `://`.
3. **CSRF middleware on all non-auth routes**: `gorilla/csrf` is mounted before
   all routes except the two public health endpoints.
4. **Secure cookie flag**: Session and CSRF cookies have `Secure: true` when
   `base_url` starts with `https`.
5. **Session HttpOnly + SameSite=Lax**: prevents XSS cookie theft and basic
   CSRF for same-origin navigation.

---

## Files Affected

All auth-related code lives in two files. No changes to the router structure,
models, or store are needed for Google OAuth specifically.

- `internal/web/auth.go` — all OAuth handlers, session helpers, provider fetch
- `internal/web/server.go` — `Server` struct, route registration, `NewServerWithConfig`
- `internal/config/config.go` — `AuthConfig`, `ProvidersConfig`, `OAuthProviderConfig`
- `internal/store/user.go` — `UpsertUser`, `GetUser`, `GetUserByProvider`
- `internal/store/migrations/001_create_users.sql` — users table
- `internal/web/templates/login.html` — login page with provider buttons
- `config.yaml.example` — shows `GOOGLE_CLIENT_ID` / `GOOGLE_CLIENT_SECRET` env vars

---

## OAuth Flow Design

### Routes

| Method | Path | Handler | Auth required |
|--------|------|---------|---------------|
| GET | `/login` | `handleLogin` | No (public) |
| GET | `/auth/{provider}` | `handleOAuthStart` | No |
| GET | `/auth/{provider}/callback` | `handleOAuthCallback` | No |
| POST | `/logout` | `handleLogout` | Yes (requireAuth + CSRF) |

The `{provider}` wildcard means Google and GitHub share the same handler pair
with zero code duplication.

### Session Handling

- Library: `gorilla/sessions` with `CookieStore` (HMAC-signed, AES-encrypted cookie)
- Session name: `jobhuntr_session`
- Keys stored:
  - `user_id` (int64) — primary user identifier
  - `oauth_state` (string) — ephemeral CSRF nonce, deleted after callback
  - `flash` (string) — one-shot error/info message consumed by next render
  - `return_to` (string) — pre-auth destination URL, validated on consumption
- MaxAge: 30 days

### CSRF Considerations

Two distinct CSRF mechanisms are in use:

1. **OAuth state parameter**: guards the `/auth/{provider}/callback` endpoint
   against CSRF login attacks. Generated per-flow, stored in session, verified
   on callback, then deleted.

2. **gorilla/csrf middleware**: guards all stateful POST endpoints (logout,
   settings, profile, onboarding) via a double-submit cookie pattern. Token
   exposed via `<meta name="csrf-token" content="...">` in layout.html and
   consumed as `X-CSRF-Token` header.

---

## Step-by-Step Implementation Breakdown

Since the implementation is complete, this section describes what a coder agent
would need to do if starting from scratch — useful for understanding the system
and for adding a third provider later.

### Step 1: Config struct (complete)

File: `internal/config/config.go`

Add to `ProvidersConfig`:
```go
Google OAuthProviderConfig `yaml:"google"`
```

`OAuthProviderConfig` already holds `ClientID` and `ClientSecret`.

### Step 2: Build the oauth2.Config for Google (complete)

File: `internal/web/auth.go`, function `oauthProviders`

```go
if authCfg.Providers.Google.ClientID != "" {
    providers["google"] = &oauth2.Config{
        ClientID:     authCfg.Providers.Google.ClientID,
        ClientSecret: authCfg.Providers.Google.ClientSecret,
        RedirectURL:  strings.TrimRight(baseURL, "/") + "/auth/google/callback",
        Scopes:       []string{"openid", "email", "profile"},
        Endpoint: oauth2.Endpoint{
            AuthURL:  "https://accounts.google.com/o/oauth2/v2/auth",
            TokenURL: "https://oauth2.googleapis.com/token",
        },
    }
}
```

Note: we use the userinfo endpoint directly rather than Google's OIDC discovery
to keep the dependency footprint minimal (no OIDC library needed).

### Step 3: Fetch userinfo from Google (complete)

File: `internal/web/auth.go`, function `fetchGoogleUser`

```go
func fetchGoogleUser(ctx context.Context, cfg *oauth2.Config, token *oauth2.Token) (*models.User, error)
```

Calls `GET https://www.googleapis.com/oauth2/v2/userinfo` with an
`oauth2.Config.Client(ctx, token)` HTTP client (automatically injects the
bearer token). Maps `id`, `email`, `name`, `picture` to `models.User`.

### Step 4: Wire into fetchProviderUser (complete)

File: `internal/web/auth.go`, function `fetchProviderUser`

```go
case "google":
    return fetchGoogleUser(ctx, cfg, token)
```

### Step 5: Login template button (complete)

File: `internal/web/templates/login.html`

The `loginData.Providers []string` slice is populated from the configured
provider map. The template ranges over it and renders a Google button only if
`"google"` is present — so unconfigured providers never appear.

### Step 6: Environment variables (complete)

`config.yaml.example` documents:
```yaml
auth:
  providers:
    google:
      client_id: "${GOOGLE_CLIENT_ID}"
      client_secret: "${GOOGLE_CLIENT_SECRET}"
```

The `config.Load()` function expands `${VAR}` placeholders from the environment
or from a `.env` file in the same directory.

---

## Remaining Gaps / Hardening Opportunities

These are not blocking — Google OAuth works end-to-end today — but are worth
tracking as follow-on work.

### 1. ID token validation (low priority)

Currently the implementation fetches userinfo via a REST call after code
exchange. An alternative is to validate the JWT `id_token` returned in the
token response directly (avoids the extra HTTP round trip to Google). This
would require `golang-jwt/jwt` or parsing the token manually.

**Recommendation**: keep the current approach. The extra round trip is
negligible and REST userinfo is simpler and more auditable.

### 2. Token refresh (not needed for current use case)

Google access tokens expire after 1 hour. Since the app only uses the token
during the login callback (no ongoing API calls to Google), no refresh logic
is needed. The gorilla/sessions session itself is what maintains the logged-in
state, not the OAuth token.

### 3. Google Console setup (operator responsibility)

To enable Google Sign-In, the operator must:
1. Go to https://console.cloud.google.com/apis/credentials
2. Create an OAuth 2.0 Client ID (Web Application type)
3. Add the callback URL: `https://<your-domain>/auth/google/callback`
4. Set `GOOGLE_CLIENT_ID` and `GOOGLE_CLIENT_SECRET` env vars (or in `.env`)
5. Set `SESSION_SECRET` to a 32+ byte random value

This is documented in `config.yaml.example` but not yet in a README section.

### 4. Account linking (future)

Currently a user who signs in with GitHub and later tries Google with the same
email creates two separate user records. `UpsertUser` matches on
`(provider, provider_id)`, not on `email`. If account linking is ever desired,
a separate migration and handler would be needed.

---

## Acceptance Criteria

- [x] `GET /auth/google` redirects to `accounts.google.com` with correct scopes (`openid email profile`)
- [x] OAuth state is stored in the session and verified on callback
- [x] Successful callback upserts the user in the `users` table (provider = "google")
- [x] New users are redirected to `/onboarding` after first login
- [x] Returning users are redirected to `return_to` (or `/`) after login
- [x] State mismatch redirects to `/login` with a flash message
- [x] Provider error (user denied consent) redirects to `/login` with a flash message
- [x] Session cookie is HttpOnly, SameSite=Lax, Secure on https
- [x] Login page shows "Sign in with Google" button only when `GOOGLE_CLIENT_ID` is set
- [x] All acceptance criteria are covered by tests in `auth_test.go` and `integration_test.go`

---

## Trade-offs and Alternatives

### Alternative A: Use `golang.org/x/oauth2/google` package endpoint

The `golang.org/x/oauth2/google` sub-package exports `google.Endpoint` (same
AuthURL/TokenURL constants we use). We chose to inline the endpoint URLs instead
to make the values explicit and avoid an additional import. Either approach is
correct.

### Alternative B: OIDC library (e.g. `coreos/go-oidc`)

Full OIDC would add nonce validation, issuer verification, and JWT signature
checks. For a single-tenant web app this adds complexity with minimal security
benefit since the code exchange already happens server-side over TLS with the
known Google token endpoint.

### Alternative C: Auth proxy (Authelia, oauth2-proxy)

For multi-tenant SaaS this would be appropriate. For a personal job search tool
with a small user base, embedding OAuth in the app is simpler to operate.

---

## Summary

**Google OAuth is fully implemented.** No new code is required for the base
feature. The implementation follows idiomatic Go patterns:

- Provider-agnostic handler pair (`/auth/{provider}`, `/auth/{provider}/callback`)
- Secure session management via gorilla/sessions
- CSRF protection via session state parameter (OAuth) and gorilla/csrf (form POSTs)
- Safe `return_to` validation preventing open-redirect
- Onboarding flow for new users
- Full test coverage including mock OAuth provider end-to-end tests

A coder agent spawned for this feature should focus on any operator-facing
documentation gaps (e.g., a "Setting up Google OAuth" section in README) or
the account-linking feature if the product requires it.
