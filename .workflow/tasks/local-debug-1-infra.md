# Task: local-debug-1-infra

- **Type**: coder
- **Status**: done
- **Repo**: . (single repo — /workspace)
- **Parallel Group**: 1
- **Branch**: feature/local-debug-1-infra
- **Source Item**: local-debug (plan: .workflow/plans/local-debug.md)
- **Dependencies**: none

## Description

Implement the complete local-debug dev infrastructure for the jobhuntr project.
This covers all 8 files identified in the plan. The goal is a frictionless
one-command dev experience (`make dev`) with hot-reload via `air`, plus a native
fallback (`make dev-native`).

Files to create:
- `.env.example`
- `.air.toml`
- `Dockerfile.dev`
- `Makefile`

Files to modify:
- `docker-compose.yml` — add `dev` service (profile-gated) and `go-mod-cache` volume
- `run.sh` — extend warning block to check `DATABASE_URL` and `SESSION_SECRET`
- `.gitignore` — add `tmp/`
- `agent.yaml` — fix `testing.command` from `npm test` to `go test ./...`

## Acceptance Criteria

- [ ] `.env.example` exists at repo root, is committed, and is NOT in `.gitignore`
- [ ] `.air.toml` exists at repo root; watches `cmd/` and `internal/`, builds to `./tmp/air-main`, excludes `tmp/`, `output/`, `worktrees/`, `.workflow/`
- [ ] `Dockerfile.dev` exists; based on `golang:1.25-bookworm`, installs `chromium` and `air`, CMD is `air`
- [ ] `docker-compose.yml` has a `dev` service with `profiles: [dev]`, builds from `Dockerfile.dev`, mounts source at `/workspace`, sets `DATABASE_URL` pointing to `db` service, depends on `db` with health check, exposes port 8080
- [ ] `docker-compose.yml` has a `go-mod-cache` named volume used by the `dev` service
- [ ] Existing `app` and `db` services in `docker-compose.yml` are unchanged
- [ ] `Makefile` exists with targets: `dev`, `dev-down`, `db-up`, `dev-native`, `build`, `run`, `test`, `test-race`, `clean`
- [ ] `make test` runs `go test ./...`
- [ ] `make build` builds to `bin/jobhuntr`
- [ ] `run.sh` warning block checks `DATABASE_URL` and `SESSION_SECRET` (in addition to whatever optional vars it already checks)
- [ ] `.gitignore` contains `tmp/`
- [ ] `agent.yaml` `testing.command` is `go test ./...`
- [ ] `go test ./...` passes (no existing tests broken)

## Interface Contracts

None — single-repo, no cross-repo contracts.

## Context

### `.env.example` content (from plan §5 Step 1)

```
# Copy this file to .env and fill in the required values.
# DATABASE_URL is managed automatically by docker compose (see below).

# Required for web UI (sessions and CSRF)
SESSION_SECRET=dev-session-secret-change-in-prod-xx

# Required for OAuth login (create a GitHub OAuth App at https://github.com/settings/developers)
GITHUB_CLIENT_ID=
GITHUB_CLIENT_SECRET=

# Optional — Google OAuth (alternative login provider)
GOOGLE_CLIENT_ID=
GOOGLE_CLIENT_SECRET=

# Optional — needed for the job scraper to find new listings
SERPAPI_KEY=

# Optional — needed for AI resume/cover letter generation
ANTHROPIC_API_KEY=

# Set automatically by docker compose; override here only for native (non-Docker) runs
# DATABASE_URL=postgres://jobhuntr:secret@localhost:5432/jobhuntr?sslmode=disable
```

### `.air.toml` content (from plan §5 Step 2)

```toml
root = "."
tmp_dir = "tmp"

[build]
cmd = "go build -o ./tmp/air-main ./cmd/jobhuntr"
bin = "./tmp/air-main"
args_bin = ["--config", "config.yaml"]
include_ext = ["go"]
include_dir = ["cmd", "internal"]
exclude_dir = ["tmp", "output", "worktrees", ".workflow", "vendor"]
delay = 500

[log]
time = true

[color]
main = "magenta"
watcher = "cyan"
build = "yellow"
runner = "green"

[misc]
clean_on_exit = true
```

### `Dockerfile.dev` content (from plan §5 Step 3)

```dockerfile
FROM golang:1.25-bookworm

# Install Chromium for go-rod PDF (same as production)
RUN apt-get update && apt-get install -y --no-install-recommends chromium && rm -rf /var/lib/apt/lists/*

# Install air for hot-reload
RUN go install github.com/air-verse/air@latest

WORKDIR /workspace

# Default: run air (mounts source at /workspace at runtime)
CMD ["air"]
```

### `docker-compose.yml` dev service addition (from plan §5 Step 3)

Add under `services:`:

```yaml
  dev:
    profiles: [dev]
    build:
      context: .
      dockerfile: Dockerfile.dev
    env_file: .env
    environment:
      DATABASE_URL: postgres://jobhuntr:secret@db:5432/jobhuntr?sslmode=disable
    volumes:
      - .:/workspace
      - go-mod-cache:/root/go/pkg/mod
    ports:
      - "8080:8080"
    depends_on:
      db:
        condition: service_healthy
    working_dir: /workspace
```

Add under `volumes:` (alongside existing `pgdata:`):

```yaml
  go-mod-cache:
```

### `Makefile` content (from plan §5 Step 4)

Targets: `dev`, `dev-down`, `db-up`, `dev-native`, `build`, `run`, `test`, `test-race`, `clean`.
Key commands:
- `dev`: `docker compose --profile dev up --build`
- `build`: `go build -o bin/jobhuntr ./cmd/jobhuntr`
- `test`: `go test ./...`
- `test-race`: `go test -race ./...`
- `clean`: `rm -rf bin/ tmp/ output/*.html output/*.pdf output/*.docx`

### `run.sh` warning block change (from plan §5 Step 5)

Read `run.sh` first. Find the existing env-var warning block. Extend it to include:

```bash
missing=()
[ -z "${DATABASE_URL:-}" ]        && missing+=("DATABASE_URL")
[ -z "${SESSION_SECRET:-}" ]      && missing+=("SESSION_SECRET")
[ -z "${ANTHROPIC_API_KEY:-}" ]   && missing+=("ANTHROPIC_API_KEY (optional)")
[ -z "${SERPAPI_KEY:-}" ]         && missing+=("SERPAPI_KEY (optional)")
```

The script already sources `.env` before this block — do not break that logic.

### Go module entry point

The main package is at `./cmd/jobhuntr` (verify by checking `go.mod` and `cmd/` directory). Use this path in `.air.toml`, `Makefile`, and `Dockerfile.dev`.

### `agent.yaml` fix

`testing.command` is currently `npm test`. Change to `go test ./...`.

## Notes

**Branch**: `feature/local-debug-1-infra`
**Commit**: `5efd53c`

All 8 files implemented in a single commit:

- `.env.example` — updated to match plan spec (DATABASE_URL commented out, SESSION_SECRET required with default dev value, GitHub OAuth required, Google/SERPAPI/ANTHROPIC optional)
- `.air.toml` — created with watches on `cmd/` and `internal/`, builds to `./tmp/air-main`, excludes `tmp/`, `output/`, `worktrees/`, `.workflow/`
- `Dockerfile.dev` — created based on `golang:1.25-bookworm`, installs chromium and air via `go install`, CMD is `air`
- `docker-compose.yml` — added `dev` service with `profiles: [dev]`, `go-mod-cache` volume, mounts source at `/workspace`, depends on `db` with health check
- `Makefile` — created with all 9 required targets; `make test` runs `go test ./...`, `make build` builds to `bin/jobhuntr`
- `run.sh` — warning block extended to check `DATABASE_URL` and `SESSION_SECRET` (required), `ANTHROPIC_API_KEY (optional)` and `SERPAPI_KEY (optional)` retained
- `.gitignore` — `tmp/` added
- `agent.yaml` — `testing.command` changed from `npm test` to `go test ./...`

**Test run**: Go toolchain not available in this container (Linux agent container, no Go SDK installed). Source changes are structurally correct. `go test ./...` should be verified on a machine with Go 1.25 installed.
