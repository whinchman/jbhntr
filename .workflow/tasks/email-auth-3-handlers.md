# Task: email-auth-3-handlers

- **Type**: coder
- **Status**: pending
- **Repo**: .
- **Parallel Group**: 3
- **Branch**: feature/email-auth-3-handlers
- **Source Item**: email-auth (email/password authentication)
- **Dependencies**: email-auth-2-server-wiring

## Description

Implement all email/password auth handlers in `internal/web/auth.go`. This task implements the full business logic: registration, login, forgot password, reset password, email verification, and per-IP rate limiting.

Also write comprehensive unit tests in `internal/web/auth_test.go` using table-driven tests.

## Acceptance Criteria

- [ ] `handleRegisterGet` renders `register.html` with a `registerData`
- [ ] `handleRegisterPost` validates input, hashes password with bcrypt cost 12, creates user, sends verification email, logs user in, redirects to `/onboarding`
- [ ] `handleLoginPost` authenticates with bcrypt, handles "not found" with 200ms sleep, flashes generic error, sets session, redirects correctly
- [ ] `handleForgotPasswordGet` renders `forgot_password.html`
- [ ] `handleForgotPasswordPost` generates reset token, sends email, always shows same success message (no enumeration)
- [ ] `handleResetPasswordGet` validates token server-side; renders form if valid, error state if expired/invalid
- [ ] `handleResetPasswordPost` calls `ConsumeResetToken`, hashes new password, logs user in, redirects to `/login` with success flash
- [ ] `handleVerifyEmail` calls `ConsumeVerifyToken`, sets flash, redirects to `/` on success or `/login` on failure
- [ ] Per-IP rate limiter: 5 req/min (burst 5) applied to POST `/register`, POST `/login`, POST `/forgot-password`, POST `/reset-password` — returns HTTP 429 on limit exceeded
- [ ] CSRF token is passed to all template data structs via `csrf.Token(r)`
- [ ] All tokens are 32-byte `crypto/rand` hex-encoded (64 chars)
- [ ] Verification token expiry: 24 hours; reset token expiry: 1 hour
- [ ] Duplicate email registration returns "An account with that email already exists. Try signing in."
- [ ] `go test ./internal/web/...` passes

## Interface Contracts

### Store methods consumed (from `email-auth-1-migration-models`)

```go
// Available on s.store (UserStore interface):
CreateUserWithPassword(ctx, email, displayName, passwordHash, verifyToken string, verifyExpiresAt time.Time) (*models.User, error)
GetUserByEmail(ctx, email string) (*models.User, error)
SetResetToken(ctx, userID int64, token string, expiresAt time.Time) error
ConsumeResetToken(ctx, token string) (*models.User, error)
SetEmailVerifyToken(ctx, userID int64, token string, expiresAt time.Time) error
ConsumeVerifyToken(ctx, token string) (*models.User, error)
```

### Mailer consumed (from `email-auth-1-mailer`)

```go
// Available on s.mailer (EmailSender interface):
s.mailer.SendMail(ctx, to, subject, body string) error
```

Email bodies are rendered from Go `html/template` templates loaded into `s.templates`. The email template names are `email/verify_email.html` and `email/reset_password.html`.

Template variable structs for emails:
```go
type verifyEmailTemplateData struct {
    DisplayName string
    VerifyURL   string
    Year        int
}
type resetEmailTemplateData struct {
    DisplayName string
    ResetURL    string
    Year        int
}
```

Build URLs from `r.Host` (or `cfg.Server.BaseURL` from config): `https://<baseURL>/verify-email?token=<token>` and `https://<baseURL>/reset-password?token=<token>`.

### Rate limiter field (from `email-auth-2-server-wiring`)

```go
// Available on Server struct:
s.rateLimiters sync.Map // map[string]*rate.Limiter
```

Helper to get or create a limiter per IP:
```go
func (s *Server) getLimiter(ip string) *rate.Limiter {
    v, _ := s.rateLimiters.LoadOrStore(ip, rate.NewLimiter(rate.Every(time.Minute/5), 5))
    return v.(*rate.Limiter)
}
```

Apply at the top of each rate-limited POST handler:
```go
ip, _, _ := net.SplitHostPort(r.RemoteAddr)
if !s.getLimiter(ip).Allow() {
    s.setFlash(w, r, "Too many requests. Please wait a minute and try again.")
    http.Redirect(w, r, r.URL.Path, http.StatusSeeOther)
    return
}
```

## Context

### Handler implementations

#### `handleRegisterGet`
```go
func (s *Server) handleRegisterGet(w http.ResponseWriter, r *http.Request) {
    s.renderTemplate(w, "register.html", registerData{CSRFToken: csrf.Token(r)})
}
```

#### `handleRegisterPost`
1. Rate limit check (see above)
2. Parse form: `email`, `password`, `confirm_password`, `display_name`
3. Validate:
   - Display name not empty
   - Email valid format (basic check or `mail.ParseAddress`)
   - Password >= 8 chars
   - Passwords match
   - On any validation error: re-render `register.html` with `registerData{Flash: "...", Form: {...}}`
4. `bcrypt.GenerateFromPassword([]byte(password), 12)`
5. Generate verify token: `crypto/rand` 32 bytes → `hex.EncodeToString`
6. `verifyExpiresAt = time.Now().UTC().Add(24 * time.Hour)`
7. `s.store.CreateUserWithPassword(ctx, email, displayName, string(hash), token, verifyExpiresAt)`
   - On conflict/duplicate email: re-render with flash "An account with that email already exists. Try signing in."
8. Render and send verification email via `s.mailer.SendMail`
9. `s.setSession(w, r, user)` — log them in immediately
10. `http.Redirect(w, r, "/onboarding", http.StatusSeeOther)`

#### `handleLoginPost`
1. Rate limit check
2. Parse: `email`, `password`
3. `user, err := s.store.GetUserByEmail(ctx, email)`
4. If `user == nil`: `time.Sleep(200 * time.Millisecond)`, flash "Invalid email or password.", redirect `/login`
5. `err = bcrypt.CompareHashAndPassword([]byte(*user.PasswordHash), []byte(password))`
6. If mismatch: flash "Invalid email or password.", redirect `/login`
7. `s.setSession(w, r, user)`
8. If `!user.OnboardingComplete`: redirect `/onboarding`
9. Else: `http.Redirect(w, r, s.consumeReturnTo(w, r), http.StatusSeeOther)`

#### `handleForgotPasswordGet`
```go
s.renderTemplate(w, "forgot_password.html", forgotPasswordData{CSRFToken: csrf.Token(r)})
```

#### `handleForgotPasswordPost`
1. Rate limit check
2. Parse: `email`
3. `user, _ := s.store.GetUserByEmail(ctx, email)` — always show success regardless
4. If `user != nil && user.PasswordHash != nil`:
   - Generate 32-byte hex token
   - `expiresAt = time.Now().UTC().Add(1 * time.Hour)`
   - `s.store.SetResetToken(ctx, user.ID, token, expiresAt)`
   - Send reset email via `s.mailer.SendMail`
5. `s.setFlash(w, r, "If that email is registered, a reset link has been sent.")`
6. `http.Redirect(w, r, "/login", http.StatusSeeOther)`

#### `handleResetPasswordGet`
1. `token := r.URL.Query().Get("token")`
2. Look up user by token (need a store helper — use a `GetUserByResetToken` query or inline SQL; see note below)
3. If not found or expired: render `reset_password.html` with `resetPasswordData{TokenValid: false, Flash: "Reset link is invalid or has expired."}`
4. Else: render `reset_password.html` with `resetPasswordData{TokenValid: true, Token: token, CSRFToken: csrf.Token(r)}`

**Note on token lookup for GET**: Add a store method `GetUserByResetToken(ctx, token string) (*models.User, error)` that does `SELECT ... WHERE reset_token = $1 AND reset_expires_at > NOW()`. This method does NOT consume the token (no UPDATE). Add it to the store and the `UserStore` interface. It returns `nil, nil` if not found.

#### `handleResetPasswordPost`
1. Rate limit check
2. Parse: `token`, `password`, `confirm_password`
3. Validate: passwords match, length >= 8
4. `hash, _ := bcrypt.GenerateFromPassword([]byte(password), 12)`
5. `user, _ := s.store.ConsumeResetToken(ctx, token, string(hash))`
   - Note: `ConsumeResetToken` must accept the new hash as a parameter — confirm with `email-auth-1-migration-models` task's implementation (see that task for signature)
6. If `user == nil`: flash "Reset link has expired.", redirect `/forgot-password`
7. `s.setSession(w, r, user)`
8. `s.setFlash(w, r, "Your password has been updated.")`
9. `http.Redirect(w, r, "/", http.StatusSeeOther)`

#### `handleVerifyEmail`
1. `token := r.URL.Query().Get("token")`
2. `user, _ := s.store.ConsumeVerifyToken(ctx, token)`
3. If `user == nil`: flash "Verification link is invalid or has expired.", redirect `/login`
4. If `user != nil`: flash "Your email has been verified.", redirect `/`

### Token generation helper

```go
func generateToken() (string, error) {
    b := make([]byte, 32)
    if _, err := rand.Read(b); err != nil {
        return "", err
    }
    return hex.EncodeToString(b), nil
}
```

Import `"crypto/rand"` and `"encoding/hex"`.

### Tests

Write `internal/web/auth_test.go` with table-driven tests. Follow existing style in `server_test.go` or `auth_test.go`. Key scenarios:

- `handleRegisterPost`: empty fields, mismatched passwords, short password, duplicate email, successful registration
- `handleLoginPost`: unknown email (timing check), wrong password, successful login, redirect to onboarding vs dashboard
- `handleForgotPasswordPost`: unknown email (still returns success flash), known email (token set and email sent via mock mailer)
- `handleResetPasswordPost`: expired token, passwords don't match, successful reset
- `handleVerifyEmail`: invalid token, valid token

Use a `httptest.NewRecorder` + mock store for handler tests.

## Notes

<!-- implementing agent fills this in -->
