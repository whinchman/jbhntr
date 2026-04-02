# Task: deploy-render

**Type:** coder
**Status:** done
**Priority:** 3
**Epic:** deployment
**Depends On:** deploy-docker

## Description

Add Render.com deployment configuration and a `/healthz` endpoint. Render will
watch the repo and auto-redeploy on push. The app will use Render's managed
Postgres (no persistent disk needed).

## Acceptance Criteria

- [ ] `GET /healthz` endpoint added to the web layer; returns `200 OK` with body `{"status":"ok"}` — no auth required, no DB query needed
- [ ] `render.yaml` at project root defines:
  - A `web` service (type: web, runtime: docker, uses the Dockerfile)
  - A `postgres` service (managed Postgres, free tier or standard)
  - `DATABASE_URL` env var on the web service wired to the postgres service's connection string
  - `healthCheckPath: /healthz`
  - All other required env vars listed as `sync: false` (user must set them in Render dashboard)
- [ ] `render.yaml` includes a comment block listing all required env vars and where to obtain them (ANTHROPIC_API_KEY, SERPAPI_KEY, NTFY_TOPIC, SESSION_SECRET, GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET, GITHUB_CLIENT_ID, GITHUB_CLIENT_SECRET)
- [ ] README has a "Deploy to Render" section with: one-time setup steps, link to render.yaml docs, note about setting env vars in the dashboard

## Context

- The web router lives in `internal/web/` — add the `/healthz` route there
- Render's managed Postgres injects `DATABASE_URL` automatically when services are linked in `render.yaml`
- The Dockerfile from `deploy-docker` is what Render will build; `render.yaml` should reference it
- `base_url` in `config.yaml` will need to be set to the Render service URL for OAuth callbacks — document this in the README section

## Notes

Implementation complete on branch `feature/deploy-render` (commit bb2a411), based on `feature/deploy-docker`.

Changes made:
- `internal/web/server.go`: Added `GET /healthz` route and `handleHealthz` handler — returns 200 `{"status":"ok"}`, no auth middleware, no DB access.
- `render.yaml`: Created at project root. Defines a `web` service (type: web, runtime: docker, uses `./Dockerfile`), a `jobhuntr-db` managed Postgres instance, `DATABASE_URL` wired via `fromDatabase.connectionString`, `healthCheckPath: /healthz`, and all eight other required env vars (`ANTHROPIC_API_KEY`, `SERPAPI_KEY`, `NTFY_TOPIC`, `SESSION_SECRET`, `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, `GITHUB_CLIENT_ID`, `GITHUB_CLIENT_SECRET`) marked `sync: false` with sourcing instructions in the header comment block.
- `README.md`: Added "Deploy to Render" section with one-time setup steps, env vars table, `base_url` OAuth callback note, and link to the Blueprint spec docs.
