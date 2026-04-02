# JobHuntr

JobHuntr is a headless Go application that automates job searching, delivers
push notifications, and generates tailored resumes and cover letters using the
Claude API.

```
Scheduler (hourly) → SerpAPI Google Jobs → SQLite
                                              ↓ new jobs
                                         ntfy.sh → Phone notification
                                              ↓ user opens link
                                         Web Dashboard (approve/reject)
                                              ↓ approved
                                    Claude API → Resume + Cover Letter HTML
                                              ↓
                                    go-rod (headless Chromium) → PDF files
                                              ↓
                                    Web Dashboard (view + download)
```

## Prerequisites

| Requirement | Version |
|-------------|---------|
| Go | 1.22+ |
| Chromium / Chrome | any recent version |
| SerpAPI account | https://serpapi.com |
| Anthropic API key | https://console.anthropic.com |
| ntfy.sh account (optional) | https://ntfy.sh |

Chromium must be on `$PATH` as `chromium`, `chromium-browser`, or `google-chrome`.
go-rod will attempt to download it automatically if none is found.

## Installation

```bash
git clone https://github.com/whinchman/jobhuntr
cd jobhuntr
go build -o bin/jobhuntr ./cmd/jobhuntr
```

## Configuration

Copy `config.yaml` and fill in your values:

```yaml
server:
  port: 8080
  base_url: "http://localhost:8080"   # used in ntfy notification links

scraper:
  interval: "1h"
  serpapi_key: "${SERPAPI_KEY}"        # from environment

search_filters:
  - keywords: "senior software engineer golang"
    location: "Remote"
    min_salary: 150000

ntfy:
  topic: "${NTFY_TOPIC}"
  server: "https://ntfy.sh"

claude:
  api_key: "${ANTHROPIC_API_KEY}"
  model: "claude-sonnet-4-20250514"

resume:
  path: "./resume.md"    # your base resume in Markdown

output:
  dir: "./output"        # generated PDFs are written here
```

Set the required environment variables (or export them before running):

```bash
cp .env.example .env
# edit .env with your keys
export $(grep -v '^#' .env | xargs)
```

Create your base resume in Markdown at `resume.md`. JobHuntr passes this to
Claude, which tailors it for each approved job.

## Running

```bash
./bin/jobhuntr -config config.yaml -db jobhuntr.db
```

Flags:

| Flag | Default | Description |
|------|---------|-------------|
| `-config` | `config.yaml` | Path to config file |
| `-db` | `jobhuntr.db` | Path to SQLite database |

The web dashboard is available at `http://localhost:8080` (or the configured port).

## Usage Workflow

1. JobHuntr scrapes Google Jobs via SerpAPI on the configured interval.
2. Each new job triggers a push notification to your phone via ntfy.sh.
3. Open the dashboard from the notification link.
4. **Approve** jobs you are interested in; **Reject** ones you are not.
5. Approved jobs are picked up by the background worker, which calls Claude
   to generate a tailored resume and cover letter as HTML.
6. go-rod converts the HTML to PDF files and saves them under `output/{job_id}/`.
7. The job detail page shows a preview and download links for both PDFs.

## ntfy Setup

1. Install the [ntfy app](https://ntfy.sh/#download) on your phone.
2. Subscribe to your topic (e.g. `jobhuntr-yourname`).
3. Set `NTFY_TOPIC=jobhuntr-yourname` in your environment.

Keep your topic name private — anyone who knows it can post notifications to
your device.

## Running with Docker

The easiest way to run JobHuntr locally is with Docker Compose, which starts
the app and a Postgres database together.

**1. Set up your environment file:**

```bash
cp .env.example .env
# Edit .env and fill in ANTHROPIC_API_KEY, SERPAPI_KEY, NTFY_TOPIC, SESSION_SECRET, etc.
```

**2. Create a `config.yaml` from the example:**

```bash
cp config.yaml.example config.yaml
```

**3. Start the stack:**

```bash
docker compose up --build
```

The app will be available at `http://localhost:8080`. Generated PDFs are
written to `./output/` on the host. The `DATABASE_URL` is set automatically
by Compose to point at the `db` service — do not override it in `.env`.

To run in the background:

```bash
docker compose up --build -d
docker compose logs -f app
```

## Running with systemd

```bash
# Build and install
go build -o /usr/local/bin/jobhuntr ./cmd/jobhuntr

# Create system user and directories
useradd --system --no-create-home jobhuntr
mkdir -p /etc/jobhuntr /var/lib/jobhuntr /opt/jobhuntr/output
chown jobhuntr:jobhuntr /var/lib/jobhuntr /opt/jobhuntr/output

# Install config and env file
cp config.yaml /etc/jobhuntr/config.yaml
cp .env.example /etc/jobhuntr/jobhuntr.env
# edit /etc/jobhuntr/jobhuntr.env with your keys

# Install and start the service
cp deploy/jobhuntr.service /etc/systemd/system/
systemctl daemon-reload
systemctl enable --now jobhuntr
```

View logs:

```bash
journalctl -u jobhuntr -f
```

## Health Check

```
GET /health
```

Returns:

```json
{
  "status": "ok",
  "uptime": "2h15m30s",
  "last_scrape": "2026-01-01T10:00:00Z"
}
```

`last_scrape` is omitted if no scrape has completed yet.

A minimal liveness probe is also available at `GET /healthz` — returns
`{"status":"ok"}` with no auth and no database access. This is the path
used by Render's health check configuration.

## Deploy to Render

JobHuntr ships with a `render.yaml` file that defines a web service (Docker)
and a managed Postgres database. Render reads this file automatically when
you connect the repository.

### One-time setup

1. **Create a Render account** at [render.com](https://render.com) if you
   do not already have one.

2. **Connect your repository:** In the Render dashboard click
   *New → Blueprint*, select this repo, and Render will detect `render.yaml`
   and create the `jobhuntr` web service and `jobhuntr-db` Postgres instance.

3. **Set environment variables:** Open the `jobhuntr` web service in the
   dashboard, go to *Environment*, and add the following secrets (these are
   marked `sync: false` in `render.yaml` so Render never stores them in the
   repo):

   | Variable | Where to get it |
   |----------|----------------|
   | `ANTHROPIC_API_KEY` | [console.anthropic.com](https://console.anthropic.com) |
   | `SERPAPI_KEY` | [serpapi.com/manage-api-key](https://serpapi.com/manage-api-key) |
   | `NTFY_TOPIC` | Any private string; subscribe in the ntfy app |
   | `SESSION_SECRET` | Run `openssl rand -hex 32` locally |
   | `GOOGLE_CLIENT_ID` | Google Cloud Console → APIs & Services → Credentials |
   | `GOOGLE_CLIENT_SECRET` | (same as above) |
   | `GITHUB_CLIENT_ID` | GitHub → Settings → Developer settings → OAuth Apps |
   | `GITHUB_CLIENT_SECRET` | (same as above) |

4. **Set `base_url`:** Update `base_url` in `config.yaml` (or override it
   via an environment variable) to your Render web service URL, e.g.
   `https://jobhuntr.onrender.com`. This is required so that OAuth redirect
   URIs resolve correctly after login. You must also register this URL as an
   authorised redirect URI in your Google and GitHub OAuth app settings.

5. **Deploy:** Render triggers a build automatically on every push to the
   connected branch. The first deploy builds the Docker image, runs the
   Postgres migration, and starts the service.

### Render.yaml reference

See [render.com/docs/blueprint-spec](https://render.com/docs/blueprint-spec)
for full documentation on the `render.yaml` Blueprint format.
