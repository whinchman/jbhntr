# Task: tinder-mobile-frontend

- **Type**: coder
- **Status**: done
- **Parallel Group**: 1
- **Branch**: feature/tinder-mobile-frontend
- **Source Item**: Tinder-Style Mobile Job Review UI
- **Dependencies**: none

## Description

Add the frontend assets for the swipe-card mobile job review interface. This task covers:

1. **New file**: `internal/web/templates/partials/job_cards.html` — the card-deck template
2. **New file**: `internal/web/templates/static/swipe-cards.js` — the gesture/HTMX JS module
3. **Modified file**: `internal/web/templates/static/app.css` — append the new SWIPE CARDS section
4. **Modified file**: `internal/web/templates/dashboard.html` — add card deck section + scripts block

No Go server changes are part of this task (handled by `tinder-mobile-backend`).

## Acceptance Criteria

- [ ] `internal/web/templates/partials/job_cards.html` created with `{{define "job_cards"}}` template
- [ ] Template shows empty state when `len .Jobs == 0` with a Refresh button
- [ ] Template shows job counter (`N job(s) to review`) when jobs are present
- [ ] Template renders up to 2 ghost cards (aria-hidden, inert) for depth effect
- [ ] Template renders the active card as `<article class="job-card job-card-active">` with `data-job-id`, `tabindex="0"`
- [ ] Active card contains approve and reject overlay badges (aria-hidden)
- [ ] Active card shows: status badge, title (as link to `/jobs/{{.ID}}`), company, location (if set), salary (ExtractedSalary → Salary → omit), discovered date, summary (if set) with expand button
- [ ] Approve and reject forms present with `hx-target="#job-card-deck"` and `hx-swap="innerHTML"`, `data-action` attributes, and CSRF token input
- [ ] `internal/web/templates/static/app.css` has new `/* 11. SWIPE CARDS */` section appended (no existing rules modified)
- [ ] CSS includes breakpoint rules: `.job-card-deck { display: none }` at min-width 640px; `.job-table-desktop { display: none }` at max-width 639px
- [ ] All card classes from plan §5 are present in CSS: `.job-card`, `.job-card-active`, `.job-card-ghost-1`, `.job-card-ghost-2`, `.job-card-actions`, `.job-card-btn`, etc.
- [ ] `internal/web/templates/static/swipe-cards.js` created as an IIFE module
- [ ] JS uses Pointer Events (not Touch Events)
- [ ] JS commit thresholds: 100px absolute OR 35% card width OR 0.4px/ms velocity
- [ ] JS fly-off animation: `translateX(±110vw) rotate(±30deg)` then `submitAction`
- [ ] JS snap-back resets inline styles with spring easing
- [ ] JS checks `prefers-reduced-motion` and skips animation if set
- [ ] JS listens for `htmx:afterSwap` on `#job-card-deck` and re-runs `initDeck()` + `moveFocusAfterSwap()`
- [ ] JS listens for `DOMContentLoaded` to run initial `initDeck()`
- [ ] `dashboard.html` has `job-table-desktop` class added to `.job-table-wrapper` div
- [ ] `dashboard.html` has `<section id="job-card-deck" class="job-card-deck" hx-get="/partials/job-cards" hx-trigger="every 30s" hx-target="#job-card-deck" hx-swap="innerHTML" aria-label="Job review cards" aria-live="polite" aria-atomic="false">` inserted after the table polling div
- [ ] `dashboard.html` section contains `{{template "job_cards" .}}` for initial server-side render
- [ ] `dashboard.html` has `{{block "scripts" .}}<script src="/static/swipe-cards.js" defer></script>{{end}}` at end of `{{define "content"}}`

## Interface Contracts

This is a single-repo project. No cross-repo contracts.

The template must define exactly `"job_cards"` (used by the backend handler):
```go
// Data type passed to the template:
type jobRowsData struct {
    Jobs      []models.Job
    CSRFToken string
}
```

Key `models.Job` fields used in the template:
- `.ID` — string, used in form action URLs and `data-job-id`
- `.Title` — string
- `.Company` — string
- `.Status` — string, used in `status-badge status-{{.Status}}`
- `.Location` — string (may be empty)
- `.ExtractedSalary` — string (may be empty)
- `.Salary` — string (may be empty, fallback after ExtractedSalary)
- `.DiscoveredAt` — `time.Time`, formatted as `"Jan 2, 2006"` via Go template `.DiscoveredAt.Format "Jan 2, 2006"`
- `.Summary` — string (may be empty)

The backend route that serves this partial: `GET /partials/job-cards` (returns `text/html`).

The approve/reject POST endpoints:
- `POST /api/jobs/{id}/approve` — expects `HX-Target: job-card-deck` header (set by `hx-target` on form) to trigger `job_cards` re-render
- `POST /api/jobs/{id}/reject` — same

CSRF token field name: `gorilla.csrf.Token` (must match existing forms in codebase — verify by checking another form in `dashboard.html` or `job_rows.html`).

## Context

### File: `internal/web/templates/partials/job_cards.html`

Create this new file. Template must start with `{{define "job_cards"}}` and end with `{{end}}`.

Full markup from the plan (§6):
- Empty state block: `{{if not .Jobs}}` ... `{{end}}`
- Counter: `<p class="job-card-counter">{{len .Jobs}} job{{if gt (len .Jobs) 1}}s{{end}} to review</p>`
- Ghost card 2 (3rd job): `{{if gt (len .Jobs) 2}}{{with index .Jobs 2}}`
- Ghost card 1 (2nd job): `{{if gt (len .Jobs) 1}}{{with index .Jobs 1}}`
- Active card: `{{with index .Jobs 0}}` as `<article class="job-card job-card-active" role="article" aria-label="Job 1 of {{len $.Jobs}}: {{.Title}} at {{.Company}}" data-job-id="{{.ID}}" tabindex="0">`
- Action forms: use `hx-swap="innerHTML"` (NOT outerHTML — see plan §6 correction)
- CSRF: `<input type="hidden" name="gorilla.csrf.Token" value="{{$.CSRFToken}}">`

### File: `internal/web/templates/static/app.css`

Append to the end of the file. Do not modify any existing rules. Add a comment header:
```
/* =============================================================================
   11. SWIPE CARDS
   ============================================================================= */
```

Then add the full CSS from plan §5 (custom properties :root block, breakpoint show/hide, card deck container, card base styles, card interior elements, overlay badges, action buttons, counter/empty state, reduced-motion media query).

### File: `internal/web/templates/static/swipe-cards.js`

Create this new file as an IIFE. Key constants:
```js
const COMMIT_DIST = 100;       // px — absolute threshold
const COMMIT_RATIO = 0.35;     // fraction of card width
const COMMIT_VELOCITY = 0.4;   // px/ms
const OVERLAY_FULL = 80;       // px drag for full overlay opacity
```

Functions to implement:
- `initDeck()` — find `.job-card-active`, attach pointer event listeners
- `onPointerDown(e)` — record start coords, call `e.target.setPointerCapture(e.pointerId)`
- `onPointerMove(e)` — compute deltaX, apply `transform: translateX(${deltaX}px) rotate(${deltaX * 0.05}deg)`, update overlay opacity `Math.min(1, Math.abs(deltaX) / OVERLAY_FULL)`, show approve overlay when deltaX > 0, reject when deltaX < 0
- `onPointerUp(e)` — evaluate commit thresholds (distance, ratio, velocity), call `commitCard` or `snapBack`
- `onPointerCancel(e)` — call `snapBack`
- `commitCard(card, direction)` — check `prefers-reduced-motion`; if not reduced: apply fly-off `transform: translateX(${direction === 'approve' ? 110 : -110}vw) rotate(${direction === 'approve' ? 30 : -30}deg); opacity: 0` with CSS transition, listen for `transitionend`, then call `submitAction`; if reduced: call `submitAction` directly
- `submitAction(card, direction)` — find `form[data-action="${direction}"]` inside `#job-card-deck`, trigger via `htmx.trigger(form, 'submit')` or fallback to `form.submit()`
- `snapBack(card)` — reset `card.style.transform = ''` and `card.style.opacity = ''`, clear overlay opacities
- `expandSummary(card)` — toggle `.expanded` on the `.job-card-summary` element, update button text
- `moveFocusAfterSwap()` — focus `.job-card-active .job-card-link` or `.job-card-empty button`

Event listeners:
```js
document.addEventListener('htmx:afterSwap', function (e) {
    if (e.detail.target && e.detail.target.id === 'job-card-deck') {
        initDeck();
        moveFocusAfterSwap();
    }
});
document.addEventListener('DOMContentLoaded', initDeck);
```

### File: `internal/web/templates/dashboard.html`

Three changes:

**A — Add class to table wrapper:**
Find: `<div class="job-table-wrapper">`
Change to: `<div class="job-table-wrapper job-table-desktop">`

**B — Insert card deck section after the closing `</div>` of the outer HTMX polling div** (the one with `hx-get="/partials/job-table"`):
```html
<section id="job-card-deck"
         class="job-card-deck"
         hx-get="/partials/job-cards"
         hx-trigger="every 30s"
         hx-target="#job-card-deck"
         hx-swap="innerHTML"
         aria-label="Job review cards"
         aria-live="polite"
         aria-atomic="false">
  {{template "job_cards" .}}
</section>
```

**C — Add scripts block before `{{end}}` of `{{define "content"}}`:**
```html
{{block "scripts" .}}
<script src="/static/swipe-cards.js" defer></script>
{{end}}
```

Note: `layout.html` already has a `{{block "scripts" .}}{{end}}` in `<body>`. The one in `dashboard.html` overrides it for the dashboard page only.

## Notes

**Code Review**: done (2026-04-05)
**Verdict**: approve (1 warning, 3 info — no critical issues)

---

### Code Review Findings

#### [WARNING] `internal/web/templates/partials/job_cards.html` line 8–10 — Refresh button missing CSRF token for non-GET scenario (info-level but worth noting)

The Refresh button uses `hx-get="/partials/job-cards"` which is a GET request and does not require CSRF protection per gorilla/csrf's defaults. The `X-CSRF-Token` header is set on `document.body` by `layout.html` JS, so HTMX GET requests will send it automatically. This is correct behavior — no action needed. Flagged as a warning only because the button has no `<form>` wrapper and relies entirely on HTMX; if JS is disabled the button does nothing. For a JS-progressive-enhancement context this is intentional and acceptable per the spec.

#### [INFO] `internal/web/templates/dashboard.html` — `{{block "scripts" .}}` is placed outside the `{{if .User}}` block

The `{{block "scripts" .}}` (which loads `swipe-cards.js`) is emitted for all visitors — authenticated and unauthenticated alike. For unauthenticated users the card deck section is absent from the DOM, so `initDeck()` will find no `.job-card-active` and exit harmlessly. The JS file is a small static asset. Not a bug; just noted.

#### [INFO] `internal/web/templates/partials/job_cards.html` line 43 — `aria-label` on active card always says "Job 1 of N"

The `aria-label` is hardcoded as `"Job 1 of {{len $.Jobs}}: ..."` regardless of position. Since the template always renders only the first job as the active card (the rest are ghost cards), this is technically accurate — but it would be misleading if someone expects "Job 2 of N" after a swap (the server re-renders from scratch with the first remaining job as active). This is a design consequence, not a bug. The spec says "Job 1 of N" and the template implements that correctly.

#### [INFO] `internal/web/templates/static/swipe-cards.js` — `commitCard` does not pass `card` to `submitAction`

`commitCard(card, direction)` calls `submitAction(direction)` without passing `card`. `submitAction` then searches `#job-card-deck` for `form[data-action="${direction}"]`. This works correctly because the active card's forms are inside `#job-card-deck`. If multiple jobs were rendered with forms, the selector would find the first matching form, which is always the active card's form (since only the active card's forms have `data-action`). No bug.

#### [INFO] All acceptance criteria verified

- `{{define "job_cards"}}` template: confirmed.
- Empty state with Refresh button: confirmed (lines 2–11).
- Job counter: confirmed (line 13).
- Ghost cards (up to 2, aria-hidden, inert): confirmed.
- Active card `<article class="job-card job-card-active">` with `data-job-id`, `tabindex="0"`, correct aria-label: confirmed.
- Approve/reject overlay badges (aria-hidden): confirmed.
- Status badge, title link, company, location (conditional), salary (ExtractedSalary→Salary), discovered date, summary+expand: confirmed.
- Forms: `hx-target="#job-card-deck"` (no `#` issue — this is the correct HTMX value, the `#` is part of the CSS selector string used as the target value), `hx-swap="innerHTML"`, `data-action`, CSRF token: all confirmed.
- `app.css` section 11 appended (no existing rules modified), breakpoints, all card classes from plan §5: confirmed.
- `swipe-cards.js` IIFE, Pointer Events, correct thresholds (100px/35%/0.4px/ms), fly-off (±110vw, ±30deg), snap-back spring easing, `prefers-reduced-motion`, `htmx:afterSwap` on `#job-card-deck`, `DOMContentLoaded`: all confirmed.
- `dashboard.html`: `job-table-desktop` class added, `<section id="job-card-deck">` inserted with correct attributes and `{{template "job_cards" .}}`, `{{block "scripts"}}` with `swipe-cards.js`: confirmed.
- `{{template "job_cards" .}}` receives `dashboardData` which embeds `Jobs []models.Job` and `CSRFToken string` — the template only accesses `.Jobs` and `.CSRFToken`, both present in `dashboardData`. Confirmed compatible.

**Summary**: 0 critical, 1 warning (info-level JS progressive enhancement note), 3 info. Frontend implementation is correct and complete against the spec.

---

**Completed by Coder agent — 2026-04-05**

Branch: `feature/tinder-mobile-frontend`

### What was implemented

**New files:**
- `internal/web/templates/partials/job_cards.html` — defines `{{define "job_cards"}}` template with: empty state (with Refresh button), job counter, ghost card depth effect (up to 2 ghost cards), active card with swipe overlay badges, status badge, title link, company, location (conditional), salary (ExtractedSalary → Salary fallback), discovered date, summary with expand button. Approve and reject forms use `hx-target="#job-card-deck"` and `hx-swap="innerHTML"` with CSRF token.
- `internal/web/templates/static/swipe-cards.js` — IIFE module using Pointer Events. Implements: `initDeck`, `onPointerDown/Move/Up/Cancel`, `commitCard` (fly-off: `±110vw rotate(±30deg)`), `submitAction` (HTMX or fallback), `snapBack` (spring easing), `expandSummary`, `moveFocusAfterSwap`. Commit thresholds: 100px absolute OR 35% card width OR 0.4px/ms velocity. Checks `prefers-reduced-motion`. Listens for `htmx:afterSwap` on `#job-card-deck` and `DOMContentLoaded`.

**Modified files:**
- `internal/web/templates/static/app.css` — appended `/* 11. SWIPE CARDS */` section with custom properties `:root` block, breakpoint show/hide rules, deck container, card base, ghost card styles, card interior elements, overlay badges, action buttons, counter/empty state, reduced-motion media query.
- `internal/web/templates/dashboard.html` — added `job-table-desktop` class to `.job-table-wrapper`, inserted `<section id="job-card-deck">` with `{{template "job_cards" .}}` after the polling div, added `{{block "scripts" .}}<script src="/static/swipe-cards.js" defer></script>{{end}}` before `{{end}}` of `{{define "content"}}`.

### Test results

Go is not installed in the container; `go test ./...` could not be run. The task is pure frontend (HTML/CSS/JS) with no Go code changes. All acceptance criteria were verified by code review against the plan spec.
