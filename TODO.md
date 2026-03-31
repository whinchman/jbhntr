# Project Backlog

Status key: `[ ]` pending | `[~]` in progress | `[x]` done

Completed work is moved to DONE.md after each feature merge.

---

## Phase 1: Foundation

### 1A: Project Skeleton
- [x] Initialize Go module (github.com/whinchman/jobhuntr), create project directory structure (cmd/jobhuntr/, internal/{config,models,store,scraper,notifier,generator,pdf,web}/), create config.yaml template with all sections (server, scraper, search_filters, ntfy, claude, resume, output), .gitignore (bin/, output/, worktrees/, *.db), and a main.go that loads config and prints "jobhuntr starting on :PORT"

### 1B: Config & Models
- [x] Implement config loading from config.yaml with env var substitution (replace ${VAR} with os.Getenv), define all data models in internal/models/: Job struct (all DB fields), JobStatus string type with constants (Discovered/Notified/Approved/Rejected/Generating/Complete/Failed) and a Valid() method, SearchFilter struct (Keywords/Location/MinSalary/MaxSalary/Title). Write tests for config parsing (including env var substitution) and model validation

### 1C: SQLite Store
- [x] Implement SQLite store in internal/store/ using modernc.org/sqlite with database/sql: auto-migrate schema on Open() (jobs table with all fields + UNIQUE(external_id,source) + CHECK on status + indexes on status and discovered_at, scrape_runs table), implement CreateJob (INSERT OR IGNORE for dedup returning whether inserted), GetJob, ListJobs(filter by status, search text via LIKE, pagination), UpdateJobStatus (with valid transition check), UpdateJobGenerated (set HTML+PDF paths), CreateScrapeRun, and write comprehensive tests using in-memory SQLite

## Phase 2: Scraping

### 2A: Scraper Interface & SerpAPI
- [x] Define Source interface in internal/scraper/source.go: Search(ctx context.Context, filter models.SearchFilter) ([]models.Job, error). Implement SerpAPISource in internal/scraper/serpapi.go that calls the SerpAPI google_jobs engine with q=filter.Keywords, location=filter.Location, parses the JSON response mapping jobs_results[] to Job models (extract title, company_name, location, description, detected_extensions.salary, apply_link or share_link as apply_url, job_id as external_id). Add rate limiting with golang.org/x/time/rate. Write tests using httptest.NewServer with sample SerpAPI JSON responses

### 2B: Scrape Scheduler
- [x] Implement Scheduler in internal/scraper/scheduler.go: a struct with Start(ctx) that launches a goroutine running scrapes at the configured interval (time.Ticker). For each tick: iterate all search filters from config, call Source.Search for each, attempt store.CreateJob for each result (dedup handled by DB), count new jobs, log scrape run via store.CreateScrapeRun, return list of newly discovered jobs. Add a method RunOnce(ctx) for testing. Wire into main.go (start scheduler in goroutine, pass cancel context). Write tests with mock Source and mock Store

## Phase 3: Notifications & Web API

### 3A: Ntfy Notifier
- [ ] Implement Notifier in internal/notifier/notifier.go: Notify(ctx, job) error that POSTs to ntfy.sh/{topic} with JSON body containing title ("New: {job.Title} at {job.Company}"), message (location + salary if available), click URL pointing to web dashboard job detail ({base_url}/jobs/{id}), priority 3 (default), and tags ["briefcase"]. Wire into scheduler: after each scrape, for every newly created job, call Notify and update status to "notified". Write tests using httptest to verify the POST payload format

### 3B: Web Server & API Endpoints
- [ ] Set up HTTP server in internal/web/server.go using chi: GET /api/jobs (query params: status, q for search, page/limit for pagination, returns JSON array), GET /api/jobs/{id} (returns JSON), POST /api/jobs/{id}/approve (updates status discovered/notified→approved, returns updated job JSON), POST /api/jobs/{id}/reject (updates status→rejected), GET /health (returns {"status":"ok"}). Add request logging middleware with slog. Wire into main.go (start HTTP server, graceful shutdown with context). Write handler tests using httptest.NewRecorder

## Phase 4: Resume Generation

### 4A: Claude API Generator
- [ ] Implement Generator in internal/generator/generator.go using go-anthropic/v2: Generate(ctx, job, baseResume) (resumeHTML, coverHTML, error). Build a system prompt that instructs Claude to output two HTML documents separated by "---SEPARATOR---": first a tailored resume HTML matching the job requirements, then a cover letter HTML. Read resume.md from config path. Use claude-sonnet-4-20250514 model. Parse response by splitting on separator. Store prompts in internal/generator/prompts.go as constants/templates. Write tests with a mock Anthropic client

### 4B: PDF Generation & Background Worker
- [ ] Implement PDF converter in internal/pdf/pdf.go using go-rod: launch browser once (rod.New().MustConnect()), PDFFromHTML(htmlContent, outputPath) that creates a page, sets HTML content, calls Page.PDF with A4 paper size and reasonable margins, saves to file. Implement Worker in internal/generator/worker.go: Start(ctx) goroutine that polls store every 30s for jobs with status "approved", for each: update status→generating, call Generator.Generate, call PDF converter for both resume and cover letter, save to output/{job_id}/, update job with HTML+PDF paths and status→complete (or →failed with error). Wire into main.go. Write tests

## Phase 5: Web Dashboard

### 5A: Dashboard Layout & Job List
- [ ] Create web dashboard templates in internal/web/templates/ using Go html/template + HTMX + Pico CSS CDN: layout.html (base layout with nav, Pico CSS link, HTMX script tag), dashboard.html (extends layout: status filter tabs using hx-get to /partials/job-table?status=X, search input with hx-get hx-trigger="keyup changed delay:300ms", job table with columns: Title, Company, Location, Salary, Status, Date, Actions), partials/job_rows.html (table body partial for HTMX swaps, auto-refresh with hx-trigger="every 30s"). Add template rendering to server.go: GET / serves dashboard, GET /partials/job-table returns partial. Embed templates with //go:embed

### 5B: Job Detail Page & PDF Downloads
- [ ] Create job_detail.html template: shows full job description, company, location, salary, apply link (external), status badge, approve/reject buttons (hx-post to /api/jobs/{id}/approve|reject, hx-swap="outerHTML" to update status badge and hide buttons), and if status=complete: rendered resume HTML preview, cover letter HTML preview, download buttons for resume.pdf and cover_letter.pdf. Add routes: GET /jobs/{id} serves detail page, GET /output/{id}/resume.pdf and /output/{id}/cover_letter.pdf serve files with Content-Disposition: attachment headers. Use http.ServeFile

### 5C: Settings Page
- [ ] Create settings.html template: display current search filters from config (keyword, location, salary range per filter), display base resume content (from resume.md), provide a form to add/remove search filters and edit resume content. POST /settings/filters updates config.yaml and reloads config in memory. POST /settings/resume writes to resume.md. Show a flash message on save. Keep it simple - direct file writes, no database for settings

## Phase 6: Polish

### 6A: Graceful Shutdown & Logging
- [ ] Add graceful shutdown to main.go: listen for SIGINT/SIGTERM, cancel root context, wait for scheduler loop to finish current scrape, wait for generator worker to finish current job, shut down HTTP server with timeout, close database, log clean exit. Ensure slog is used consistently across all packages with structured fields (job_id, source, status, duration, etc). Add GET /health endpoint returning uptime and last scrape time

### 6B: Systemd & Docs
- [ ] Create deploy/jobhuntr.service systemd unit file (Type=simple, restart on failure, EnvironmentFile for secrets), create .env.example with all required env vars (ANTHROPIC_API_KEY, SERPAPI_KEY, NTFY_TOPIC), create README.md documenting: what jobhuntr does, prerequisites (Go 1.24, Chromium), installation, configuration (config.yaml fields), running (direct and systemd), ntfy setup, usage workflow
