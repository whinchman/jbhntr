# Task: ui-minor-2-qa

- **Type**: qa
- **Status**: pending
- **Repo**: .
- **Parallel Group**: 2
- **Branch**: feature/ui-minor-1-implementation
- **Source Item**: UI Minor Features — Scrape Countdown Timer (A) + Footer Attribution (B)
- **Dependencies**: ui-minor-1-implementation

## Description

Verify the two UI features implemented in task `ui-minor-1-implementation` are
correct and working:

**Feature A — Scrape Countdown Timer**: The authenticated dashboard should show
a live countdown between the search bar and the job table. The countdown should
tick, handle zero-time (no scrape yet), and handle overdue scrapes gracefully.

**Feature B — Footer Attribution**: Every page should render a footer with the
"created out of spite by 217Industries" text linking to `https://www.217industries.com`
in a new tab.

Write tests covering the server-side logic (NextScrapeAt computation) and
confirm the template snippets and CSS rules are present. Run the full test
suite and report results.

### Areas to test

1. **`Interval()` method** — unit test that `Scheduler.Interval()` returns the
   value passed at construction.

2. **`WithScrapeInterval` / `NextScrapeAt` computation** — unit test the
   `handleDashboard` logic or the data-population logic:
   - When `lastScrapeFn` is nil → `NextScrapeAt` is zero.
   - When `lastScrapeFn` returns zero → `NextScrapeAt` is zero.
   - When `lastScrapeFn` returns a valid time and `scrapeInterval > 0` →
     `NextScrapeAt == last + interval`.
   - When `scrapeInterval == 0` → `NextScrapeAt` is zero.

3. **Template smoke checks** — verify the templates contain expected markers:
   - `dashboard.html` contains `scrape-countdown` and `data-next-scrape`.
   - `layout.html` contains `app-footer` and `217industries.com`.
   - `app.css` contains `.app-footer` and `.scrape-countdown` rule definitions.

4. **Build check** — `go build ./...` must pass with no errors.

5. **Existing tests** — run `go test ./...` and confirm no regressions.

## Acceptance Criteria

- [ ] Unit test for `Scheduler.Interval()` passes
- [ ] Unit or integration tests for `NextScrapeAt` computation cover all four cases (nil fn, zero time, valid time, zero interval)
- [ ] Template content assertions pass for `dashboard.html`, `layout.html`, and `app.css`
- [ ] `go build ./...` exits 0
- [ ] `go test ./...` exits 0 with no regressions
- [ ] Any bugs found are logged to `/workspace/workflow/BUGS.md` with file, severity, and reproduction steps

## Interface Contracts

No cross-repo contracts. Single-repo project.

## Context

Architecture plan: `/workspace/plans/ui-minor-features.md`

Implementation branch: `feature/ui-minor-1-implementation`

Key files touched by the implementation task:
- `internal/scraper/scheduler.go` — new `Interval()` method
- `internal/web/server.go` — new `scrapeInterval` field, `WithScrapeInterval` setter, `NextScrapeAt` in `dashboardData`
- `cmd/jobhuntr/main.go` — `.WithScrapeInterval(interval)` added to builder chain
- `internal/web/templates/dashboard.html` — countdown widget inserted
- `internal/web/templates/layout.html` — footer element inserted
- `internal/web/templates/static/app.css` — `.app-footer` and `.scrape-countdown` rules appended

Existing test command from `agent.yaml`: `go test ./...`

## Notes

