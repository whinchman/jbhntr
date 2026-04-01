# Done

Completed work. Features moved here after implementation, review, and merge
to the default branch.

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

