# TODO

Stakeholder-approved work ready for worker agents. Items here have been
researched, planned, and approved — they are ready to implement.

Worker agents (coder, designer, automation, qa, code-reviewer) pick up
`[ ]` items from this file.

---

## modern-design — Update design look to be more modern

**Plan:** `.workflow/plans/modern-design.md`
**Design spec:** `.workflow/plans/modern-design-spec.md`
**Tasks:**
- [ ] `modern-design-1-css-static` (Group 1) — Create app.css + /static/* route
- [ ] `modern-design-2-layout-login` (Group 2, parallel) — Update layout.html + login.html
- [ ] `modern-design-3-content-templates` (Group 2, parallel) — Update remaining templates

## resume-export — Markdown, DOCX, and optional PDF downloads

**Plan:** `.workflow/plans/resume-export.md`
**Tasks:**
- [ ] `resume-export-1-foundation` (Group 1) — Migration, models, generator interface, optional PDF
- [ ] `resume-export-2-exporter` (Group 2) — internal/exporter DOCX package
- [ ] `resume-export-3-routes` (Group 3) — Download routes + UI buttons
- [ ] `resume-export-4-review` (Group 4, parallel) — Code review
- [ ] `resume-export-5-qa` (Group 4, parallel) — QA

## oauth-google — Google OAuth (ALREADY IMPLEMENTED)

**Note:** Architect confirmed Google OAuth is fully implemented in `internal/web/auth.go`.
Routes, session handling, CSRF, DB upsert, and tests all exist.
Only gap: operator setup docs in README (Google Console steps, env vars).
No implementation tasks needed — close this item.

