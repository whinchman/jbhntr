# Plan: Modern Design Refresh

**Feature**: Update design look to be more modern  
**Date**: 2026-04-03  
**Status**: Ready for implementation

---

## Architecture Overview

### Current State

JobHuntr uses [PicoCSS v2](https://picocss.com/) loaded from a CDN in every template. The templates are server-rendered HTML using Go's `html/template`, with HTMX for dynamic partial updates. There is no build pipeline, no bundler, and no static asset directory — all CSS today comes from the Pico CDN link and small `<style>` blocks inline in individual template files.

The template hierarchy is:
- `internal/web/templates/layout.html` — the shared shell (nav, head, CSS imports)
- `internal/web/templates/login.html` — standalone page (its own `<head>`)
- `internal/web/templates/dashboard.html` — job table + landing hero
- `internal/web/templates/job_detail.html` — single job view
- `internal/web/templates/settings.html` — filters, notifications, resume
- `internal/web/templates/profile.html` — user account
- `internal/web/templates/onboarding.html` — first-time welcome
- `internal/web/templates/partials/job_rows.html` — HTMX-swapped table rows

All templates are embedded into the binary via `//go:embed templates` in `internal/web/server.go`. A static CSS file placed inside `templates/` will be embedded automatically.

### Chosen Approach: Single Custom CSS File + Keep Pico (Scoped Override)

**Decision**: Keep PicoCSS v2 as the base (it already handles reset, form normalization, accessible defaults) and layer a single custom stylesheet `templates/static/app.css` on top. This file overrides Pico CSS custom properties (variables) and adds component-specific styles.

**Why not switch to Tailwind**: No build step exists. Adding a PostCSS/Node pipeline for a Go project with no JS framework is disproportionate scope. The constraint says "no bundler".

**Why not drop Pico and go fully custom**: Pico provides accessible form controls, button normalisation, and a decent reset. Replacing that from scratch is significant work with no clear quality gain.

**Why not use another lightweight lib (e.g. MVP.css, Water.css)**: PicoCSS v2 already ships CSS custom properties for theming, making it the lowest-effort path to a polished result.

**Approach**: Override Pico's CSS custom properties (`:root { --pico-* }`) to apply a contemporary palette and type scale, then write focused component classes for nav, status badges, cards, tables, buttons, and forms. No inline styles remain after the refresh (they are extracted into `app.css`).

---

## Design System

### Colour Palette

A neutral-base, indigo-accent palette — clean, professional, readable.

```css
/* Design tokens */
--color-bg:           #f8f9fb;   /* page background */
--color-surface:      #ffffff;   /* card/table surface */
--color-surface-alt:  #f1f3f7;   /* muted surfaces, zebra */
--color-border:       #e2e5ec;   /* dividers, table borders */
--color-text:         #1a1d23;   /* primary text */
--color-text-muted:   #6b7280;   /* secondary/placeholder text */
--color-accent:       #4f46e5;   /* indigo-600 — links, primary buttons */
--color-accent-hover: #4338ca;   /* indigo-700 */
--color-accent-subtle:#eef2ff;   /* indigo-50 — active tab bg */
--color-success:      #16a34a;   /* green-600 */
--color-success-bg:   #dcfce7;   /* green-100 */
--color-warning:      #d97706;   /* amber-600 */
--color-warning-bg:   #fef3c7;   /* amber-100 */
--color-danger:       #dc2626;   /* red-600 */
--color-danger-bg:    #fee2e2;   /* red-100 */
--color-info:         #2563eb;   /* blue-600 */
--color-info-bg:      #dbeafe;   /* blue-100 */
--color-purple:       #7c3aed;   /* violet-600 */
--color-purple-bg:    #ede9fe;   /* violet-100 */
--color-teal:         #0d9488;   /* teal-600 */
--color-teal-bg:      #ccfbf1;   /* teal-100 */
```

### Typography

Use the system font stack for body text (already Pico's default). Override font sizes to use a tighter, more modern scale.

```css
--font-sans: ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont,
             "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
--font-mono: ui-monospace, "Cascadia Code", "Source Code Pro", Menlo,
             Consolas, "DejaVu Sans Mono", monospace;

/* Type scale (rem) */
--text-xs:   0.75rem;   /* 12px */
--text-sm:   0.875rem;  /* 14px */
--text-base: 1rem;      /* 16px */
--text-lg:   1.125rem;  /* 18px */
--text-xl:   1.25rem;   /* 20px */
--text-2xl:  1.5rem;    /* 24px */
--text-3xl:  1.875rem;  /* 30px */
```

### Spacing

Use an 8 px grid. Key values:
- `--space-1: 0.25rem` (4px)
- `--space-2: 0.5rem`  (8px)
- `--space-3: 0.75rem` (12px)
- `--space-4: 1rem`    (16px)
- `--space-6: 1.5rem`  (24px)
- `--space-8: 2rem`    (32px)

### Border Radius

Consistent rounding:
- `--radius-sm: 0.375rem` (6px) — badges, small elements
- `--radius-md: 0.5rem`   (8px) — inputs, buttons, cards
- `--radius-lg: 0.75rem`  (12px) — modals, big cards

---

## File Changes

### New File: `internal/web/templates/static/app.css`

The single source of truth for all custom styles. Embedded via the existing `//go:embed templates` directive — no server.go changes needed for the file to be served as a static asset IF a static route is added (see Step 2 below).

**Sections inside app.css:**
1. Design tokens (`:root` custom properties)
2. Pico override mappings (maps `--pico-*` vars to our tokens)
3. Layout (body, `.app-container`, nav)
4. Navigation (`.app-nav`)
5. Status badges (`.status-badge` and each `status-*` variant)
6. Tab bar (`.tab-bar`)
7. Table styles (`.job-table`)
8. Forms (inputs, labels, buttons)
9. Cards (`.card`)
10. Alert/flash messages (`.alert`, `.alert-success`, `.alert-danger`)
11. Hero section (`.hero`)
12. Utility classes (`.text-muted`, `.sr-only`)

### Modified: `internal/web/server.go`

Add a static file route to serve `templates/static/` at `/static/`:

```go
// In Handler(), after existing middleware:
staticFS, _ := fs.Sub(templateFS, "templates/static")
r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))
```

`templateFS` is already declared as `//go:embed templates` at package level, so `fs.Sub` works without any additional embed directive.

### Modified: `internal/web/templates/layout.html`

1. Replace Pico CDN link with two links: Pico CDN (keep) + `/static/app.css`
2. Remove all inline `<style>` block (move to `app.css`)
3. Replace inline `style=""` attributes on nav elements with classes
4. Add `class="app-nav"` to `<nav>`
5. Update body to use `class="app-body"` and container to `class="app-container"`

### Modified: `internal/web/templates/login.html`

1. Add `/static/app.css` link (login has its own `<head>`)
2. Remove inline `<style>` block — replace with classes from `app.css`
3. Replace `<article class="login-card">` structure with cleaner card markup

### Modified: `internal/web/templates/dashboard.html`

1. Add `class="job-table"` to `<table>`
2. Remove inline `style` attributes from hero section — use `.hero` class
3. Tab bar buttons: remove inline `role="button"` styling variants, use tab-specific classes

### Modified: `internal/web/templates/job_detail.html`

1. Remove inline `style=""` attributes on description `<pre>`, document borders
2. Add classes: `.job-description`, `.document-preview`
3. Status section: remove inline button `style` attributes, use `.btn-sm`

### Modified: `internal/web/templates/settings.html`

1. Remove inline `style="background:#e8f5e9…"` alert — use `.alert .alert-success`
2. Remove inline `style="color:#999"` — use `.text-muted`
3. Grid layout for "Add Filter" form: move inline `style="display:grid…"` to `.filter-form-grid` class
4. Remove inline `style` on textarea, buttons

### Modified: `internal/web/templates/profile.html`

1. Remove inline alert `style=""` — use `.alert .alert-success` / `.alert .alert-danger`
2. Remove inline `style=""` on display-name flex row — use `.inline-form`
3. Remove inline `style=""` on avatar image — use `.avatar`

### Modified: `internal/web/templates/onboarding.html`

1. Remove inline `style="color:var(--pico-color-red-500)"` — use `.text-danger`

### Modified: `internal/web/templates/partials/job_rows.html`

1. Remove inline `style="text-align:center;color:#999"` on empty row — use classes
2. Remove inline `style` on summary row — use `.job-summary-row`
3. Remove inline button `style=""` on action buttons — use `.btn-sm`

---

## Component Specifications

### Navigation (`.app-nav`)

```
┌──────────────────────────────────────────────────────────────┐
│  JobHuntr   Jobs · Settings · Profile        Avatar  Name ·  │
│                                                   Sign out   │
└──────────────────────────────────────────────────────────────┘
```

- Sticky top, white background, subtle bottom border
- Brand name in semibold indigo
- Nav links with hover underline
- Right side: avatar (32px circle) + display name + sign-out link
- No heavy box shadow — just a 1px border-bottom

### Tab Bar (`.tab-bar`)

- Pills style, not square buttons
- Active tab: indigo background (solid), white text
- Inactive: transparent background, muted border, text-color text
- Hover: slight background tint
- Gap: 0.5rem; no underlines

### Job Table (`.job-table`)

- No external borders on the table itself
- Subtle horizontal dividers between rows (1px `--color-border`)
- Header row: `--color-surface-alt` background, uppercase `--text-xs` label, `--color-text-muted` color
- Sortable columns: caret indicator, cursor pointer
- Row hover: `--color-surface-alt` background transition
- Status badge inline in Status column
- Action buttons: small, pill-shaped, outline style

### Status Badges

All badges use the same base `.status-badge` class:
- `border-radius: var(--radius-sm)`
- `font-size: var(--text-xs)`
- `font-weight: 600`
- `padding: 2px 8px`
- `letter-spacing: 0.03em`
- `text-transform: uppercase`

Colour mappings:
| Status      | Background          | Text colour         |
|-------------|---------------------|---------------------|
| discovered  | `--color-info-bg`   | `--color-info`      |
| notified    | `--color-purple-bg` | `--color-purple`    |
| approved    | `--color-success-bg`| `--color-success`   |
| rejected    | `--color-danger-bg` | `--color-danger`    |
| generating  | `--color-warning-bg`| `--color-warning`   |
| complete    | `--color-teal-bg`   | `--color-teal`      |
| failed      | `--color-danger-bg` | `--color-danger`    |

### Forms

- Labels: `--text-sm`, semibold, `--color-text`, `margin-bottom: 4px`
- Inputs: `border: 1px solid --color-border`, `border-radius: --radius-md`, `padding: 0.5rem 0.75rem`, focus ring `2px solid --color-accent` with 2px offset
- Textareas: same as inputs
- Button primary: `background: --color-accent`, white text, `border-radius: --radius-md`, hover `--color-accent-hover`
- Button outline/secondary: transparent background, `border: 1px solid --color-border`, hover `--color-surface-alt`
- Button small (`.btn-sm`): `padding: 0.2rem 0.6rem`, `font-size: --text-xs`

### Alert / Flash Messages

Two variants `.alert.alert-success` and `.alert.alert-danger`:
- `border-radius: --radius-md`
- `padding: 0.75rem 1rem`
- Left accent border (`border-left: 3px solid`)
- No box shadow

### Hero (Landing / Unauthenticated Dashboard)

```css
.hero {
  text-align: center;
  padding: 5rem 1rem;
}
.hero h1 { font-size: var(--text-3xl); font-weight: 700; }
.hero p  { max-width: 40rem; margin: 1rem auto; color: var(--color-text-muted); font-size: var(--text-lg); }
```

### Login Card

- Centered on page (flex column centering)
- White card, `border-radius: --radius-lg`, subtle box-shadow
- Max-width 400px
- Provider buttons full-width, pill shape

---

## Step-by-Step Implementation

### Step 1 — Create `app.css`

**Agent**: coder  
**File**: `internal/web/templates/static/app.css`

Create the directory `internal/web/templates/static/` and write `app.css` containing:
1. All design token custom properties under `:root`
2. Pico override mappings that remap `--pico-*` variables to our tokens
3. All component styles as described in the component specifications above
4. No inline styles — every class needed by the templates is defined here

### Step 2 — Add static file route to `server.go`

**Agent**: coder  
**File**: `internal/web/server.go`

Import `io/fs` (already available in stdlib) and add a static file handler in `Handler()`:

```go
import "io/fs"

// inside Handler():
staticFS, err := fs.Sub(templateFS, "templates/static")
if err != nil {
    panic(err)
}
r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))
```

### Step 3 — Update `layout.html`

**Agent**: coder  
**File**: `internal/web/templates/layout.html`

- Add `<link rel="stylesheet" href="/static/app.css">` after Pico link
- Remove the entire `<style>` block (its content moves to `app.css`)
- Add class `app-nav` to `<nav>`
- Replace inline `style` attributes in nav with semantic CSS classes

### Step 4 — Update `login.html`

**Agent**: coder  
**File**: `internal/web/templates/login.html`

- Add `/static/app.css` link
- Replace inline `<style>` block with class references
- Replace `<article class="login-card">` markup with updated class names

### Step 5 — Update `dashboard.html`, `job_detail.html`, `partials/job_rows.html`

**Agent**: coder  
**Files**:
- `internal/web/templates/dashboard.html`
- `internal/web/templates/job_detail.html`
- `internal/web/templates/partials/job_rows.html`

- Dashboard: add `class="job-table"` to table; add `.hero` to unauthenticated section
- Job detail: replace inline styles with `.job-description`, `.document-preview`, `.btn-sm`
- Job rows: replace inline styles on empty row, summary row, action buttons

### Step 6 — Update `settings.html`, `profile.html`, `onboarding.html`

**Agent**: coder  
**Files**:
- `internal/web/templates/settings.html`
- `internal/web/templates/profile.html`
- `internal/web/templates/onboarding.html`

- Settings: `.alert.alert-success`, `.text-muted`, `.filter-form-grid`, remove inline styles
- Profile: `.alert`, `.inline-form`, `.avatar`, remove inline styles
- Onboarding: `.text-danger` for error text

---

## Trade-offs and Alternatives

### Alternative A: Full Tailwind CSS Integration

**Pros**: Utility-first, no custom class naming, consistent spacing, easy iteration.  
**Cons**: Requires a PostCSS/Node build step; contradicts the "no bundler" constraint; adds `node_modules` to a pure Go project; CDN-only Tailwind (Play CDN) is not production-appropriate.  
**Verdict**: Rejected.

### Alternative B: Drop PicoCSS, Write Fully Custom CSS

**Pros**: Complete control, no unused CSS from Pico.  
**Cons**: Must rebuild form normalisation, button resets, accessible focus states from scratch; higher effort with no material quality difference.  
**Verdict**: Rejected.

### Alternative C: Switch to a Different Lightweight Lib (e.g. Chota, Classless)

**Pros**: Fresh start, might better fit the aesthetic goal.  
**Cons**: Requires auditing all templates against new class names; Pico v2 already ships CSS custom properties making it essentially an existing "design token" system.  
**Verdict**: Rejected — Pico override approach is lower risk.

### Chosen: Pico v2 + Custom Override CSS

**Pros**: Minimal code changes, no build tooling, leverages existing Pico variable system, all overrides in one file, easy to maintain.  
**Cons**: Pico's opinionated defaults may resist some overrides (requires `!important` in a few places or higher-specificity selectors).  
**Risk mitigation**: All Pico overrides are done via CSS custom property reassignment, not class overrides, which is the documented way to theme Pico v2.

---

## Acceptance Criteria

- [ ] A single `/static/app.css` file exists and is served by the Go binary (no CDN dependency for custom styles)
- [ ] The page background uses `#f8f9fb` (neutral off-white), not stark white
- [ ] Navigation bar has sticky positioning, a 1px bottom border, and the brand name is visually distinct
- [ ] Status badges use the specified colour mapping with uppercase lettering and pill shape
- [ ] Tab bar uses pill-style tabs with an indigo active state
- [ ] Job table has sortable column headers with hover states; no outer border on the table element itself
- [ ] Form inputs have a visible focus ring using the indigo accent colour
- [ ] Buttons have consistent border-radius and hover transitions
- [ ] Alert messages (flash save confirmations, errors) use the `.alert` pattern with a left accent border
- [ ] The login page is centred with a card shadow and full-width provider buttons
- [ ] No inline `style=""` attributes remain in any template file (except dynamically-generated content)
- [ ] All existing HTMX functionality continues to work (no JS changes required)
- [ ] The app renders correctly on mobile viewports (≥320px wide)

---

## Dependencies and Prerequisites

- No new Go dependencies
- No new npm/Node dependencies
- No database migrations
- No environment changes
- The `internal/web/templates/static/` directory must be created before embedding; Go's `//go:embed` will include it automatically

---

## Notes for Manager Agent

This feature should be decomposed into **two parallel-capable tasks** after Step 1:

1. **Task: create-app-css** (coder) — Steps 1 and 2 (new CSS file + static route). This is a prerequisite for all other tasks.
2. **Task: update-layout-login** (coder) — Steps 3 and 4 (layout + login templates). Depends on task 1.
3. **Task: update-content-templates** (coder) — Steps 5 and 6 (all content templates). Depends on task 1. Can run in parallel with task 2.

Tasks 2 and 3 can run in parallel once task 1 is complete.

Recommended QA check: visual spot-check of all 6 template routes after implementation. The QA agent should also verify no inline style attributes remain (grep check).
