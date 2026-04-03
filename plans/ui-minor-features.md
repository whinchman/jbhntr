# Architecture Plan: UI Minor Features

**Features**: Scrape Countdown Timer (A) + Footer Attribution (B)
**Date**: 2026-04-03
**Status**: Draft

---

## Architecture Overview

Both features are purely presentational and require only template and minor
server changes. Neither introduces new dependencies, database migrations, or
background goroutines.

### Feature A — Scrape Countdown Timer

The goal is to show "Next scrape in X minutes Y seconds" between the search/
filter bar and the job listing table on the authenticated dashboard view.

**Key architectural decisions:**

1. **Where does the "next scrape" time come from?**
   The `Scheduler` struct in `internal/scraper/scheduler.go` already exposes
   `LastScrapeAt() time.Time` (line 54), which is the timestamp of the most
   recent completed cycle. The scrape interval comes from `config.yaml` →
   `scraper.interval` (currently `1h`). `NextScrapeAt = LastScrapeAt + interval`.

2. **How does the server know the interval?**
   `main.go` creates the scheduler with `interval` parsed from config (line 65)
   and already calls `webSrv.WithLastScrapeFn(sched.LastScrapeAt)`. We need to
   add a parallel `WithScrapeInterval(d time.Duration)` setter so the web server
   can compute `NextScrapeAt` without an extra RPC.

3. **How does the countdown stay live in the browser?**
   **Chosen approach: pure client-side JavaScript countdown (static HTML + JS).**
   The server renders `data-next-scrape="<RFC3339 timestamp>"` into the HTML
   once per page load. A small inline `<script>` reads that attribute and counts
   down using `setInterval`. This avoids HTMX polling overhead (a new HTTP
   request every second would be wasteful) and SSE plumbing for a purely
   cosmetic widget.

   Alternative considered: HTMX polling (`hx-trigger="every 1s"`) against a
   new `/partials/scrape-countdown` endpoint. Rejected because per-second HTTP
   polling from every dashboard tab is disproportionate to the value.

   Alternative considered: Server-Sent Events. Rejected as overkill for a
   single countdown label.

4. **What if no scrape has run yet?** (`LastScrapeAt` returns zero value)
   Show "Scrape running soon…" until the first cycle completes.

5. **Where is the widget placed?**
   In `dashboard.html`, inside the `{{if .User}}` block, between the search
   `<input>` element and the `<div hx-get="/partials/job-table" …>` wrapper.

### Feature B — Footer Attribution

Add a `<footer>` to the global layout. The layout template
(`internal/web/templates/layout.html`) currently has no footer element — the
`<body>` closes immediately after `<main class="app-container">`. We add a
minimal footer above `</body>`.

**Key decisions:**

1. Static HTML — no server-side data needed.
2. The footer is added to `layout.html` so it appears on every page
   (dashboard, settings, profile, job detail, etc.).
3. Link opens in a new tab (`target="_blank" rel="noopener noreferrer"`).
4. Styled with existing CSS tokens; one new `.app-footer` rule in `app.css`.

---

## Files Affected

| File | Change |
|------|--------|
| `internal/scraper/scheduler.go` | Add `Interval() time.Duration` method |
| `internal/web/server.go` | Add `scrapeInterval` field; add `WithScrapeInterval` setter; add `NextScrapeAt time.Time` to `dashboardData`; populate it in `handleDashboard` |
| `internal/web/templates/dashboard.html` | Add countdown widget between search bar and table |
| `internal/web/templates/layout.html` | Add `<footer>` element |
| `internal/web/templates/static/app.css` | Add `.app-footer` and `.scrape-countdown` CSS rules |
| `cmd/jobhuntr/main.go` | Call `WithScrapeInterval(interval)` when constructing the web server |

---

## How to Get the Next Scrape Time from the Backend

### Current state (scheduler.go)

```go
// Scheduler already has:
func (s *Scheduler) LastScrapeAt() time.Time   // returns s.lastScrapeAt (mutex-protected)

// Add this new method:
func (s *Scheduler) Interval() time.Duration {
    return s.interval
}
```

### Server-side wiring (server.go)

```go
// Add field to Server struct:
scrapeInterval time.Duration  // zero means unknown

// Add setter (mirrors WithLastScrapeFn pattern):
func (s *Server) WithScrapeInterval(d time.Duration) *Server {
    s.scrapeInterval = d
    return s
}

// Extend dashboardData:
type dashboardData struct {
    // ... existing fields ...
    NextScrapeAt time.Time  // zero if unknown
}

// In handleDashboard, compute NextScrapeAt:
var nextScrape time.Time
if s.lastScrapeFn != nil && s.scrapeInterval > 0 {
    last := s.lastScrapeFn()
    if !last.IsZero() {
        nextScrape = last.Add(s.scrapeInterval)
    }
}
data := dashboardData{
    // ... existing fields ...
    NextScrapeAt: nextScrape,
}
```

### main.go change

```go
webSrv := web.NewServerWithConfig(db, db, db, cfg).
    WithLastScrapeFn(sched.LastScrapeAt).
    WithScrapeInterval(interval).   // add this line
    WithMailer(m)
```

---

## Template Implementation

### dashboard.html — countdown widget

Insert between the closing `</input>` search bar and the `<div hx-get=...>` table wrapper:

```html
{{if .User}}
{{if not .NextScrapeAt.IsZero}}
<p class="scrape-countdown"
   data-next-scrape="{{.NextScrapeAt.UTC.Format "2006-01-02T15:04:05Z"}}">
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
{{else}}
<p class="scrape-countdown">Scrape running soon…</p>
{{end}}
{{end}}
```

Note: Go templates do not have a `not` keyword. Use `{{if .NextScrapeAt.IsZero}}` /
`{{else}}` with the branches swapped:

```html
{{if .User}}
<p class="scrape-countdown"
   {{if not .NextScrapeAt.IsZero}}data-next-scrape="{{.NextScrapeAt.UTC.Format "2006-01-02T15:04:05Z"}}"{{end}}>
  {{if .NextScrapeAt.IsZero}}Scrape running soon…{{else}}Next scrape in <span id="scrape-timer">…</span>{{end}}
</p>
{{if not .NextScrapeAt.IsZero}}
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
{{end}}
```

Go templates do not support `not` as a function out of the box. Use a template
function or restructure with nested `if/else`. The coder should use:

```html
{{if .User}}
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
{{end}}
```

Placement in `dashboard.html`: after the `</input>` closing tag for the search
box (line 37) and before the `<div hx-get="/partials/job-table"` opening tag
(line 39).

### layout.html — footer

Insert before `</body>`:

```html
  <footer class="app-footer">
    <p>created out of spite by <a href="https://www.217industries.com" target="_blank" rel="noopener noreferrer">217Industries</a></p>
  </footer>
```

### app.css — new rules

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

---

## Step-by-Step Implementation Plan

### Step 1 — Add `Interval()` method to Scheduler

**File**: `internal/scraper/scheduler.go`

Add after the existing `LastScrapeAt()` method (after line 58):

```go
// Interval returns the configured scrape interval.
func (s *Scheduler) Interval() time.Duration {
    return s.interval
}
```

No locking needed — `interval` is set once at construction and never mutated.

### Step 2 — Wire interval into the web Server

**File**: `internal/web/server.go`

2a. Add field to `Server` struct (after `lastScrapeFn` on line 123):
```go
scrapeInterval time.Duration
```

2b. Add `WithScrapeInterval` setter (after `WithLastScrapeFn` at line 229):
```go
// WithScrapeInterval sets the scrape interval so the dashboard can display a
// countdown to the next scheduled run.
func (s *Server) WithScrapeInterval(d time.Duration) *Server {
    s.scrapeInterval = d
    return s
}
```

2c. Extend `dashboardData` struct (after `User *models.User` on line 399):
```go
NextScrapeAt time.Time
```

2d. Populate `NextScrapeAt` in `handleDashboard` (after the `jobs` fetch, before
building `data`):
```go
var nextScrape time.Time
if s.lastScrapeFn != nil && s.scrapeInterval > 0 {
    if last := s.lastScrapeFn(); !last.IsZero() {
        nextScrape = last.Add(s.scrapeInterval)
    }
}
```
Add `NextScrapeAt: nextScrape` to the `dashboardData` literal.

### Step 3 — Update main.go

**File**: `cmd/jobhuntr/main.go`

Add `.WithScrapeInterval(interval)` to the `webSrv` builder chain (after
`.WithLastScrapeFn(sched.LastScrapeAt)` on line 111).

### Step 4 — Update dashboard.html

**File**: `internal/web/templates/dashboard.html`

Insert the countdown snippet (see Template Implementation above) between the
search `<input>` (ends at line 37) and the `<div hx-get="/partials/job-table"`
(starts at line 39). Keep the block inside the outer `{{if .User}}` guard.

### Step 5 — Update layout.html

**File**: `internal/web/templates/layout.html`

Insert `<footer class="app-footer">…</footer>` immediately before `</body>`
(currently line 55).

### Step 6 — Update app.css

**File**: `internal/web/templates/static/app.css`

Append sections 17 (footer) and 18 (scrape countdown) at the end of the file.

---

## Trade-offs and Alternatives

### Countdown approach

| Approach | Pros | Cons |
|----------|------|------|
| **Client-side JS countdown (chosen)** | Zero extra HTTP requests; instant ticks; simple to implement | Tiny clock drift vs. server time (acceptable) |
| HTMX polling every 1s | No JS to write; pure HTMX pattern | 1 req/sec per open tab; wasteful for a cosmetic widget |
| Server-Sent Events | Real-time push, no polling | Significant plumbing; overkill for a countdown label |

### Passing interval to the server

| Approach | Pros | Cons |
|----------|------|------|
| **New `WithScrapeInterval` setter (chosen)** | Mirrors existing `WithLastScrapeFn` pattern exactly | Requires two setters instead of one |
| Pass a `NextScrapeAtFn func() time.Time` closure | Encapsulates both pieces | Caller must compute `last+interval` in main.go |
| Expose interval via config in server | Server already holds `*config.Config` | Config has `Interval` as string, needs parsing again |

The chosen approach is the smallest change consistent with existing patterns.

---

## Acceptance Criteria

### Feature A — Scrape Countdown Timer

- [ ] An authenticated user visiting `/` sees a "Next scrape in Xm Ys" line
  between the search bar and the job table.
- [ ] The countdown ticks down in real time without page reloads.
- [ ] When `NextScrapeAt` is in the past (overdue scrape) the widget shows
  "any moment now" rather than a negative countdown.
- [ ] Before the first scrape has run (zero time), the widget shows
  "Scrape running soon…".
- [ ] The widget is not shown when the user is logged out.
- [ ] No new HTTP endpoints are required for the countdown.

### Feature B — Footer Attribution

- [ ] Every page (dashboard, settings, profile, job detail) shows a footer
  with the text "created out of spite by 217Industries".
- [ ] "217Industries" is a hyperlink to `https://www.217industries.com`.
- [ ] The link opens in a new tab.
- [ ] The footer is visually unobtrusive — small muted text at the page bottom.

---

## Dependencies and Prerequisites

- No new Go modules required.
- No database migrations.
- No environment variable changes.
- The `interval` variable is already available in `main.go` (line 65) before
  the web server is constructed.

---

## Risks and Notes

- The countdown is computed from the server-rendered `NextScrapeAt` timestamp.
  If the user's browser clock differs significantly from the server clock, the
  countdown will be slightly off. This is acceptable for a cosmetic indicator.
- If the scheduler is restarted or the server restarts, `LastScrapeAt` resets
  to zero and the widget reverts to "Scrape running soon…" until the next
  cycle completes. This is correct behaviour.
- Go's `html/template` auto-escapes attribute values. The RFC3339 timestamp
  contains only safe characters (`0-9`, `T`, `Z`, `-`, `:`), so no special
  escaping is needed.
