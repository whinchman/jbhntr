# TODO

Stakeholder-approved work ready for worker agents. Items here have been
researched, planned, and approved — they are ready to implement.

Worker agents (coder, designer, automation, qa, code-reviewer) pick up
`[ ]` items from this file.

---

## email-auth — Email/password authentication

**Plan:** `.workflow/plans/email-auth.md`
**Design spec:** `.workflow/plans/email-auth-design.md`
**Tasks:**
- [ ] `email-auth-1-migration-models` (Group 1, parallel) — Migration 009, models, store methods
- [ ] `email-auth-1-mailer` (Group 1, parallel) — internal/mailer package
- [ ] `email-auth-1-config` (Group 1, parallel) — SMTPConfig, OAuth enable flag
- [ ] `email-auth-2-server-wiring` (Group 2) — routes, middleware, mailer injection, main.go
- [ ] `email-auth-3-handlers` (Group 3, parallel) — auth handlers + unit tests
- [ ] `email-auth-3-templates` (Group 3, parallel) — 5 browser templates + 2 email templates + CSS
- [ ] `email-auth-4-review` (Group 4, parallel) — security-focused code review
- [ ] `email-auth-4-qa` (Group 4, parallel) — integration tests + full test suite

## oauth-google — Google OAuth (ALREADY IMPLEMENTED)

**Note:** Architect confirmed Google OAuth is fully implemented in `internal/web/auth.go`.
Routes, session handling, CSRF, DB upsert, and tests all exist.
Only gap: operator setup docs in README (Google Console steps, env vars).
No implementation tasks needed — close this item.

