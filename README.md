# JobHuntr

JobHuntr is a multi-user Go web application that automates job searching,
delivers push notifications, and generates tailored resumes and cover letters
using the Claude API.

```
Scheduler (hourly) → SerpAPI Google Jobs → PostgreSQL
                                              ↓ new jobs
                                         ntfy.sh → Phone notification (per-user topic)
                                              ↓ user opens link
                                         Web Dashboard (approve/reject)
                                              ↓ approved
                                    Claude API → Resume + Cover Letter (HTML + Markdown)
                                              ↓
                                    go-rod (headless Chromium) → PDF (optional)
                                              ↓
                                    Web Dashboard (view + download MD / DOCX / PDF)
```

Users sign in via Google or GitHub OAuth. Each user has their own job feed,
search filters, resume, and notification topic — all configured in the Settings page.

## Prerequisites

| Requirement | Notes |
|-------------|-------|
| Go 1.25+ | [go.dev/dl](https://go.dev/dl) |
| Docker + Docker Compose | For the database and dev stack |
| GitHub OAuth App | Required for login — [create one](https://github.com/settings/developers) |
| Google OAuth App | Optional second login provider — [Google Cloud Console](https://console.cloud.google.com) |
| SerpAPI account | Optional — needed for job scraping ([serpapi.com](https://serpapi.com)) |
| Anthropic API key | Optional — needed for resume generation ([console.anthropic.com](https://console.anthropic.com)) |
| Chromium / Chrome | Optional — needed for PDF generation; skipped gracefully if absent |

Chromium must be on `$PATH` as `chromium`, `chromium-browser`, or `google-chrome`.
go-rod will attempt to download it automatically if none is found. PDF generation
is non-fatal — the app runs normally without it.

## Local Development

### Quick start (Docker + hot-reload)

```bash
git clone https://github.com/whinchman/jobhuntr
cd jobhuntr

cp .env.example .env          # fill in at minimum SESSION_SECRET and GITHUB_CLIENT_ID/SECRET
cp config.yaml.example config.yaml

make dev                      # starts postgres + app with air hot-reload
```

The app will be available at `http://localhost:8080`. Source changes in `cmd/`
or `internal/` trigger an automatic rebuild via [air](https://github.com/air-verse/air).

`DATABASE_URL` is set automatically by Compose — do not override it in `.env`
unless running natively.

### Native (without Docker app container)

```bash
make db-up          # start only the postgres container
# set DATABASE_URL in .env (uncomment the postgres line)
make dev-native     # run app with air hot-reload (requires air: go install github.com/air-verse/air@latest)
```

### Make targets

| Target | Description |
|--------|-------------|
| `make dev` | Docker dev stack with hot-reload |
| `make dev-down` | Stop the dev stack |
| `make db-up` | Start only the database |
| `make dev-native` | Hot-reload natively (requires `air` on PATH) |
| `make build` | Build binary to `bin/jobhuntr` |
| `make run` | Build and run via `run.sh` |
| `make test` | `go test ./...` |
| `make test-race` | `go test -race ./...` |
| `make clean` | Remove `bin/`, `tmp/`, generated output files |

## Configuration

Copy `config.yaml.example` to `config.yaml` and fill in the values, or provide
everything via environment variables. `config.Load()` sources `.env` automatically.

### Environment variables

| Variable | Required | Description |
|----------|----------|-------------|
| `SESSION_SECRET` | Yes | 32+ byte random key — `openssl rand -hex 32` |
| `GITHUB_CLIENT_ID` | Yes | GitHub OAuth App client ID |
| `GITHUB_CLIENT_SECRET` | Yes | GitHub OAuth App client secret |
| `DATABASE_URL` | Yes (native only) | PostgreSQL DSN — set automatically by Compose |
| `GOOGLE_CLIENT_ID` | No | Google OAuth App client ID |
| `GOOGLE_CLIENT_SECRET` | No | Google OAuth App client secret |
| `SERPAPI_KEY` | No | Enables job scraping |
| `ANTHROPIC_API_KEY` | No | Enables resume/cover letter generation |

### OAuth setup

**GitHub** (required):
1. Go to GitHub → Settings → Developer settings → OAuth Apps → New OAuth App.
2. Set *Homepage URL* to your app's base URL (e.g. `http://localhost:8080`).
3. Set *Authorization callback URL* to `<base_url>/auth/github/callback`.
4. Copy the client ID and secret into `.env`.

**Google** (optional):
1. Open [Google Cloud Console](https://console.cloud.google.com) → APIs & Services → Credentials.
2. Create an OAuth 2.0 Client ID (Web application).
3. Add `<base_url>/auth/google/callback` to *Authorised redirect URIs*.
4. Copy the client ID and secret into `.env`.

### Per-user settings

The following are configured per-user in the **Settings** page after sign-in —
they are not global environment variables:

- **Search filters** — keywords, location, salary range
- **ntfy topic** — your personal ntfy.sh topic for job notifications
- **Resume** — your base resume in Markdown; Claude tailors it per job

## Usage Workflow

1. Sign in via GitHub or Google OAuth.
2. Add search filters in Settings.
3. JobHuntr scrapes Google Jobs via SerpAPI on the configured interval (default 1 hour).
4. Each new job triggers a push notification to your ntfy topic.
5. Open the dashboard from the notification link.
6. **Approve** jobs you are interested in; **Reject** ones you are not.
7. Approved jobs are queued for the background worker, which calls Claude to
   generate a tailored resume and cover letter in HTML and Markdown.
8. Optionally, Chromium converts the HTML to PDF.
9. The job detail page shows a preview and download links for Markdown, DOCX, and PDF formats.

## ntfy Setup

1. Install the [ntfy app](https://ntfy.sh/#download) on your phone.
2. Subscribe to a private topic (e.g. `jobhuntr-yourname-abc123`).
3. Enter the topic name in Settings → Notifications after signing in.

Keep your topic name private — anyone who knows it can post notifications to your device.

## Running with Docker (production-like)

```bash
cp .env.example .env          # fill in all required secrets
cp config.yaml.example config.yaml

docker compose up --build     # starts app + postgres
```

To run in the background:

```bash
docker compose up --build -d
docker compose logs -f app
```

## Deploy to Render

JobHuntr ships with a `render.yaml` Blueprint that provisions a web service
(Docker) and a managed Postgres database.

### One-time setup

1. **Create a Render account** at [render.com](https://render.com).

2. **Connect the repository:** Dashboard → *New → Blueprint*, select this repo.
   Render detects `render.yaml` and creates the `jobhuntr` web service and
   `jobhuntr-db` Postgres instance automatically.

3. **Set environment variables:** Open the `jobhuntr` service → *Environment*
   and add these secrets (marked `sync: false` so Render never stores them in
   the repo):

   | Variable | Where to get it |
   |----------|----------------|
   | `SESSION_SECRET` | `openssl rand -hex 32` |
   | `GITHUB_CLIENT_ID` | GitHub → Settings → Developer settings → OAuth Apps |
   | `GITHUB_CLIENT_SECRET` | (same) |
   | `GOOGLE_CLIENT_ID` | Google Cloud Console → APIs & Services → Credentials |
   | `GOOGLE_CLIENT_SECRET` | (same) |
   | `ANTHROPIC_API_KEY` | [console.anthropic.com](https://console.anthropic.com) |
   | `SERPAPI_KEY` | [serpapi.com/manage-api-key](https://serpapi.com/manage-api-key) |

4. **Set `base_url`:** Update `base_url` in `config.yaml` to your Render URL
   (e.g. `https://jobhuntr.onrender.com`). Register this URL as an authorised
   redirect URI in both your GitHub and Google OAuth app settings.

5. **Deploy:** Render builds automatically on every push to the connected branch.
   The first deploy runs Postgres migrations and starts the service.

### Render.yaml reference

See [render.com/docs/blueprint-spec](https://render.com/docs/blueprint-spec).

## Health Checks

`GET /healthz` — liveness probe, no auth, no database access:

```json
{"status": "ok"}
```

Used by Render's health check configuration.
