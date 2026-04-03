# Done

Completed work. Features moved here after implementation, review, and merge
to the default branch.

---

## Resume/Cover Letter Export Formats

**Completed:** 2026-04-03

Added Markdown and DOCX download formats for generated resumes and cover letters. PDF generation made optional (non-fatal on failure). New `internal/exporter` package converts Markdown to DOCX using `gomutex/godocx`. Migration 008 adds `resume_markdown`/`cover_markdown` columns. Four new download routes + conditional UI buttons in `job_detail.html`. BUG-012 (underscore word-boundary in italic parser) and BUG-013 (test body length guard) logged for future fix.

Tasks: resume-export-1-foundation, resume-export-2-exporter, resume-export-3-routes (all done, merged to development)

---

## Modern Design Refresh

**Completed:** 2026-04-03

Replaced all inline styles with a single `app.css` override file on top of PicoCSS v2. Remapped Pico custom properties to a fresh token set (indigo accent, neutral off-white background, 7 status badge variants). Added `/static/*` route via Go embed. All templates cleaned of inline styles. No build tooling added.

BUG-010 (low): `.providers-section` missing margin rule in app.css — cosmetic only, logged for future fix.

Tasks: modern-design-1-css-static, modern-design-2-layout-login, modern-design-3-content-templates (all done, merged to development)

---

## Per-User NTFY Notifications

**Completed:** 2026-04-01

Replaced global `NTFY_TOPIC` env var with a per-user `ntfy_topic` field. Users set their own ntfy.sh topic in Settings → Notifications; notifications are skipped if the field is blank. `NTFY_TOPIC` removed from config, `.env.example`, and `render.yaml`.

Tasks: per-user-ntfy (done, merged to development)

---

## Deployment Epic

**Epic:** deployment
**Completed:** 2026-04-01

Full production deployment stack: SQLite → PostgreSQL migration (pgx/v5 stdlib adapter, all queries and migrations ported); multi-stage Dockerfile with Chromium runtime + non-root user; docker-compose for local dev (app + postgres:16-alpine, named volume, health check); render.yaml for one-click Render Blueprint deploy with managed Postgres; `/healthz` endpoint; README sections for Docker and Render setup.

Tasks: deploy-postgres-migration, deploy-docker, deploy-render (all done, merged to development)

---

## Full Sign-In / Sign-Up Flow

**Epic:** auth-signin-flow
**Plan:** plans/auth-signin-flow.md
**Completed:** 2026-04-01

Polished login page with flash messages and loading states; first-time onboarding screen; profile/account page; return-to redirect after OAuth; dashboard auth-awareness (hero CTA for logged-out visitors, job table for authenticated users); layout nav with Sign In / Profile links. Code review findings (BUG-005/006/007) fixed inline.

Tasks: auth-task1-model, auth-task2-login-polish, auth-task3-return-to, auth-task4-onboarding, auth-task5-profile, auth-task6-dashboard-auth (all done, merged to development)

---

## OAuth Multi-User Authentication

**Epic:** oauth-multi-user
**Plan:** plans/oauth-multi-user.md
**Branches:** feature/task1-schema-migration, feature/task2-auth-oauth, feature/task3-peruser-routes, feature/task4-multiuser-scraper, feature/task5-integration-testing
**Completed:** 2026-04-01

Added OAuth 2.0 authentication (Google + GitHub), multi-user support with fully isolated per-user data, session management with CSRF protection, per-user route scoping, multi-user scraper, and integration tests.

Tasks:
- tasks/oauth-multi-user.md — Architect plan (done)
- tasks/task1-schema-migration.md — Schema migration and users table (done, verified)
- tasks/task2-auth-oauth.md — OAuth handlers, session management, route protection (done, verified)
- task3-peruser-routes — Per-user data isolation in routes (done, verified)
- task4-multiuser-scraper — Multi-user scraper support (done, verified)
- task5-integration-testing — Integration tests and final cleanup (done, verified)

