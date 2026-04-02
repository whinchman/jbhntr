# Task: deploy-docker

**Type:** coder
**Status:** done
**Priority:** 2
**Epic:** deployment
**Depends On:** deploy-postgres-migration

## Description

Containerize the jobhuntr application with a multi-stage Dockerfile and a
`docker-compose.yml` that brings up the app + Postgres together. This should
be the standard local dev environment going forward (replaces running the
binary directly).

## Acceptance Criteria

- [ ] `Dockerfile` at project root uses multi-stage build: Go builder stage → minimal runtime image (e.g. `gcr.io/distroless/static` or `debian:bookworm-slim`)
- [ ] Final image runs the compiled binary as a non-root user
- [ ] `docker-compose.yml` defines two services: `db` (postgres:16-alpine) and `app` (built from Dockerfile)
- [ ] `docker-compose.yml` mounts a named volume for Postgres data persistence
- [ ] `docker-compose.yml` mounts `./output` into the container at `/app/output`
- [ ] `docker-compose.yml` mounts `./resume.md` into the container at `/app/resume.md` (read-only)
- [ ] `docker-compose.yml` reads env vars from `.env` file for the app service
- [ ] App service depends_on db with a health check so it waits for Postgres to be ready
- [ ] `.dockerignore` excludes: `.git`, `worktrees/`, `*.db`, `*.db-shm`, `*.db-wal`, `output/`, `.env`, `bin/`
- [ ] `docker compose up` starts the full stack and the app is reachable at `http://localhost:8080`
- [ ] README updated with a "Running with Docker" section (brief — compose up command and env setup)

## Context

- The app binary is built via `go build ./cmd/...` (check `cmd/` for the main package name)
- Config is loaded from `config.yaml` + env vars; `DATABASE_URL` should be set in the compose env to point at the `db` service
- `output/` dir is where generated PDFs land — needs to be writable and ideally persisted on the host
- `resume.md` is the fallback resume path referenced in `config.yaml`
- Postgres service should use `POSTGRES_DB`, `POSTGRES_USER`, `POSTGRES_PASSWORD` env vars consistent with what the app's `DATABASE_URL` expects

## Notes

Implemented on branch `feature/deploy-docker` (worktree at `/workspace/worktrees/deploy-docker`).
Branch is based on `feature/deploy-postgres-migration` so all Postgres changes are included.

Files created:
- `Dockerfile`: multi-stage (golang:1.22-bookworm builder → debian:bookworm-slim runtime); installs chromium for go-rod PDF generation; runs as non-root user `appuser` (uid 1001).
- `docker-compose.yml`: services `db` (postgres:16-alpine with named volume `pgdata`) and `app` (built from Dockerfile); mounts `./output:/app/output`, `./resume.md:/app/resume.md:ro`, `./config.yaml:/app/config.yaml:ro`; reads `.env`; `DATABASE_URL` overridden to point at `db` service; `depends_on` with `pg_isready` health check.
- `.dockerignore`: excludes `.git`, `worktrees/`, `*.db`, `*.db-shm`, `*.db-wal`, `output/`, `.env`, `bin/`.
- `README.md`: added "Running with Docker" section with setup and `docker compose up --build` instructions.

All acceptance criteria met. Commit: c569388.
