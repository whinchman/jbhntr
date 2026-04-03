# TODO

Stakeholder-approved work ready for worker agents. Items here have been
researched, planned, and approved — they are ready to implement.

Worker agents (coder, designer, automation, qa, code-reviewer) pick up
`[ ]` items from this file.

---

## local-debug — Local debug deployment for testing

**Plan:** `.workflow/plans/local-debug.md`
**Tasks:**
- [ ] `local-debug-1-infra` (Group 1) — .env.example, .air.toml, Dockerfile.dev, Makefile, docker-compose dev service, run.sh, .gitignore, agent.yaml
- [ ] `local-debug-2-review` (Group 2) — Code review
- [ ] `local-debug-3-qa` (Group 3) — QA

## oauth-google — Google OAuth (ALREADY IMPLEMENTED)

**Note:** Architect confirmed Google OAuth is fully implemented in `internal/web/auth.go`.
Routes, session handling, CSRF, DB upsert, and tests all exist.
Only gap: operator setup docs in README (Google Console steps, env vars).
No implementation tasks needed — close this item.

