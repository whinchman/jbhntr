# Task: oauth-multi-user

**Type:** architect
**Status:** done
**Priority:** 1
**Epic:** oauth-multi-user
**Depends On:** none

## Description

Research and produce a detailed implementation plan for adding OAuth 2.0 authentication and multi-user support to JobHuntr. The app is currently single-user. This feature should allow multiple users to authenticate via OAuth providers (e.g., Google, GitHub), maintain separate sessions, and have fully isolated data (job searches, profiles, resumes, cover letters).

Key areas to address in the plan:

- **OAuth provider selection** — which providers, which Go library (e.g., golang.org/x/oauth2, markbates/goth)
- **User model** — schema additions to the SQLite store for users, sessions, and linking existing models to a user ID
- **Session management** — cookie-based sessions, CSRF protection, session store
- **Per-user data isolation** — how to scope all queries to the authenticated user
- **Route protection** — middleware to enforce auth on all API/web routes, login/logout/callback flows
- **Migration strategy** — how to migrate existing single-user data (if any) to the multi-user schema
- **Web UI changes** — login page, user-aware navigation, sign-out

The plan should go in `plans/oauth-multi-user.md` with enough detail for coder agents to implement each piece independently.

## Acceptance Criteria

- [ ] Implementation plan produced at `plans/oauth-multi-user.md`
- [ ] Plan covers all key areas listed above
- [ ] Plan includes a task breakdown with clear boundaries per task
- [ ] Plan references specific files/packages in the existing codebase that will be affected

## Context

- Existing packages: `cmd/jobhuntr`, `internal/config`, `internal/models`, `internal/store`, `internal/scraper`, `internal/generator`, `internal/notifier`, `internal/web`, `internal/pdf`
- Web stack: Chi router + HTMX + Pico CSS, Go html/template with //go:embed
- Database: modernc.org/sqlite (pure Go, no CGO)
- See `agent.yaml` for full code standards

## Notes

Plan produced at `plans/oauth-multi-user.md`. Covers all key areas: OAuth
provider selection (Google + GitHub via golang.org/x/oauth2), user model/schema,
session management (gorilla/sessions), per-user data isolation, route protection
middleware, migration strategy, and web UI changes. Includes 5-task breakdown.
