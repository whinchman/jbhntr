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
