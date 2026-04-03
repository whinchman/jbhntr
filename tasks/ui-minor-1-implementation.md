# Task: ui-minor-1-implementation

- **Type**: coder
- **Status**: pending
- **Repo**: .
- **Parallel Group**: 1
- **Branch**: feature/ui-minor-1-implementation
- **Source Item**: UI Minor Features — Scrape Countdown Timer (A) + Footer Attribution (B)
- **Dependencies**: none

## Description

Implement two small UI features that touch overlapping files. Both are purely
presentational — no new dependencies, no database migrations, no background goroutines.

**Feature A — Scrape Countdown Timer**

Show "Next scrape in Xm Ys" between the search/filter bar and the job table on
the authenticated dashboard. The countdown is rendered client-side from a
`data-next-scrape` RFC3339 timestamp written into the HTML once per page load.

**Feature B — Footer Attribution**

Add a `<footer class="app-footer">` to the global layout so that every page
(dashboard, settings, profile, job detail) shows "created out of spite by 217Industries"
linking to `https://www.217industries.com` in a new tab.

### Step-by-step

**Step 1 — `internal/scraper/scheduler.go`**

Add `Interval()` method after the existing `LastScrapeAt()` method (around line 58):

```go
// Interval returns the configured scrape interval.
func (s *Scheduler) Interval() time.Duration {
    return s.interval
}
```

No mutex needed — `interval` is set once at construction and never mutated.

**Step 2 — `internal/web/server.go`**

2a. Add field to `Server` struct (after `lastScrapeFn` around line 123):
```go
scrapeInterval time.Duration
```

2b. Add setter after `WithLastScrapeFn` (around line 229):
```go
// WithScrapeInterval sets the scrape interval so the dashboard can display a
// countdown to the next scheduled run.
func (s *Server) WithScrapeInterval(d time.Duration) *Server {
    s.scrapeInterval = d
    return s
}
```

2c. Extend `dashboardData` struct (after `User *models.User` around line 399):
```go
NextScrapeAt time.Time
```

2d. Populate `NextScrapeAt` in `handleDashboard` before building `data`:
```go
var nextScrape time.Time
if s.lastScrapeFn != nil && s.scrapeInterval > 0 {
    if last := s.lastScrapeFn(); !last.IsZero() {
        nextScrape = last.Add(s.scrapeInterval)
    }
}
```
Add `NextScrapeAt: nextScrape` to the `dashboardData` literal.

**Step 3 — `cmd/jobhuntr/main.go`**

Add `.WithScrapeInterval(interval)` to the `webSrv` builder chain after
`.WithLastScrapeFn(sched.LastScrapeAt)` (around line 111):

```go
webSrv := web.NewServerWithConfig(db, db, db, cfg).
    WithLastScrapeFn(sched.LastScrapeAt).
    WithScrapeInterval(interval).
    WithMailer(m)
```

**Step 4 — `internal/web/templates/dashboard.html`**

Insert the following block between the closing search `<input>` (line ~37) and
the `<div hx-get="/partials/job-table"` wrapper (line ~39). Keep it inside the
outer `{{if .User}}` guard:

```html
{{if .NextScrapeAt.IsZero}}
<p class="scrape-countdown">Scrape running soon…</p>
{{else}}
<p class="scrape-countdown" data-next-scrape="{{.NextScrapeAt.UTC.Format "2006-01-02T15:04:05Z"}}">
  Next scrape in <span id="scrape-timer">…</span>
</p>
<script>
(function() {
  var el = document.querySelector('[data-next-scrape]');
  if (!el) return;
  var target = new Date(el.getAttribute('data-next-scrape'));
  var span = document.getElementById('scrape-timer');
  function tick() {
    var diff = Math.max(0, Math.round((target - Date.now()) / 1000));
    if (diff === 0) { span.textContent = 'any moment now'; return; }
    var m = Math.floor(diff / 60);
    var s = diff % 60;
    span.textContent = (m > 0 ? m + 'm ' : '') + s + 's';
  }
  tick();
  setInterval(tick, 1000);
})();
</script>
{{end}}
```

Note: the outer `{{if .User}}` guard already exists in the template — insert
only the inner block above.

**Step 5 — `internal/web/templates/layout.html`**

Insert `<footer>` immediately before `</body>` (currently around line 55):

```html
  <footer class="app-footer">
    <p>created out of spite by <a href="https://www.217industries.com" target="_blank" rel="noopener noreferrer">217Industries</a></p>
  </footer>
```

**Step 6 — `internal/web/templates/static/app.css`**

Append two new sections at the end of the file:

```css
/* =============================================================================
   17. FOOTER
   ============================================================================= */

.app-footer {
  margin-top: var(--space-12);
  padding: var(--space-6) var(--space-4);
  border-top: 1px solid var(--color-border);
  text-align: center;
}

.app-footer p {
  font-size: var(--text-xs);
  color: var(--color-text-muted);
  margin: 0;
}

.app-footer a {
  color: var(--color-text-muted);
  text-decoration: underline;
}

.app-footer a:hover {
  color: var(--color-accent);
}

/* =============================================================================
   18. SCRAPE COUNTDOWN
   ============================================================================= */

.scrape-countdown {
  font-size: var(--text-sm);
  color: var(--color-text-muted);
  margin-bottom: var(--space-4);
}
```

## Acceptance Criteria

- [ ] `internal/scraper/scheduler.go` exposes `Interval() time.Duration` method
- [ ] `internal/web/server.go` has `scrapeInterval` field, `WithScrapeInterval` setter, and `NextScrapeAt time.Time` in `dashboardData`
- [ ] `handleDashboard` computes and populates `NextScrapeAt` correctly
- [ ] `cmd/jobhuntr/main.go` calls `.WithScrapeInterval(interval)` in the builder chain
- [ ] Authenticated dashboard shows "Next scrape in Xm Ys" between the search bar and the job table when a previous scrape has run
- [ ] Countdown ticks down in real time without page reloads
- [ ] When `NextScrapeAt` is in the past the widget shows "any moment now"
- [ ] Before the first scrape runs (zero time) the widget shows "Scrape running soon…"
- [ ] The countdown widget is not rendered when the user is logged out
- [ ] No new HTTP endpoints are added
- [ ] Every page shows a footer with "created out of spite by 217Industries"
- [ ] "217Industries" links to `https://www.217industries.com` and opens in a new tab
- [ ] `.app-footer` and `.scrape-countdown` CSS rules are present in `app.css`
- [ ] `go build ./...` succeeds with no errors

## Interface Contracts

No cross-repo contracts. This is a single-repo project.

## Context

Architecture plan: `/workspace/plans/ui-minor-features.md`

The `interval` variable is already in scope in `main.go` (parsed from config at
line ~65) before the web server is constructed. `WithLastScrapeFn` is the
existing pattern this change mirrors.

Go's `html/template` auto-escapes attribute values. The RFC3339 timestamp
contains only safe characters so no special handling is needed. Go templates do
not have a `not` function — use nested `{{if}}` / `{{else}}` as shown above.

## Notes

