# Task: email-auth-4-review

- **Type**: code-reviewer
- **Status**: pending
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

<!-- code-reviewer fills in verdict and findings summary -->
