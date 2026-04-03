# Architecture Plan: Local Debug Deployment

**Feature**: Setup local debug deployment for testing
**Date**: 2026-04-03
**Status**: Draft

---

## 1. What Already Exists

### What Works Well

| Asset | Location | Notes |
|-------|----------|-------|
| `docker-compose.yml` | `/workspace/docker-compose.yml` | Defines `db` (postgres:16-alpine) and `app` services. The `app` service builds from `Dockerfile`, uses `env_file: .env`, and forwards port 8080. The `db` service has a health check. |
| `run.sh` | `/workspace/run.sh` | Builds binary, bootstraps `config.yaml` from example, warns on missing env vars, then runs. Does `.env` sourcing. Already handles the "binary stale" problem by removing and rebuilding. |
| `config.yaml.example` | `/workspace/config.yaml.example` | Comprehensive template with every env var referenced via `${VAR}` placeholders. |
| `.env` | `/workspace/.env` | Gitignored — developer has real secrets here already. |
| `config.Load()` | `internal/config/config.go` | Reads `.env` from the config file's directory before expanding placeholders, so a plain `./config.yaml` + `.env` in the same directory is natively supported. |
| `Dockerfile` | `/workspace/Dockerfile` | Multi-stage build, installs `chromium` for go-rod PDF, runs as non-root `appuser`. |
| PDF converter | `internal/pdf/pdf.go` | Uses go-rod headless Chromium. **Gracefully degrades**: `main.go` logs a warning if `NewRodConverter()` fails and continues with `pdfConverter=nil` — the worker skips PDF generation. This means Chromium is optional for local dev. |

### Gaps / Pain Points

1. **`docker-compose.yml` has no dev target.** The `app` service builds and runs the production image — a full multi-stage Docker build — every time. No hot-reload, no source mount, rebuild requires `docker compose build`.
2. **No `Makefile`.** There is no single-command entry point beyond `./run.sh`. Developers need to remember `docker compose up`, `./run.sh`, or manual `go build`.
3. **No `.env.example` in root.** `config.yaml.example` exists and covers all vars, but there is no standalone `.env.example` that developers can `cp .env.example .env` and fill in.
4. **`run.sh` always rebuilds.** It removes the existing binary and rebuilds — correct, but means every invocation does a full compile. For hot-reload, we want something that watches for file changes.
5. **`docker-compose.yml` app service doesn't mount source code.** The current `app` container runs a pre-built binary with no way to iterate on code.
6. **No explicit `DATABASE_URL` fallback for local non-Docker runs.** The config loads `.env` automatically, but `config.yaml` must still have `database.url: ${DATABASE_URL}`. If `DATABASE_URL` is unset in `.env`, the app exits hard.
7. **Chromium dependency.** In the Dockerfile Chromium is installed system-wide. For local runs (`./run.sh` or `go run`), go-rod auto-downloads a compatible browser binary to `~/.cache/rod`. This works, but the first run may be slow and requires internet access. The graceful degradation in `main.go` ensures it's not blocking.
8. **`run.sh` port mismatch.** `config.yaml` has `port: 44566` (developer's personal config) but `docker-compose.yml` maps `8080:8080`. A `.env.example` and example dev config should standardise on 8080.
9. **`agent.yaml` testing command is `npm test`** — leftover default. Go tests are run with `go test ./...`.

---

## 2. Requirements

A frictionless local dev/debug experience means:

- **One command** to start the full stack (DB + app).
- **Fast iteration**: code changes reflected without a full Docker rebuild.
- **Sensible defaults**: dev config and `.env.example` that work out-of-the-box for the web UI (OAuth, session), allowing API keys to remain optional for UI-only testing.
- **DB auto-migration**: schema and migrations run on startup (already implemented in `store.Open()`).
- **PDF optional**: already gracefully degraded — no action needed.
- **Works without Docker** as well: `./run.sh` for developers who want a native Go process against a local or Docker-managed Postgres.

---

## 3. Recommended Approach

### Option A: docker-compose `dev` service + `air` hot-reload (Recommended)

Add a `dev` profile to `docker-compose.yml` that:
- Mounts source code into the container.
- Uses a lightweight `golang:1.25-bookworm` builder image (not the multi-stage production image).
- Runs `air` for hot-reload inside the container.
- Shares the same `db` service.

Also add a `Makefile` with convenience targets (`make dev`, `make run`, `make test`, `make build`).

**Pros:**
- Fully self-contained: developer only needs Docker installed.
- Hot-reload: `air` watches `.go` files and rebuilds/restarts the binary automatically.
- Same OS/packages as production (Debian bookworm), Chromium available inside the dev container.
- No Go toolchain required on the host.

**Cons:**
- Requires Docker.
- First-time `docker compose pull` + image build takes a moment.
- Air must be installed inside the dev container (done via `go install` in the `dev` image).

### Option B: Native `./run.sh` + external Postgres (Alternative)

Enhance `run.sh` to also start a Postgres container if one is not already running, then run the Go binary natively.

**Pros:**
- Simpler toolchain: just Go + Docker CLI.
- Native binaries, easiest to attach a debugger (dlv).

**Cons:**
- Script complexity: managing a sidecar Docker container from a shell script is fragile.
- No hot-reload (run.sh already does a full rebuild each time).
- Developer must have Go installed.

### Option C: `air` only (no Docker for app)

Install `air` on the dev machine and run `air` with a `.air.toml` config. DB is managed by `docker compose up db`.

**Pros:**
- Minimal — works with existing `docker-compose.yml`.
- Native debugger attachment is easy.

**Cons:**
- Developer must install `air` globally (`go install github.com/air-verse/air@latest`).
- Cross-platform consistency is lower.

### Decision: Option A + Option C fallback

Implement both paths:

1. **Primary**: Add a `dev` profile service to `docker-compose.yml` using `air` inside the container. One command: `docker compose --profile dev up`.
2. **Secondary / native**: Add a `.air.toml` so developers who install `air` locally can run `air` against `docker compose up db`. Document in `make dev-native`.
3. **Makefile** wraps both paths and provides `make test`, `make build`, `make run`.
4. **`.env.example`** at repo root.
5. **Updated `config.yaml.example`** to serve as the dev default (port 8080, sane defaults).

---

## 4. All Env Vars and Dev Defaults

| Variable | Purpose | Dev Default | Required? |
|----------|---------|-------------|-----------|
| `DATABASE_URL` | PostgreSQL DSN | `postgres://jobhuntr:secret@localhost:5432/jobhuntr?sslmode=disable` | **Yes** (app exits without it) |
| `SESSION_SECRET` | Gorilla sessions 32-byte key | `dev-session-secret-change-in-prod` (32 chars) | **Yes** (CSRF + sessions) |
| `ANTHROPIC_API_KEY` | Claude API for resume generation | *(empty — PDF/resume generation skipped gracefully)* | No |
| `SERPAPI_KEY` | Job scraper | *(empty — scraper will log errors but app stays up)* | No |
| `GITHUB_CLIENT_ID` | GitHub OAuth | *(empty — login redirects fail, but app boots)* | No |
| `GITHUB_CLIENT_SECRET` | GitHub OAuth | *(empty)* | No |
| `GOOGLE_CLIENT_ID` | Google OAuth | *(empty)* | No |
| `GOOGLE_CLIENT_SECRET` | Google OAuth | *(empty)* | No |

For the local DB (managed by docker-compose `db` service):
- Host: `localhost:5432` (when accessed from host) or `db:5432` (from within compose network)
- User: `jobhuntr`
- Password: `secret`
- Database: `jobhuntr`

**Note on OAuth**: without client ID/secret the OAuth flows will 400/500. For UI-only testing without login, a future task could add a dev bypass. For now, the README notes that GitHub OAuth credentials are free to create and easiest to set up.

---

## 5. Step-by-Step Implementation Breakdown

### Step 1: Create `.env.example`

**File**: `/workspace/.env.example`

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

### Step 2: Create `.air.toml`

**File**: `/workspace/.air.toml`

Configure air to:
- Watch `cmd/` and `internal/` directories for `.go` changes.
- Build to `./tmp/air-main`.
- Run `./tmp/air-main --config config.yaml`.
- Exclude `output/`, `worktrees/`, `.workflow/`, `*.db`.

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

### Step 3: Update `docker-compose.yml` — add `dev` profile

Add a `dev` service (profile: dev) that:
- Starts from `golang:1.25-bookworm` (has Go toolchain + apt for Chromium).
- Installs `air` via `go install` in the image entrypoint or as a custom image.
- Mounts the repo source at `/workspace`.
- Sets `DATABASE_URL` pointing to the `db` service.
- Reads `.env` via `env_file`.
- Exposes port 8080.
- Depends on `db`.

Because `go install air` on every container start is slow, use a custom lightweight Dockerfile for dev: `Dockerfile.dev`.

**File**: `/workspace/Dockerfile.dev`

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

**Updated `docker-compose.yml`**:

Add `dev` service under `services`, with `profiles: [dev]`, keeping the existing `app` and `db` services unchanged.

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

volumes:
  pgdata:
  go-mod-cache:
```

The `go-mod-cache` volume speeds up rebuilds by persisting the module cache across container restarts.

### Step 4: Create `Makefile`

**File**: `/workspace/Makefile`

```makefile
.PHONY: dev dev-native build run test lint clean

# Primary dev target: hot-reload inside Docker (requires Docker)
dev:
	docker compose --profile dev up --build

# Stop and remove dev containers
dev-down:
	docker compose --profile dev down

# Start only the database (for native/host development)
db-up:
	docker compose up db -d

# Native hot-reload: requires `go install github.com/air-verse/air@latest`
dev-native: db-up
	air

# Build the binary to ./bin/jobhuntr
build:
	go build -o bin/jobhuntr ./cmd/jobhuntr

# Run the app natively (builds first)
run: build
	./run.sh

# Run all tests
test:
	go test ./...

# Run tests with race detector
test-race:
	go test -race ./...

# Clean build artifacts
clean:
	rm -rf bin/ tmp/ output/*.html output/*.pdf output/*.docx
```

### Step 5: Update `run.sh` (minor)

The existing `run.sh` is good. One small improvement: if `DATABASE_URL` is not set, print a clear message pointing to `.env.example`.

Change the warning block to also check `DATABASE_URL` and `SESSION_SECRET`:

```bash
missing=()
[ -z "${DATABASE_URL:-}" ]        && missing+=("DATABASE_URL")
[ -z "${SESSION_SECRET:-}" ]      && missing+=("SESSION_SECRET")
[ -z "${ANTHROPIC_API_KEY:-}" ]   && missing+=("ANTHROPIC_API_KEY (optional)")
[ -z "${SERPAPI_KEY:-}" ]         && missing+=("SERPAPI_KEY (optional)")
```

### Step 6: Add `tmp/` to `.gitignore`

Air writes to `./tmp/`. Add `tmp/` to `.gitignore`.

### Step 7: Update `agent.yaml` test command

Change `testing.command` from `npm test` to `go test ./...`.

---

## 6. Files to Create / Modify

| File | Action | Agent |
|------|--------|-------|
| `/workspace/.env.example` | Create | coder |
| `/workspace/.air.toml` | Create | coder |
| `/workspace/Dockerfile.dev` | Create | coder |
| `/workspace/docker-compose.yml` | Modify — add `dev` service + `go-mod-cache` volume | coder |
| `/workspace/Makefile` | Create | coder |
| `/workspace/run.sh` | Modify — add `DATABASE_URL` and `SESSION_SECRET` to warning block | coder |
| `/workspace/.gitignore` | Modify — add `tmp/` | coder |
| `/workspace/agent.yaml` | Modify — fix `testing.command` | coder |

---

## 7. Acceptance Criteria

- [ ] `cp .env.example .env` + fill in `SESSION_SECRET` + `GITHUB_CLIENT_ID`/`GITHUB_CLIENT_SECRET` is the only setup required
- [ ] `make dev` (or `docker compose --profile dev up`) starts Postgres + app with hot-reload; saving a `.go` file triggers a rebuild and restart within ~5s
- [ ] `make db-up && air` works for developers who prefer native Go toolchain
- [ ] `make test` runs `go test ./...` successfully
- [ ] `make build` produces `bin/jobhuntr`
- [ ] App starts up without `ANTHROPIC_API_KEY` or `SERPAPI_KEY` set (optional keys)
- [ ] App exits with a clear error message if `DATABASE_URL` or `SESSION_SECRET` is missing
- [ ] `docker compose up` (without `--profile dev`) still works for the production-like `app` service
- [ ] `.env.example` is committed to the repo; `.env` remains gitignored
- [ ] `tmp/` is gitignored
- [ ] No existing tests are broken

---

## 8. Trade-offs and Risks

| Trade-off | Decision |
|-----------|----------|
| `air` inside Docker vs on host | Docker-first reduces "works on my machine" issues; `.air.toml` fallback supports native workflow |
| `Dockerfile.dev` vs modifying `Dockerfile` | Separate file keeps production image clean; dev image can be bigger without impacting prod |
| `golang:1.25-bookworm` image size (~900MB) | Acceptable for dev; not shipped to production |
| Chromium in dev image | Keeps dev/prod parity for PDF; go-rod downloads its own browser anyway if Chromium is absent, so it's a nicety |
| Hot-reload vs `go run ./cmd/jobhuntr` | `air` is the established Go hot-reload tool; `go run` doesn't support watching |
| Module path `go 1.25.0` in go.mod | Dockerfile currently uses `golang:1.22` — update `Dockerfile.dev` to match `1.25` |

---

## 9. Dependencies

- `github.com/air-verse/air` — hot-reload tool, installed at image build time inside `Dockerfile.dev`. Not a Go module dependency (dev tooling only).
- Docker and Docker Compose v2 — already referenced in existing `docker-compose.yml`.
- No new Go module dependencies required.

---

## 10. Notes / Assumptions

- The existing `config.yaml` in `.gitignore` is the developer's personal config. The plan does not overwrite it. `config.yaml.example` serves as the canonical template and is committed.
- `run.sh` assumes Go is on PATH when running natively — that is the current behaviour and is preserved.
- The `go-mod-cache` Docker volume dramatically speeds up `air` rebuilds after the first one.
- Chromium auto-download by go-rod (via `~/.cache/rod`) works in the dev container because `$HOME` is `/root` inside the container and the volume mount covers `/workspace` only. On first run go-rod may download Chromium if the system Chromium is incompatible with the version of go-rod. This is acceptable — subsequent runs are instant.
- The `deploy/jobhuntr.service` systemd unit references `-db /var/lib/jobhuntr/jobhuntr.db` (SQLite flag) — this appears to be an outdated artifact from before the PostgreSQL migration. It is out of scope for this plan but should be cleaned up in a future task.
