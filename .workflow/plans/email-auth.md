# Architecture Plan: Email/Password Authentication

**Feature:** Replace OAuth with standard email+password auth  
**Date:** 2026-04-03  
**Author:** Architect agent

---

## 1. Current Auth State

### What exists today

The current auth system is entirely OAuth-based:

- **Providers:** Google and GitHub (configured via `config.AuthConfig.Providers`)
- **Session management:** gorilla/sessions cookie store, keyed by `cfg.Auth.SessionSecret`
- **CSRF protection:** gorilla/csrf middleware, applied when a session store is configured
- **User model:** `models.User` has `Provider` (string) and `ProviderID` (string) as the primary identity; no password field
- **Users table:** `UNIQUE(provider, provider_id)` constraint; no password_hash, no token columns
- **Upsert pattern:** `UpsertUser` inserts/updates on `(provider, provider_id)` conflict
- **Login page:** `login.html` renders only OAuth provider buttons; no form fields
- **Onboarding:** New OAuth users are redirected to `/onboarding` to pick a display name and paste a resume
- **`loginData` struct:** carries `Providers []string` and `Flash string`

### What changes

| Component | Change |
|-----------|--------|
| `internal/config/config.go` | Add `EmailAuthConfig` (SMTP settings, `enabled` flags) to `AuthConfig` |
| `internal/models/user.go` | Add `PasswordHash`, `EmailVerified`, `EmailVerifyToken`, `ResetToken`, `ResetExpiresAt` fields |
| `internal/store/migrations/` | Add migration 009 (password/token columns) |
| `internal/store/user.go` | Add `CreateUserWithPassword`, `GetUserByEmail`, `SetPasswordHash`, `SetEmailVerified`, `SetResetToken`, `ConsumeResetToken`, `SetVerifyToken`, `ConsumeVerifyToken` |
| `internal/web/auth.go` | Add email/password handlers; gate OAuth behind config flag |
| `internal/web/server.go` | Register new routes; add `EmailSender` to `Server` |
| `internal/web/templates/login.html` | Add email+password form; keep OAuth buttons conditionally |
| `internal/web/templates/` | Add `register.html`, `forgot.html`, `reset_password.html`, `verify_email.html` |
| `internal/mailer/` | New package: SMTP email sender |
| `internal/config/config.go` | Add `SMTPConfig` struct |

### What stays (unchanged)

- gorilla/sessions session store and all session helpers (`setSession`, `clearSession`, `setFlash`, `consumeFlash`, `consumeReturnTo`)
- gorilla/csrf middleware
- `requireAuth` and `optionalAuth` middleware
- `UserFromContext` helper
- `UpsertUser` (used by OAuth path)
- All other store methods, job handlers, settings handlers
- Onboarding flow (email-registered users also go through onboarding unless we skip it — see §4.3)

---

## 2. Database Migrations

### Migration 009: Add email-auth columns

File: `internal/store/migrations/009_add_email_auth.sql`

```sql
-- Password hash (bcrypt). NULL means the account was created via OAuth only.
ALTER TABLE users ADD COLUMN IF NOT EXISTS password_hash TEXT;

-- Email verification. Default true for existing OAuth users (their email was
-- verified by the provider).
ALTER TABLE users ADD COLUMN IF NOT EXISTS email_verified INTEGER NOT NULL DEFAULT 1;

-- One-time email verification token and its expiry.
ALTER TABLE users ADD COLUMN IF NOT EXISTS email_verify_token TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS email_verify_expires_at TIMESTAMPTZ;

-- Password-reset token and its expiry.
ALTER TABLE users ADD COLUMN IF NOT EXISTS reset_token TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS reset_expires_at TIMESTAMPTZ;

-- Unique index on email for GetUserByEmail lookups. We need to allow multiple
-- rows with empty email (legacy rows), so index only non-empty emails.
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email_unique
    ON users (email)
    WHERE email != '';
```

**Notes on the unique email index:**
- Existing OAuth rows all have `provider`+`provider_id` as their identity. If two OAuth accounts share an email (e.g. same address at Google and GitHub) there could be a conflict. The partial unique index (`WHERE email != ''`) handles empty-email legacy rows. Rows with the same email but different providers are a known edge case — see §7 (Security).
- `email_verified = 1` default means existing OAuth users are marked as verified (their email came from a verified OAuth source). New email/password registrations start with `email_verified = 0`.

---

## 3. Email Sending Approach

### Options evaluated

| Option | Self-hosted | Render | Complexity | Cost |
|--------|-------------|--------|------------|------|
| Raw SMTP via `net/smtp` (stdlib) | Yes | No (port 25 blocked) | Low | Free |
| SMTP via authenticated relay (Gmail, Mailgun SMTP, SendGrid SMTP) | Yes (requires relay) | Yes | Low | Free tier |
| Mailgun HTTP API | Yes (outbound HTTP) | Yes | Medium | Free tier (100/day) |
| Postmark HTTP API | Yes | Yes | Medium | Paid |
| Resend HTTP API | Yes | Yes | Low | Free tier (100/day) |

### Recommendation: SMTP relay with `net/smtp` + `golang.org/x/net/smtp`

**Use Go's standard `net/smtp` with TLS (port 587 STARTTLS or port 465 TLS).**

Rationale:
- Zero new dependencies: `net/smtp` is stdlib; for TLS we can use `crypto/tls` (also stdlib)
- Works self-hosted when pointed at any SMTP relay (Gmail, Mailgun SMTP gateway, Sendgrid SMTP, or a local Postfix)
- Works on Render when pointed at an authenticated relay (Render blocks port 25, but 587/465 are open)
- Simplest possible interface: one function `SendMail(to, subject, body string) error`
- Easy to swap for a transactional HTTP API later without changing callers

**Alternative (if SMTP relay is unavailable):** Resend's HTTP API is the cleanest minimal alternative — single HTTP POST, generous free tier, no SDK required. The plan includes a note about this in the config.

**Library:** No new library needed. The standard `net/smtp` package handles authenticated SMTP. We will create `internal/mailer/mailer.go` with a thin wrapper.

### `internal/mailer/mailer.go` interface

```go
package mailer

// Mailer sends transactional emails.
type Mailer interface {
    SendMail(ctx context.Context, to, subject, body string) error
}

// SMTPMailer implements Mailer using authenticated SMTP.
type SMTPMailer struct {
    host     string  // e.g. "smtp.mailgun.org"
    port     int     // e.g. 587
    username string
    password string
    from     string  // e.g. "noreply@jobhuntr.example.com"
}

func NewSMTPMailer(host string, port int, username, password, from string) *SMTPMailer

func (m *SMTPMailer) SendMail(ctx context.Context, to, subject, body string) error
// Implementation: dial host:port with STARTTLS, AUTH PLAIN, send RFC 2822 message.
// Body is plain text (no HTML required for MVP).

// NoopMailer drops all emails — used in tests and when email is disabled.
type NoopMailer struct{}
func (n *NoopMailer) SendMail(ctx context.Context, to, subject, body string) error { return nil }
```

---

## 4. Full Flow Design

### 4.1 Registration Flow

```
POST /register
  → validate email + password (min 8 chars)
  → check email not already registered (GetUserByEmail)
  → bcrypt hash password (cost 12)
  → generate 32-byte random email verify token (hex-encoded)
  → set email_verify_expires_at = now + 24h
  → CreateUserWithPassword(email, displayName, passwordHash, verifyToken, verifyExpiresAt)
  → send verification email: "Verify your email: https://<base>/verify-email?token=<token>"
  → setSession(user)   ← log them in immediately
  → redirect to /onboarding (same as OAuth new users)
```

**Design decision:** Log the user in immediately after registration (before email verification). This matches the UX of most SaaS products and avoids friction. Protected routes that require a verified email can check `user.EmailVerified` and redirect to a "please verify" page if needed. For MVP, we do NOT gate any routes on email verification — the verify link simply marks the account as verified.

### 4.2 Email Verification Flow

```
GET /verify-email?token=<token>
  → ConsumeVerifyToken(token)
      → find user where email_verify_token = token AND email_verify_expires_at > NOW()
      → set email_verified = 1, clear token and expiry
  → setFlash("Your email has been verified.")
  → redirect to /
```

If token not found or expired:
```
  → setFlash("Verification link is invalid or has expired.")
  → redirect to /login
```

### 4.3 Login Flow

```
GET /login  →  render login.html (email+password form + optional OAuth buttons)

POST /login
  → parse email + password from form
  → GetUserByEmail(email)
  → if not found: sleep 200ms (timing equalization), flash "Invalid email or password", redirect /login
  → bcrypt.CompareHashAndPassword(user.PasswordHash, password)
  → if mismatch: flash "Invalid email or password", redirect /login
  → setSession(user)
  → if !user.OnboardingComplete: redirect /onboarding
  → redirect consumeReturnTo()
```

**Note on generic error messages:** We intentionally return the same message for "user not found" and "wrong password" to prevent user enumeration.

### 4.4 Forgot Password Flow

```
GET /forgot-password  →  render forgot.html (email input)

POST /forgot-password
  → parse email
  → GetUserByEmail(email) — if not found, still show success (no enumeration)
  → if found AND user.PasswordHash != nil (i.e. has a password account):
      → generate 32-byte random reset token (hex-encoded)
      → set reset_expires_at = now + 1h
      → SetResetToken(userID, token, expiresAt)
      → send reset email: "Reset your password: https://<base>/reset-password?token=<token>"
  → always: flash "If that email is registered, a reset link has been sent."
  → redirect /login
```

### 4.5 Reset Password Flow

```
GET /reset-password?token=<token>
  → find user by reset token (not yet expired)
  → if not found/expired: flash "Reset link is invalid or has expired.", redirect /forgot-password
  → render reset_password.html (new password + confirm fields, token in hidden input)

POST /reset-password
  → parse token + new_password + confirm_password
  → validate passwords match, min 8 chars
  → ConsumeResetToken(token)
      → find user where reset_token = token AND reset_expires_at > NOW()
      → bcrypt hash new_password (cost 12)
      → set password_hash = newHash, clear reset_token, reset_expires_at
      → return user
  → setSession(user)   ← log them in after reset
  → flash "Your password has been updated."
  → redirect /
```

### 4.6 OAuth Gate (disable without deleting)

In `NewServerWithConfig`, the OAuth providers map is populated only when `cfg.Auth.OAuth.Enabled` is true (new config flag) AND provider credentials are set. The `handleLogin` template data will only receive `Providers` entries when OAuth is enabled.

In `server.go`:
```go
if cfg.Auth.OAuth.Enabled {
    srv.oauthProviders = oauthProviders(cfg.Auth, cfg.Server.BaseURL)
}
```

OAuth route registration in `Handler()` stays gated on `len(s.oauthProviders) > 0`.

This means: with `oauth_enabled: false` in config, OAuth routes are not registered and login page shows only the email form. The OAuth code in `auth.go` remains intact and operational if re-enabled.

---

## 5. Security Considerations

### 5.1 Password hashing
- **Algorithm:** bcrypt, cost factor **12** (≈300ms on a modern server — enough to slow brute force, not enough to noticeably delay legitimate login)
- **Library:** `golang.org/x/crypto/bcrypt` — already available as a transitive dependency of the oauth2 package, and idiomatic in Go
  - Add explicitly: `go get golang.org/x/crypto`

### 5.2 Token entropy
- Email verify and reset tokens: `crypto/rand` 32 bytes → hex-encoded → 64-char string
- This gives 256 bits of entropy — brute-force infeasible

### 5.3 Token expiry
- Email verify token: **24 hours** (generous; users may be slow to verify)
- Reset token: **1 hour** (short; limits exposure if email is compromised)

### 5.4 Timing attacks
- `GetUserByEmail` returning "not found" before `bcrypt.Compare` creates a timing difference. Mitigate with a fixed ~200ms sleep on the "not found" branch of `/login` and `/forgot-password`. This matches common Go auth patterns.

### 5.5 Rate limiting
- The existing `golang.org/x/time/rate` package is already in `go.mod`. Add a per-IP rate limiter middleware for `/login`, `/register`, `/forgot-password`, and `/reset-password`:
  - 5 requests per minute per IP (token bucket, burst=5)
  - Return HTTP 429 with flash message on limit exceeded
- Rate limiter is stored in `Server` as a `sync.Map[string]*rate.Limiter`

### 5.6 CSRF
- All new POST handlers are behind the existing gorilla/csrf middleware (already applied globally when `sessionStore != nil`)
- Forms must include `{{ .CSRFToken }}` → `<input type="hidden" name="gorilla.csrf.Token" value="{{.CSRFToken}}">`

### 5.7 Email enumeration
- `/forgot-password` always shows the same flash message regardless of whether the email is registered
- `/login` uses a single "Invalid email or password" message

### 5.8 Unique email constraint
- The partial unique index on `email WHERE email != ''` prevents two email/password accounts with the same address
- Edge case: if an OAuth user (with email "x@y.com") later tries to register with email/password using the same address, `CreateUserWithPassword` returns a conflict error. The handler should flash "An account with that email already exists. Try signing in."

---

## 6. Routes and Handler Signatures

### New routes (add to `server.go` `Handler()`)

All new POST handlers are registered inside the `if s.sessionStore != nil` block, alongside existing OAuth routes.

```
GET  /register            handleRegisterGet(w, r)
POST /register            handleRegisterPost(w, r)
POST /login               handleLoginPost(w, r)         ← login GET already exists
GET  /forgot-password     handleForgotPasswordGet(w, r)
POST /forgot-password     handleForgotPasswordPost(w, r)
GET  /reset-password      handleResetPasswordGet(w, r)
POST /reset-password      handleResetPasswordPost(w, r)
GET  /verify-email        handleVerifyEmail(w, r)
```

### Handler signatures in `internal/web/auth.go`

```go
func (s *Server) handleRegisterGet(w http.ResponseWriter, r *http.Request)
func (s *Server) handleRegisterPost(w http.ResponseWriter, r *http.Request)
func (s *Server) handleLoginPost(w http.ResponseWriter, r *http.Request)
func (s *Server) handleForgotPasswordGet(w http.ResponseWriter, r *http.Request)
func (s *Server) handleForgotPasswordPost(w http.ResponseWriter, r *http.Request)
func (s *Server) handleResetPasswordGet(w http.ResponseWriter, r *http.Request)
func (s *Server) handleResetPasswordPost(w http.ResponseWriter, r *http.Request)
func (s *Server) handleVerifyEmail(w http.ResponseWriter, r *http.Request)
```

### Template data structs

```go
// loginData extended — add email/password form support
type loginData struct {
    Providers  []string   // OAuth providers (empty if OAuth disabled)
    Flash      string
    CSRFToken  string
    Email      string     // pre-fill email on validation error
}

type registerData struct {
    Flash     string
    CSRFToken string
    Email     string
}

type forgotPasswordData struct {
    Flash     string
    CSRFToken string
}

type resetPasswordData struct {
    Flash     string
    Token     string     // hidden field value
    CSRFToken string
}
```

### Updated `UserStore` interface (in `server.go`)

Add new methods:
```go
type UserStore interface {
    // existing:
    GetUser(ctx context.Context, id int64) (*models.User, error)
    UpsertUser(ctx context.Context, user *models.User) (*models.User, error)
    UpdateUserOnboarding(ctx context.Context, userID int64, displayName string, resume string) error
    UpdateUserDisplayName(ctx context.Context, userID int64, displayName string) error

    // new:
    CreateUserWithPassword(ctx context.Context, email, displayName, passwordHash, verifyToken string, verifyExpiresAt time.Time) (*models.User, error)
    GetUserByEmail(ctx context.Context, email string) (*models.User, error)
    SetResetToken(ctx context.Context, userID int64, token string, expiresAt time.Time) error
    ConsumeResetToken(ctx context.Context, token string) (*models.User, error)
    SetEmailVerifyToken(ctx context.Context, userID int64, token string, expiresAt time.Time) error
    ConsumeVerifyToken(ctx context.Context, token string) (*models.User, error)
}
```

### `EmailSender` interface (add to `server.go`)

```go
type EmailSender interface {
    SendMail(ctx context.Context, to, subject, body string) error
}
```

Add to `Server` struct:
```go
mailer EmailSender
```

---

## 7. Config Changes

### Updated `AuthConfig`

```go
type AuthConfig struct {
    SessionSecret string          `yaml:"session_secret"`
    OAuth         OAuthConfig     `yaml:"oauth"`
    Providers     ProvidersConfig `yaml:"providers"`   // kept for backward compat
}

type OAuthConfig struct {
    Enabled bool `yaml:"enabled"`
}
```

### New `SMTPConfig`

```go
type SMTPConfig struct {
    Host     string `yaml:"host"`
    Port     int    `yaml:"port"`
    Username string `yaml:"username"`
    Password string `yaml:"password"`
    From     string `yaml:"from"`
}
```

Add to `Config`:
```go
SMTP SMTPConfig `yaml:"smtp"`
```

### Example `config.yaml` additions

```yaml
auth:
  session_secret: ${SESSION_SECRET}
  oauth:
    enabled: false          # set true to re-enable OAuth buttons on login page
  providers:
    google:
      client_id: ""
      client_secret: ""
    github:
      client_id: ""
      client_secret: ""

smtp:
  host: ${SMTP_HOST}        # e.g. smtp.mailgun.org
  port: 587
  username: ${SMTP_USERNAME}
  password: ${SMTP_PASSWORD}
  from: noreply@example.com
```

### Mailer construction in `main.go`

```go
var mailer mailer.Mailer
if cfg.SMTP.Host != "" {
    mailer = mailer.NewSMTPMailer(cfg.SMTP.Host, cfg.SMTP.Port, cfg.SMTP.Username, cfg.SMTP.Password, cfg.SMTP.From)
} else {
    mailer = &mailer.NoopMailer{}
    slog.Warn("SMTP not configured — emails will be dropped")
}
```

Pass `mailer` into `NewServerWithConfig` as a new parameter (or via a `WithMailer` option method to avoid a breaking change to the existing signature).

**Preferred approach:** Add a `WithMailer(m EmailSender) *Server` method (like the existing `WithLastScrapeFn`) to avoid changing `NewServerWithConfig` signature.

---

## 8. Models Changes

### `internal/models/user.go`

Add to `User` struct:
```go
PasswordHash         *string    // nil = OAuth-only account
EmailVerified        bool
EmailVerifyToken     *string
EmailVerifyExpiresAt *time.Time
ResetToken           *string
ResetExpiresAt       *time.Time
```

Use `*string` (pointer) so we can detect NULL vs empty string in the DB scan.

---

## 9. Step-by-Step Implementation Breakdown

Each step is a discrete, independently testable unit of work. Steps 1–4 are foundational; steps 5–9 build on them. Steps 5–9 can be parallelized across two coders after steps 1–4 are done.

### Step 1 — Migration and model update (coder)
**Files:** `internal/store/migrations/009_add_email_auth.sql`, `internal/models/user.go`, `internal/store/user.go`

1. Create `009_add_email_auth.sql` (as specified in §2)
2. Add new pointer fields to `models.User`
3. Update `scanUser` to scan the 5 new nullable columns (use `sql.NullString` / `sql.NullTime`)
4. Implement new store methods:
   - `CreateUserWithPassword`
   - `GetUserByEmail`
   - `SetResetToken` / `ConsumeResetToken`
   - `SetEmailVerifyToken` / `ConsumeVerifyToken`
5. Add new methods to `UserStore` interface in `server.go`
6. Write unit tests in `internal/store/user_test.go`

### Step 2 — Mailer package (coder)
**Files:** `internal/mailer/mailer.go`, `internal/mailer/mailer_test.go`

1. Create `internal/mailer/mailer.go` with `Mailer` interface, `SMTPMailer`, `NoopMailer`
2. `SMTPMailer.SendMail`: dial STARTTLS (port 587) or TLS (port 465 based on port value), AUTH PLAIN, send RFC 2822 message
3. Write unit test using `NoopMailer` (SMTP itself is an integration concern)

### Step 3 — Config extension (coder)
**Files:** `internal/config/config.go`

1. Add `SMTPConfig` struct and `SMTP` field to `Config`
2. Add `OAuthConfig.Enabled` bool to `AuthConfig`
3. Update `config_test.go` to cover the new fields

### Step 4 — Server wiring (coder)
**Files:** `internal/web/server.go`

1. Add `EmailSender` interface
2. Add `mailer EmailSender` field to `Server`
3. Add `WithMailer(m EmailSender) *Server` method
4. Gate `oauthProviders` population behind `cfg.Auth.OAuth.Enabled`
5. Register new routes in `Handler()` (inside `if s.sessionStore != nil` block)
6. Register new templates: `register.html`, `forgot.html`, `reset_password.html`, `verify_email.html`
7. Add new template fields to `NewServerWithConfig`

### Step 5 — Auth handlers (coder)
**Files:** `internal/web/auth.go`

1. Update `loginData` struct (add `CSRFToken`, `Email`)
2. Update `handleLogin` to pass `CSRFToken: csrf.Token(r)` in template data
3. Implement `handleRegisterGet` / `handleRegisterPost`
4. Implement `handleLoginPost`
5. Implement `handleForgotPasswordGet` / `handleForgotPasswordPost`
6. Implement `handleResetPasswordGet` / `handleResetPasswordPost`
7. Implement `handleVerifyEmail`
8. Add per-IP rate limiter (sync.Map of *rate.Limiter); apply to registration and login POST handlers
9. Write handler unit tests in `internal/web/auth_test.go`

### Step 6 — Templates (coder)
**Files:** `internal/web/templates/login.html`, `internal/web/templates/register.html`, `internal/web/templates/forgot.html`, `internal/web/templates/reset_password.html`, `internal/web/templates/verify_email.html`

1. Update `login.html`: add email+password form above OAuth buttons; show OAuth section only when `len .Providers > 0`; include CSRF hidden input; pre-fill email on error
2. Create `register.html`: email, display name (optional), password, confirm password fields; CSRF hidden input
3. Create `forgot.html`: email field; CSRF hidden input
4. Create `reset_password.html`: hidden token, new password, confirm password; CSRF hidden input
5. Create `verify_email.html`: simple "Verifying..." page that shows success/failure flash after redirect (or render inline)

All templates use the same PicoCSS + `login-card` pattern as the existing `login.html`.

---

## 10. Trade-offs and Alternatives

### Alternative A: Keep OAuth, add email/password alongside it

**Pros:** Existing users unaffected; more flexibility  
**Cons:** More complexity; need to handle account-linking (same email, two providers); the request explicitly asks to disable OAuth

**Decision:** Gate OAuth behind a config flag per the requirements. Existing code is preserved.

### Alternative B: Use a third-party auth library (e.g. go-guardian, casdoor)

**Pros:** Handles edge cases automatically  
**Cons:** Opinionated; adds a heavy dependency; the current codebase is deliberately lightweight and uses chi + gorilla directly

**Decision:** Implement auth handlers directly, consistent with existing patterns.

### Alternative C: Use a transactional email HTTP API (Resend, Mailgun HTTP)

**Pros:** No SMTP config needed; works on any hosting  
**Cons:** Additional HTTP dependency; requires API key management; SMTP is universally supported

**Decision:** SMTP with stdlib. If the operator doesn't have an SMTP relay, the `NoopMailer` drops emails gracefully and a warning is logged. A Resend adapter can be added later with the same `Mailer` interface.

---

## 11. Dependencies to Add

```
go get golang.org/x/crypto
```

`golang.org/x/crypto/bcrypt` — for password hashing. This is the only new direct dependency. `x/crypto` may already be an indirect dependency (it is pulled in by some oauth2 transitive deps); making it explicit is correct.

---

## 12. Acceptance Criteria

- [ ] Email+password registration creates a user, sends a verification email, and logs the user in
- [ ] Clicking the verification link marks `email_verified = true` on the user's account
- [ ] Email+password login returns a session cookie and redirects appropriately
- [ ] Wrong password returns a generic "Invalid email or password" message (no enumeration)
- [ ] Forgot password flow sends a reset link when the email is registered; shows the same message when it is not
- [ ] Reset link expires after 1 hour; expired link shows an appropriate error
- [ ] Reset password flow logs the user in and redirects to dashboard
- [ ] OAuth login is disabled when `auth.oauth.enabled: false` (no `/auth/google` or `/auth/github` routes registered; login page shows no OAuth buttons)
- [ ] OAuth login still works when `auth.oauth.enabled: true`
- [ ] All POST endpoints are protected by gorilla/csrf (token validated)
- [ ] Registration and login POST endpoints are rate-limited (5 req/min per IP)
- [ ] Passwords are hashed with bcrypt cost 12; plaintext is never stored or logged
- [ ] Reset and verify tokens are 32-byte random hex strings; stored as single-use, expired after use
- [ ] SMTP is configured via `smtp.*` config keys; `NoopMailer` is used (with a log warning) when SMTP host is empty
- [ ] All new store methods have unit tests
- [ ] All new auth handlers have unit tests (using table-driven tests matching existing style)
- [ ] Migration 009 runs cleanly on a fresh database and on an existing database with OAuth users

---

## 13. Parallel Group Mapping for Manager

| Group | Task | Type | Dependencies |
|-------|------|------|--------------|
| 1 | Migration + model + store methods | coder | — |
| 1 | Mailer package | coder | — |
| 1 | Config extension | coder | — |
| 2 | Server wiring | coder | Group 1 |
| 3 | Auth handlers | coder | Group 2 |
| 3 | Templates | coder | Group 2 |
| 4 | QA / tests pass | qa | Group 3 |
| 4 | Code review | code-reviewer | Group 3 |
