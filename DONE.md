# Completed Work

Features moved here after merge to the default branch.

---

## Phase 7: Post-launch fixes & enhancements (2026-03-31)

### Job description summarization
- Claude Haiku summarizes each new job listing (1-2 sentences) and extracts salary
- Summary displayed below each row on the dashboard
- New `summary` and `extracted_salary` columns with auto-migration for existing DBs

### Sortable dashboard columns
- Clickable column headers toggle ascending/descending sort
- Sort param whitelist prevents SQL injection
- Default: discovered_at DESC (newest first)

### Bug fixes
- Worker now reads resume.md instead of passing empty string to Claude
- HTMX approve/reject returns HTML for HTMX requests, JSON for API clients
- Error messages stored in DB on job failure (new UpdateJobError method)
- First scrape runs immediately on startup instead of waiting for interval
- Settings page preserves ${ENV_VAR} placeholders when writing config.yaml
- SerpAPI: "Remote" location folded into query string (SerpAPI rejects it as a location)
- Auto-load .env file from config directory (no manual sourcing needed)
- SerpAPI error responses now include body text for debugging

## Phase 6: Polish (2026-03-31)

### 6B: Systemd & Docs
- deploy/jobhuntr.service systemd unit (Type=simple, restart on failure)
- .env.example with all required env vars
- README.md with full setup and usage docs

### 6A: Graceful Shutdown & Logging
- SIGINT/SIGTERM handling, context cancellation, WaitGroup for in-flight work
- HTTP server graceful shutdown with 10s timeout
- GET /health with uptime and last scrape time
- Structured slog logging throughout all packages

## Phase 5: Web Dashboard (2026-03-31)

### 5C: Settings Page
- settings.html: view/edit search filters and base resume
- Add/remove filters, save resume content
- Flash message on save

### 5B: Job Detail Page & PDF Downloads
- job_detail.html: full description, approve/reject buttons, generated doc previews
- PDF download routes with Content-Disposition headers

### 5A: Dashboard Layout & Job List
- Go html/template + HTMX + Pico CSS
- Status filter tabs, search with debounce, auto-refresh every 30s
- Embedded templates via //go:embed

## Phase 4: Resume Generation (2026-03-31)

### 4B: PDF Generation & Background Worker
- RodConverter: go-rod headless Chrome A4 PDF with 1.5cm margins
- Worker: 30s poll loop, approved→generating→complete/failed transitions
- 5 worker tests with mocks

### 4A: Claude API Generator
- AnthropicGenerator: system+user prompt, separator-based parsing
- Prompt templates in prompts.go
- Tests with mock Anthropic client

## Phase 3: Notifications & Web API (2026-03-31)

### 3B: Web Server & API Endpoints
- chi router with slog request logger + recovery middleware
- GET /health, GET /api/jobs, GET /api/jobs/{id}, POST /approve, POST /reject
- 10 tests with mock JobStore

### 3A: Ntfy Notifier
- NtfyNotifier: POST to ntfy.sh with title, message, click URL, tags
- Wired into scheduler for new job notifications

## Phase 2: Scraping (2026-03-31)

### 2B: Scrape Scheduler
- Scheduler with Start(ctx) and RunOnce(ctx)
- Iterates filters, deduplicates, logs scrape runs
- Tests with mock Source and Store

### 2A: Scraper Interface & SerpAPI
- Source interface (Search method)
- SerpAPISource with 1 req/s rate limiter, full JSON mapping
- 5 httptest-based tests

## Phase 1: Foundation (2026-03-31)

### 1C: SQLite Store
- Open with WAL mode, auto-migrate schema
- CreateJob (INSERT OR IGNORE dedup), GetJob, ListJobs, UpdateJobStatus, UpdateJobGenerated, CreateScrapeRun
- State machine transition enforcement
- 20 in-memory SQLite tests

### 1B: Config & Models
- Job struct, JobStatus constants + Valid(), SearchFilter
- YAML config loading with ${ENV_VAR} substitution
- Table-driven tests

### 1A: Project Skeleton
- Go module: github.com/whinchman/jobhuntr
- Full directory structure, config.yaml.example, .gitignore
- cmd/jobhuntr/main.go entry point with slog
