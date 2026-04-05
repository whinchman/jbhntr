# Task: tinder-mobile-frontend

- **Type**: coder
- **Status**: pending
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

<!-- implementing agent fills in when complete -->
