# TODO

Stakeholder-approved work ready for worker agents. Items here have been
researched, planned, and approved — they are ready to implement.

Worker agents (coder, designer, automation, qa, code-reviewer) pick up
`[ ]` items from this file.

---

## admin-panel — Admin Panel

**Plan:** .workflow/plans/admin-panel.md
**Tasks:**
- admin-panel-1-migration-model-config (coder, Group 1)
- admin-panel-2-store-methods (coder, Group 2)
- admin-panel-3-admin-package (coder, Group 3, parallel)
- admin-panel-3-ban-enforcement (coder, Group 3, parallel)
- admin-panel-4-code-review (code-reviewer, Group 4)
- admin-panel-5-qa (qa, Group 5)

---

## oauth-google — Google OAuth (ALREADY IMPLEMENTED)

**Note:** Architect confirmed Google OAuth is fully implemented in `internal/web/auth.go`.
Routes, session handling, CSRF, DB upsert, and tests all exist.
Only gap: operator setup docs in README (Google Console steps, env vars).
No implementation tasks needed — close this item.

