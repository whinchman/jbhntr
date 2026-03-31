# Completed Work

Features moved here after merge to the default branch.

## Phase 1: Foundation

### 1C: SQLite Store (merged 2026-03-31)
- internal/store: Open(WAL mode), auto-migrate jobs+scrape_runs schema
- CreateJob (INSERT OR IGNORE dedup), GetJob, ListJobs, UpdateJobStatus, UpdateJobGenerated, CreateScrapeRun
- State machine transition enforcement (discovered→notified→approved→generating→complete/failed)
- 20 in-memory SQLite tests

### 1B: Config & Models (merged 2026-03-31)
- internal/models: Job struct (all DB fields + timestamps), JobStatus constants + Valid(), SearchFilter
- table-driven tests for all 7 status constants and invalid inputs

### 1A: Project Skeleton (merged 2026-03-31)
- Go module initialized: github.com/whinchman/jobhuntr
- Full directory structure: cmd/jobhuntr/, internal/{config,models,store,scraper,notifier,generator,pdf,web}/
- internal/config: YAML loading with ${ENV_VAR} substitution, table-driven tests
- config.yaml.example with all config sections
- .gitignore covering bin/, output/, worktrees/, *.db, config.yaml
- cmd/jobhuntr/main.go: loads config, structured slog output, prints startup message
