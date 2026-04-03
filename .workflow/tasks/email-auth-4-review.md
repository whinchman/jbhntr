# Task: email-auth-4-review

- **Type**: code-reviewer
- **Status**: done
- **Repo**: .
- **Parallel Group**: 4
- **Branch**: feature/email-auth-4-review
- **Source Item**: email-auth (email/password authentication)
- **Dependencies**: email-auth-3-handlers, email-auth-3-templates

## Description

Review all code changes introduced by the email-auth feature across all branches. Focus on security correctness (auth is high-stakes), code quality, and adherence to project conventions.

Review these branches/changes:
- `feature/email-auth-1-migration-models` — migration SQL, model fields, store methods
- `feature/email-auth-1-mailer` — mailer package
- `feature/email-auth-1-config` — config structs
- `feature/email-auth-2-server-wiring` — server changes, route registration
- `feature/email-auth-3-handlers` — auth handlers and handler tests
- `feature/email-auth-3-templates` — HTML templates and CSS

## Acceptance Criteria

- [ ] All critical and warning findings logged to `.workflow/BUGS.md`
- [ ] Verdict recorded in this task file's Notes: `approve` or `request-changes`
- [ ] Security checklist reviewed (see Context)
- [ ] Code standards compliance checked

## Interface Contracts

N/A — this task reads code, produces findings.

## Context

### Security checklist (mandatory review points)

1. **bcrypt cost** — verify cost is 12, not lower: `bcrypt.GenerateFromPassword([]byte(password), 12)`
2. **Token entropy** — verify tokens are `crypto/rand` 32 bytes hex-encoded (64-char string), NOT `math/rand`
3. **Token expiry** — verify email verify token = 24h, reset token = 1h
4. **ConsumeResetToken / ConsumeVerifyToken** — verify they both clear the token in the same query (not two separate queries), preventing race conditions
5. **Timing attack mitigation** — verify `time.Sleep(200ms)` on the "user not found" path of `handleLoginPost`
6. **Email enumeration** — verify `/forgot-password` POST always redirects with the same flash message regardless of whether the email exists
7. **Generic error messages** — verify `/login` POST uses "Invalid email or password." for both "not found" and "wrong password"
8. **CSRF tokens** — verify ALL new POST handlers have the gorilla/csrf token in the form AND that the middleware is applied (should be inherited from existing setup)
9. **Rate limiting** — verify per-IP rate limiter applied to POST `/register`, `/login`, `/forgot-password`, `/reset-password` with 5 req/min limit
10. **Password never logged** — search for any `slog.` / `log.` calls in auth handlers; ensure password field is never in a log statement
11. **SQL injection** — verify all store methods use parameterized queries ($1, $2...), no string concatenation
12. **Partial unique index** — verify `009_add_email_auth.sql` has `WHERE email != ''` in the unique index
13. **OAuth gate** — verify `cfg.Auth.OAuth.Enabled` check wraps `oauthProviders` initialization, not just route registration
14. **NoopMailer on SMTP not configured** — verify `main.go` falls back to `NoopMailer` with a `slog.Warn` when `cfg.SMTP.Host == ""`

### Code quality checklist

- Handler tests cover error paths, not just happy path
- `ConsumeResetToken` signature: must accept the new password hash as a parameter (single UPDATE ... RETURNING, not two round trips)
- `GetUserByEmail` returns `nil, nil` (not an error) when no row found
- `registerData.Form` struct populated correctly on re-render (never re-populate password fields)
- Templates: `aria-invalid` set on fields with errors; `role="alert"` on flash divs
- CSS: new rules added under `/* Section 12 */` comment, not scattered
- Email templates: fully inline CSS, no external stylesheets

### Findings format for `.workflow/BUGS.md`

```
## [email-auth] <severity>: <one-line description>
- File: <path>
- Line: <approx>
- Severity: critical | warning | info
- Description: <detail>
- Reproduction: <steps or N/A>
```

## Notes

### Review verdict: APPROVE (with warnings)

**Findings summary: 0 critical, 2 warning, 2 info**

---

### Security checklist results

1. **bcrypt cost** — PASS. `bcrypt.GenerateFromPassword([]byte(password), 12)` used in both `handleRegisterPost` (line 133) and `handleResetPasswordPost` (line 363). Test fixture uses cost 4 (acceptable for tests).

2. **Token entropy** — PASS. `generateToken()` uses `crypto/rand.Read(b)` with 32 bytes hex-encoded to 64 chars. No `math/rand` usage anywhere in auth code.

3. **Token expiry** — PASS. Email verify token: `24 * time.Hour` (auth_email.go line 147). Reset token: `1 * time.Hour` (auth_email.go line 269). Both correctly enforced in DB queries via `reset_expires_at > NOW()` and `email_verify_expires_at > NOW()`.

4. **ConsumeResetToken / ConsumeVerifyToken race condition** — PASS. Both use a single atomic `UPDATE ... WHERE token = $1 AND expires_at > NOW() RETURNING ...` query. No two-step check-then-update. (user.go lines 166–168, 196–198.)

5. **Timing attack mitigation** — PASS. `handleLoginPost` sleeps 200ms on both "user not found" (line 204) and "OAuth-only account" (line 213) paths before returning, equalising timing with the bcrypt compare path.

6. **Email enumeration on forgot-password** — PASS. `handleForgotPasswordPost` always redirects to `/login` with the same flash message ("If that email is registered, a reset link has been sent.") regardless of whether the user exists. (auth_email.go line 291.)

7. **Generic error messages on login** — PASS. Both "not found" and "wrong password" cases return "Invalid email or password." (auth_email.go lines 205, 219.)

8. **CSRF tokens** — PASS. All new POST handlers (`/register`, `/login`, `/forgot-password`, `/reset-password`) include `<input type="hidden" name="gorilla.csrf.Token" value="{{.CSRFToken}}">`. CSRF middleware applied globally via `r.Use(csrfMiddleware)` when `s.sessionStore != nil` (server.go lines 237–244).

9. **Rate limiting** — PASS. `s.rateLimit()` called at the top of `handleRegisterPost`, `handleLoginPost`, `handleForgotPasswordPost`, and `handleResetPasswordPost`. Limiter is 5 req/min per IP using `golang.org/x/time/rate`. (auth_email.go lines 87, 188, 251, 339.)

10. **Password never logged** — PASS. Reviewed all `slog.*` calls in auth_email.go. None reference `password`, `hash`, or credential fields. Only errors, user IDs, and error messages are logged.

11. **SQL injection** — PASS. Every query in user.go uses positional parameters (`$1`, `$2`, etc.). No string concatenation in query construction.

12. **Partial unique index** — PASS. Migration `009_add_email_auth.sql` (line 18–21) creates `CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email_unique ON users (email) WHERE email != ''`. The `WHERE email != ''` predicate is present.

13. **OAuth gate** — PASS. `cfg.Auth.OAuth.Enabled` check in `NewServerWithConfig` wraps the `srv.oauthProviders = oauthProviders(...)` assignment (server.go line 193). Routes are registered conditionally when `s.sessionStore != nil`. OAuth providers are only populated when both the session is configured AND the OAuth flag is enabled.

14. **NoopMailer on SMTP not configured** — PASS. `main.go` (lines 102–107) checks `cfg.SMTP.Host != ""` and falls back to `&mailer.NoopMailer{}` with `slog.Warn("SMTP not configured — emails will be dropped (NoopMailer)")`.

---

### Code quality checklist results

- **Handler tests cover error paths** — PASS. `TestHandleRegisterPost` covers 5 error cases + success. `TestHandleLoginPost` covers unknown email, wrong password, and two success paths. `TestHandleForgotPasswordPost` covers unknown email, OAuth-only user, and known user. `TestHandleResetPasswordPost` covers mismatched passwords, short password, expired token, and valid token. `TestHandleVerifyEmail` covers invalid and valid tokens.

- **ConsumeResetToken signature** — PASS. Accepts `newPasswordHash string` as a parameter and performs a single `UPDATE ... RETURNING` (user.go line 166). No two round trips.

- **GetUserByEmail returns nil, nil on not-found** — PASS. user.go line 115–117 returns `nil, nil` when `sql.ErrNoRows`.

- **registerData.Form never re-populates password fields** — PASS. `renderErr` closure in `handleRegisterPost` (lines 101–112) only sets `data.Form.DisplayName` and `data.Form.Email`. Password fields are never re-populated.

- **Templates: aria-invalid on error fields** — PASS. `login.html` sets `aria-invalid="true"` on email and password inputs when `.Flash` is set (lines 33, 43). `register.html` does not use `aria-invalid` — minor accessibility gap but not a security issue.

- **Templates: role="alert" on flash divs** — PASS. All flash divs use `role="alert"` (login.html line 19, register.html line 19, forgot_password.html line 19, reset_password.html line 19, verify_email.html lines 19 and 22).

- **CSS new rules under Section 12** — PASS. New login-page rules are in Section 12 (lines 627–708) and Section 12 additions (lines 801–874). See BUG-019 for consolidation suggestion.

- **Email templates: fully inline CSS** — PASS. Both `templates/email/verify_email.html` and `templates/email/reset_password.html` use only inline `style=` attributes. No external stylesheet links.

---

### Findings detail

#### [WARNING] internal/mailer/mailer.go:88 — HTML email sent with Content-Type: text/plain (BUG-016)

`SMTPMailer` declares `Content-Type: text/plain` but sends HTML email body. Email clients will show raw HTML source to users. Logged as BUG-016. Fix: change to `text/html`.

#### [WARNING] internal/store/user.go:449 — isUniqueViolation uses fragile string matching (BUG-017)

Falls back to `strings.Contains(err.Error(), "unique")` which could match unrelated errors. Should use `pgconn.PgError` type assertion. Logged as BUG-017.

#### [INFO] internal/store/migrate_test.go — migration 009 not in test expected list (BUG-018)

When `TEST_DATABASE_URL` is set, `TestMigrate` will fail because 009 is not in its expected slice. Logged as BUG-018.

#### [INFO] internal/web/templates/static/app.css — duplicate Section 12 blocks (BUG-019)

Two "Section 12" blocks define login-page CSS; `.login-card-footer` appears in both. Cosmetic/maintainability issue. Logged as BUG-019.

---

**Verdict: approve**

No critical security issues found. All 14 security checklist items pass. Two warnings (HTML email content-type mismatch, fragile unique violation detection) are correctness issues that should be fixed but do not block the feature. Two info items are test/style quality issues.
