# Task: email-auth-2-server-wiring

- **Type**: coder
- **Status**: pending
- **Repo**: .
- **Parallel Group**: 2
- **Branch**: feature/email-auth-2-server-wiring
- **Source Item**: email-auth (email/password authentication)
- **Dependencies**: email-auth-1-migration-models, email-auth-1-mailer, email-auth-1-config

## Description

Wire the email-auth feature into `internal/web/server.go`: add the `EmailSender` interface, add a `mailer` field to `Server`, add a `WithMailer` option method, gate OAuth provider initialization behind `cfg.Auth.OAuth.Enabled`, register all new routes, and register all new templates. Also update `main.go` to construct the mailer and inject it into the server.

This is the integration point between Group 1 (migration, mailer, config) and Group 3 (handlers, templates). It must compile against the new `UserStore` interface methods from `email-auth-1-migration-models`.

## Acceptance Criteria

- [ ] `EmailSender` interface defined in `internal/web/server.go` with `SendMail(ctx, to, subject, body) error`
- [ ] `Server` struct has a `mailer EmailSender` field
- [ ] `WithMailer(m EmailSender) *Server` method exists and sets `s.mailer = m`
- [ ] `NewServerWithConfig` no longer panics when `cfg.Auth.OAuth.Enabled` is false (OAuth providers not populated)
- [ ] OAuth provider init is gated: `if cfg.Auth.OAuth.Enabled { srv.oauthProviders = oauthProviders(...) }`
- [ ] `Handler()` registers all 8 new routes inside the `if s.sessionStore != nil` block (see routes below)
- [ ] `Handler()` also registers the `GET /verify-email` route
- [ ] All new template files (`register.html`, `verify_email.html`, `forgot_password.html`, `reset_password.html`) are parsed/registered in `NewServerWithConfig` alongside existing templates
- [ ] `main.go` constructs mailer from `cfg.SMTP` and calls `srv.WithMailer(m)`
- [ ] Per-IP rate limiter map (`sync.Map`) field added to `Server` for use by auth handlers
- [ ] `go build ./...` succeeds

## Interface Contracts

### Routes to register (inside `if s.sessionStore != nil` in `Handler()`)

```go
r.Get("/register",          s.handleRegisterGet)
r.Post("/register",         s.handleRegisterPost)
r.Post("/login",            s.handleLoginPost)
r.Get("/forgot-password",   s.handleForgotPasswordGet)
r.Post("/forgot-password",  s.handleForgotPasswordPost)
r.Get("/reset-password",    s.handleResetPasswordGet)
r.Post("/reset-password",   s.handleResetPasswordPost)
r.Get("/verify-email",      s.handleVerifyEmail)
```

The handler methods themselves are implemented by the `email-auth-3-handlers` task and will live in `internal/web/auth.go`. For this task, stubs that return `http.StatusNotImplemented` are sufficient to make the build pass — but the signatures must match exactly:

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

**Note**: if `email-auth-3-handlers` is implementing on the same branch, use real implementations, not stubs. If on a separate branch, stubs are fine for this task's build check — the merge will replace them.

### `UserStore` interface additions (from `email-auth-1-migration-models`)

The following methods must be present in the `UserStore` interface in `server.go` for the build to succeed:

```go
CreateUserWithPassword(ctx context.Context, email, displayName, passwordHash, verifyToken string, verifyExpiresAt time.Time) (*models.User, error)
GetUserByEmail(ctx context.Context, email string) (*models.User, error)
SetResetToken(ctx context.Context, userID int64, token string, expiresAt time.Time) error
ConsumeResetToken(ctx context.Context, token string) (*models.User, error)
SetEmailVerifyToken(ctx context.Context, userID int64, token string, expiresAt time.Time) error
ConsumeVerifyToken(ctx context.Context, token string) (*models.User, error)
```

These are implemented in `internal/store/user.go` by the migration/models task.

### Template data structs

Add these structs to `internal/web/auth.go` (or a new `internal/web/auth_types.go`). They are needed by both this task (template registration) and the handlers task (Group 3):

```go
type loginData struct {
    Providers    []string  // OAuth providers (empty if OAuth disabled)
    Flash        string
    FlashSuccess string
    CSRFToken    string
    Email        string    // pre-fill email on validation error
}

type registerData struct {
    Flash     string
    CSRFToken string
    Form      struct {
        DisplayName string
        Email       string
    }
}

type forgotPasswordData struct {
    Flash     string
    CSRFToken string
    Sent      bool
    Form      struct {
        Email string
    }
}

type resetPasswordData struct {
    Flash      string
    CSRFToken  string
    Token      string
    TokenValid bool
}

type verifyEmailData struct {
    Flash      string
    FlashError string
    CSRFToken  string
    Email      string
}
```

## Context

### `EmailSender` interface

```go
// EmailSender is the interface consumed by auth handlers to send email.
// It is satisfied by *mailer.SMTPMailer and *mailer.NoopMailer.
type EmailSender interface {
    SendMail(ctx context.Context, to, subject, body string) error
}
```

Add `mailer EmailSender` to the `Server` struct and add:

```go
func (s *Server) WithMailer(m EmailSender) *Server {
    s.mailer = m
    return s
}
```

### Rate limiter field

Add to `Server` struct:

```go
rateLimiters sync.Map // map[string]*rate.Limiter — keyed by IP
```

Import `"golang.org/x/time/rate"` and `"sync"`. The rate limiter logic itself (5 req/min, burst 5) is implemented in the handlers task. This task only adds the field.

### `main.go` changes

After `NewServerWithConfig(cfg)`, construct and inject the mailer:

```go
var m web.EmailSender
if cfg.SMTP.Host != "" {
    m = mailer.NewSMTPMailer(cfg.SMTP.Host, cfg.SMTP.Port, cfg.SMTP.Username, cfg.SMTP.Password, cfg.SMTP.From)
} else {
    slog.Warn("SMTP not configured — emails will be dropped (NoopMailer)")
    m = &mailer.NoopMailer{}
}
srv.WithMailer(m)
```

### Template registration

The `NewServerWithConfig` function parses templates using `template.ParseGlob` or individual `template.ParseFiles` calls. Add the four new browser templates to the same glob/parse call:
- `internal/web/templates/register.html`
- `internal/web/templates/verify_email.html`
- `internal/web/templates/forgot_password.html`
- `internal/web/templates/reset_password.html`

Also add email templates (read as plain strings or via embed):
- `internal/web/templates/email/verify_email.html`
- `internal/web/templates/email/reset_password.html`

Check how existing templates are loaded (ParseGlob vs ParseFiles) and follow the same pattern.

### OAuth gate

Find the existing code that populates `srv.oauthProviders` (search for `oauthProviders` in `server.go` or `auth.go`) and wrap it:

```go
if cfg.Auth.OAuth.Enabled {
    srv.oauthProviders = oauthProviders(cfg.Auth, cfg.Server.BaseURL)
}
```

The existing login handler (`handleLogin` GET) already passes `Providers: providerNames(s.oauthProviders)` — this will naturally return an empty slice when OAuth is disabled, so no change needed there beyond the gate.

## Notes

<!-- implementing agent fills this in -->
