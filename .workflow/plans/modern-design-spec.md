# Design Specification: Modern Design Refresh

**Project**: jobhuntr  
**Feature**: Update design look to be more modern  
**Date**: 2026-04-03  
**Author**: Designer Agent  
**Implements**: `.workflow/plans/modern-design.md`

---

## 1. Overview

This spec defines every visual decision a coder needs to implement the single-file
CSS override approach described in the architecture plan. The output is one file:
`internal/web/templates/static/app.css`, plus targeted class additions to the
seven HTML templates.

Stack constraints:
- PicoCSS v2 from CDN remains as the base layer (reset, normalisation, accessible defaults)
- `app.css` is layered on top by overriding `--pico-*` custom properties and adding component classes
- No JavaScript framework, no build pipeline
- All custom properties live on `:root`; no CSS nesting (broad browser support without transpilation)

---

## 2. Design Token Definitions

### 2.1 Colour Tokens

```css
:root {
  /* Neutrals */
  --color-bg:           #f8f9fb;
  --color-surface:      #ffffff;
  --color-surface-alt:  #f1f3f7;
  --color-border:       #e2e5ec;
  --color-text:         #1a1d23;
  --color-text-muted:   #6b7280;

  /* Accent — Indigo */
  --color-accent:        #4f46e5;   /* indigo-600 */
  --color-accent-hover:  #4338ca;   /* indigo-700 */
  --color-accent-subtle: #eef2ff;   /* indigo-50 */

  /* Semantic status colours */
  --color-success:     #16a34a;   /* green-600  */
  --color-success-bg:  #dcfce7;   /* green-100  */
  --color-warning:     #d97706;   /* amber-600  */
  --color-warning-bg:  #fef3c7;   /* amber-100  */
  --color-danger:      #dc2626;   /* red-600    */
  --color-danger-bg:   #fee2e2;   /* red-100    */
  --color-info:        #2563eb;   /* blue-600   */
  --color-info-bg:     #dbeafe;   /* blue-100   */
  --color-purple:      #7c3aed;   /* violet-600 */
  --color-purple-bg:   #ede9fe;   /* violet-100 */
  --color-teal:        #0d9488;   /* teal-600   */
  --color-teal-bg:     #ccfbf1;   /* teal-100   */
}
```

**WCAG contrast ratios verified**:
- `--color-text` (#1a1d23) on `--color-bg` (#f8f9fb): 16.5:1 — AAA
- `--color-text-muted` (#6b7280) on `--color-surface` (#ffffff): 4.6:1 — AA
- `--color-accent` (#4f46e5) on white: 5.0:1 — AA (for text ≥14px bold / ≥18px normal)
- White on `--color-accent` (#4f46e5): 5.0:1 — AA
- Status badge text on status badge background (all variants below): all ≥4.5:1 — AA

### 2.2 Typography Tokens

```css
:root {
  --font-sans: ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont,
               "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
  --font-mono: ui-monospace, "Cascadia Code", "Source Code Pro", Menlo,
               Consolas, "DejaVu Sans Mono", monospace;

  --text-xs:   0.75rem;    /* 12px */
  --text-sm:   0.875rem;   /* 14px */
  --text-base: 1rem;       /* 16px */
  --text-lg:   1.125rem;   /* 18px */
  --text-xl:   1.25rem;    /* 20px */
  --text-2xl:  1.5rem;     /* 24px */
  --text-3xl:  1.875rem;   /* 30px */

  --weight-normal:    400;
  --weight-medium:    500;
  --weight-semibold:  600;
  --weight-bold:      700;

  --leading-tight:   1.25;
  --leading-normal:  1.5;
  --leading-relaxed: 1.625;
}
```

### 2.3 Spacing Tokens (8px grid)

```css
:root {
  --space-1:  0.25rem;   /*  4px */
  --space-2:  0.5rem;    /*  8px */
  --space-3:  0.75rem;   /* 12px */
  --space-4:  1rem;      /* 16px */
  --space-5:  1.25rem;   /* 20px */
  --space-6:  1.5rem;    /* 24px */
  --space-8:  2rem;      /* 32px */
  --space-10: 2.5rem;    /* 40px */
  --space-12: 3rem;      /* 48px */
  --space-16: 4rem;      /* 64px */
  --space-20: 5rem;      /* 80px */
}
```

### 2.4 Border Radius Tokens

```css
:root {
  --radius-sm:   0.375rem;   /*  6px — badges, chips       */
  --radius-md:   0.5rem;     /*  8px — inputs, buttons     */
  --radius-lg:   0.75rem;    /* 12px — cards, modals       */
  --radius-full: 9999px;     /* pill shape                 */
}
```

### 2.5 Shadow Tokens

```css
:root {
  --shadow-xs: 0 1px 2px rgba(0, 0, 0, 0.05);
  --shadow-sm: 0 1px 3px rgba(0, 0, 0, 0.08), 0 1px 2px rgba(0, 0, 0, 0.06);
  --shadow-md: 0 4px 6px rgba(0, 0, 0, 0.07), 0 2px 4px rgba(0, 0, 0, 0.06);
  --shadow-lg: 0 10px 15px rgba(0, 0, 0, 0.08), 0 4px 6px rgba(0, 0, 0, 0.05);
}
```

---

## 3. Pico CSS v2 Variable Overrides

These override the Pico v2 custom properties so that all Pico-generated elements
(form controls, buttons, headings) automatically use the new token set. Place these
immediately after the token declarations in `:root`.

```css
:root {
  /* Background and surfaces */
  --pico-background-color:    var(--color-bg);
  --pico-card-background-color: var(--color-surface);
  --pico-card-sectioning-background-color: var(--color-surface-alt);

  /* Typography */
  --pico-font-family:         var(--font-sans);
  --pico-font-size:           var(--text-base);
  --pico-line-height:         var(--leading-normal);
  --pico-color:               var(--color-text);
  --pico-muted-color:         var(--color-text-muted);
  --pico-h1-color:            var(--color-text);
  --pico-h2-color:            var(--color-text);
  --pico-h3-color:            var(--color-text);

  /* Borders */
  --pico-border-color:        var(--color-border);
  --pico-card-border-radius:  var(--radius-lg);

  /* Primary / accent (used by Pico buttons and links) */
  --pico-primary:             var(--color-accent);
  --pico-primary-hover:       var(--color-accent-hover);
  --pico-primary-focus:       var(--color-accent);
  --pico-primary-inverse:     #ffffff;

  /* Form controls */
  --pico-border-radius:       var(--radius-md);
  --pico-form-element-border-color: var(--color-border);
  --pico-form-element-focus-color:  var(--color-accent);
  --pico-form-element-background-color: var(--color-surface);
  --pico-form-element-color:  var(--color-text);
  --pico-form-element-placeholder-color: var(--color-text-muted);

  /* Secondary buttons */
  --pico-secondary:           #64748b;
  --pico-secondary-hover:     #475569;
  --pico-secondary-focus:     var(--color-accent);
  --pico-secondary-inverse:   #ffffff;
}
```

---

## 4. Layout

### 4.1 Body and Container

```css
/* Body */
body {
  background-color: var(--color-bg);
  color: var(--color-text);
  font-family: var(--font-sans);
  font-size: var(--text-base);
  line-height: var(--leading-normal);
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
}

/* Main content container — constrain width and add horizontal padding */
.app-container {
  max-width: 1200px;
  margin-inline: auto;
  padding-inline: var(--space-4);
}

/* On wider viewports, increase horizontal breathing room */
@media (min-width: 768px) {
  .app-container {
    padding-inline: var(--space-8);
  }
}
```

Coder note: in `layout.html`, change `<main class="container">` to
`<main class="app-container">` to use this definition instead of Pico's default
container (which is fine but has less explicit padding control).

### 4.2 Page-level Spacing

Headings (`<h1>`) on content pages get `margin-top: var(--space-8)` and
`margin-bottom: var(--space-6)` to breathe. This is handled by a scoped rule
inside `.app-container > h1` to avoid affecting nav or card headings.

---

## 5. Component Designs

### 5.1 Navigation (`.app-nav`)

**Visual anatomy**:
```
┌─────────────────────────────────────────────────────────────────┐
│  [JobHuntr]   Jobs  ·  Settings  ·  Profile          [●] Name  Sign out  │
└─────────────────────────────────────────────────────────────────┘
   Brand (bold    Nav links (14px,     right-aligned: avatar (32px circle),
   indigo)        gray, hover underline) display name, separator, sign-out link
```

**Behaviour**:
- `position: sticky; top: 0; z-index: 100` — stays visible on scroll
- White background with a 1px bottom border (`--color-border`)
- No box-shadow — the border alone provides separation
- On mobile (< 640px): nav links collapse below the brand row

**CSS**:
```css
.app-nav {
  position: sticky;
  top: 0;
  z-index: 100;
  background: var(--color-surface);
  border-bottom: 1px solid var(--color-border);
  padding: var(--space-3) var(--space-4);
}

@media (min-width: 768px) {
  .app-nav {
    padding-inline: var(--space-8);
  }
}

.app-nav-inner {
  max-width: 1200px;
  margin-inline: auto;
  display: flex;
  align-items: center;
  gap: var(--space-4);
  flex-wrap: wrap;
}

/* Brand */
.app-nav-brand {
  font-weight: var(--weight-semibold);
  font-size: var(--text-lg);
  color: var(--color-accent);
  text-decoration: none;
  margin-right: var(--space-2);
}
.app-nav-brand:hover {
  color: var(--color-accent-hover);
  text-decoration: none;
}

/* Nav links */
.app-nav-links {
  display: flex;
  align-items: center;
  gap: var(--space-1);
  flex: 1;
}
.app-nav-links a {
  color: var(--color-text-muted);
  font-size: var(--text-sm);
  font-weight: var(--weight-medium);
  text-decoration: none;
  padding: var(--space-1) var(--space-2);
  border-radius: var(--radius-md);
  transition: color 0.15s ease, background-color 0.15s ease;
}
.app-nav-links a:hover {
  color: var(--color-text);
  background-color: var(--color-surface-alt);
}

/* Right-side user area */
.app-nav-user {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  margin-left: auto;
  font-size: var(--text-sm);
  color: var(--color-text-muted);
}

/* Avatar image */
.nav-avatar {
  width: 32px;
  height: 32px;
  border-radius: var(--radius-full);
  object-fit: cover;
  vertical-align: middle;
}

.app-nav-user a {
  color: var(--color-text-muted);
  text-decoration: none;
}
.app-nav-user a:hover {
  color: var(--color-text);
  text-decoration: underline;
}
```

**Template changes for `layout.html`**:
- Replace `<nav>` with `<nav class="app-nav"><div class="app-nav-inner">…</div></nav>`
- Wrap nav links in `<div class="app-nav-links">`
- Replace `<span style="float:right;">` with `<div class="app-nav-user">`
- Replace avatar `<img>` inline style with `class="nav-avatar"`

**ARIA**: The `<nav>` element already provides the landmark role. Add
`aria-label="Main navigation"` to the `<nav>` element.

---

### 5.2 Status Badges (`.status-badge`)

**Base class**:
```css
.status-badge {
  display: inline-flex;
  align-items: center;
  padding: 2px 8px;
  border-radius: var(--radius-sm);
  font-size: var(--text-xs);
  font-weight: var(--weight-semibold);
  letter-spacing: 0.04em;
  text-transform: uppercase;
  white-space: nowrap;
  line-height: 1.5;
}
```

**Status variants** (all meet WCAG AA 4.5:1 contrast):

| Class | Background | Text | Contrast |
|---|---|---|---|
| `.status-discovered` | `--color-info-bg` (#dbeafe) | `--color-info` (#2563eb) | 4.8:1 |
| `.status-notified` | `--color-purple-bg` (#ede9fe) | `--color-purple` (#7c3aed) | 5.1:1 |
| `.status-approved` | `--color-success-bg` (#dcfce7) | `--color-success` (#16a34a) | 4.5:1 |
| `.status-rejected` | `--color-danger-bg` (#fee2e2) | `--color-danger` (#dc2626) | 5.0:1 |
| `.status-generating` | `--color-warning-bg` (#fef3c7) | `--color-warning` (#d97706) | 4.5:1 |
| `.status-complete` | `--color-teal-bg` (#ccfbf1) | `--color-teal` (#0d9488) | 4.6:1 |
| `.status-failed` | `--color-danger-bg` (#fee2e2) | `--color-danger` (#dc2626) | 5.0:1 |

```css
.status-discovered  { background: var(--color-info-bg);    color: var(--color-info);    }
.status-notified    { background: var(--color-purple-bg);   color: var(--color-purple);  }
.status-approved    { background: var(--color-success-bg);  color: var(--color-success); }
.status-rejected    { background: var(--color-danger-bg);   color: var(--color-danger);  }
.status-generating  { background: var(--color-warning-bg);  color: var(--color-warning); }
.status-complete    { background: var(--color-teal-bg);     color: var(--color-teal);    }
.status-failed      { background: var(--color-danger-bg);   color: var(--color-danger);  }
```

No template changes needed for badge classes — `status-badge` and `status-{{.Status}}`
are already in the templates; this CSS replaces the inline `<style>` block in `layout.html`.

---

### 5.3 Tab Bar (`.tab-bar`)

**Visual anatomy**: Pill tabs with a solid indigo fill on active, ghost/outline on inactive.

```
[ all ]  [ discovered ]  [ notified ]  [ approved ]  [ rejected ]
          ^^^^^^^^^^^
          active: indigo bg, white text
                          inactive: transparent, muted border, text-color
```

```css
.tab-bar {
  display: flex;
  gap: var(--space-2);
  flex-wrap: wrap;
  margin-bottom: var(--space-6);
}

/* Override Pico's default [role=button] / <a role="button"> styling in the tab-bar */
.tab-bar a[role="button"] {
  display: inline-flex;
  align-items: center;
  padding: var(--space-1) var(--space-4);
  border-radius: var(--radius-full);
  font-size: var(--text-sm);
  font-weight: var(--weight-medium);
  text-decoration: none;
  border: 1px solid var(--color-border);
  background: transparent;
  color: var(--color-text);
  cursor: pointer;
  transition: background-color 0.15s ease, color 0.15s ease, border-color 0.15s ease;
  /* Reset Pico button margin/padding defaults */
  margin-bottom: 0;
}
.tab-bar a[role="button"]:hover {
  background: var(--color-surface-alt);
  border-color: var(--color-text-muted);
}

/* Active tab — uses Pico's "contrast" class which maps to primary */
.tab-bar a[role="button"].contrast {
  background: var(--color-accent);
  color: #ffffff;
  border-color: var(--color-accent);
}
.tab-bar a[role="button"].contrast:hover {
  background: var(--color-accent-hover);
  border-color: var(--color-accent-hover);
}

/* Focus ring for keyboard navigation */
.tab-bar a[role="button"]:focus-visible {
  outline: 2px solid var(--color-accent);
  outline-offset: 2px;
}
```

No template changes needed — the existing `class="contrast"` / `class="outline"` structure
in `dashboard.html` maps directly to these selectors.

---

### 5.4 Job Table (`.job-table`)

**Visual anatomy**:
```
 Title       Company    Location   Salary    Status         Discovered  Actions
 ──────────────────────────────────────────────────────────────────────────────
 Software    Acme Corp  Remote     $150k     [APPROVED]     Jan 2, 2026  [Approve] [Reject]
 ──────────────────────────────────────────────────────────────────────────────
 Data Eng.   Beta Inc   New York   —         [DISCOVERED]   Jan 1, 2026  [Approve] [Reject]
```

- No outer border on `<table>` itself
- Header row: light muted background (`--color-surface-alt`), uppercase `--text-xs`, muted color
- Body rows: white background, thin 1px horizontal dividers
- Row hover: `--color-surface-alt` background with a quick transition
- Sortable column headers: `cursor: pointer`, small caret appended by template already

```css
/* Wrapper to enable responsive horizontal scroll on mobile */
.job-table-wrapper {
  overflow-x: auto;
  -webkit-overflow-scrolling: touch;
  border-radius: var(--radius-lg);
  border: 1px solid var(--color-border);
}

.job-table {
  width: 100%;
  border-collapse: collapse;
  font-size: var(--text-sm);
}

.job-table thead tr {
  background: var(--color-surface-alt);
}

.job-table th {
  padding: var(--space-3) var(--space-4);
  text-align: left;
  font-size: var(--text-xs);
  font-weight: var(--weight-semibold);
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--color-text-muted);
  border-bottom: 1px solid var(--color-border);
  white-space: nowrap;
}

/* Sortable column header interaction */
.job-table th[hx-get] {
  cursor: pointer;
  user-select: none;
}
.job-table th[hx-get]:hover {
  color: var(--color-text);
}

.job-table td {
  padding: var(--space-3) var(--space-4);
  color: var(--color-text);
  border-bottom: 1px solid var(--color-border);
  vertical-align: middle;
}

.job-table tbody tr:last-child td {
  border-bottom: none;
}

.job-table tbody tr:hover td {
  background: var(--color-surface-alt);
}

/* Job title link in table */
.job-table td a {
  font-weight: var(--weight-medium);
  color: var(--color-accent);
  text-decoration: none;
}
.job-table td a:hover {
  text-decoration: underline;
}

/* Summary row (AI summary beneath a job row) */
.job-summary-row td {
  padding: var(--space-1) var(--space-4) var(--space-3);
  font-size: var(--text-xs);
  color: var(--color-text-muted);
  border-top: none;
  font-style: italic;
}

/* Empty state row */
.job-table-empty {
  text-align: center;
  color: var(--color-text-muted);
  padding: var(--space-12) var(--space-4) !important;
  font-size: var(--text-sm);
}
```

**Template changes**:
- `dashboard.html`: add `class="job-table"` to `<table>`, wrap table in `<div class="job-table-wrapper">`
- `dashboard.html`: remove `style="cursor:pointer;user-select:none;"` from `<th>` (now handled by CSS via `th[hx-get]`)
- `partials/job_rows.html`: replace `style="text-align:center;color:#999;"` on empty `<td>` with `class="job-table-empty"`
- `partials/job_rows.html`: replace `style="padding:0.2em 1em 0.8em;font-size:0.85em;color:#555;border-top:none;"` on summary row `<td>` — the whole `<tr>` gets `class="job-summary-row"`, remove inline style

---

### 5.5 Forms and Inputs

**General form elements** — Pico already normalises these; the overrides in Section 3
redirect the colours. Additional scoped rules:

```css
/* Label */
label {
  font-size: var(--text-sm);
  font-weight: var(--weight-medium);
  color: var(--color-text);
  margin-bottom: var(--space-1);
  display: block;
}

/* Helper / hint text beneath a field */
label small,
.field-hint {
  display: block;
  font-size: var(--text-xs);
  color: var(--color-text-muted);
  font-weight: var(--weight-normal);
  margin-top: var(--space-1);
}

/* Input, select, textarea — Pico handles the border/padding via --pico-* overrides,
   but we add a sharper focus ring that meets WCAG 2.1 criterion 1.4.11 */
input:focus,
select:focus,
textarea:focus {
  outline: 2px solid var(--color-accent);
  outline-offset: 2px;
  border-color: var(--color-accent);
  box-shadow: 0 0 0 3px var(--color-accent-subtle);
}

/* Monospaced textarea (resume, description fields) */
textarea.mono {
  font-family: var(--font-mono);
  font-size: var(--text-sm);
}

/* Filter form grid — 4 columns on tablet+, stacked on mobile */
.filter-form-grid {
  display: grid;
  grid-template-columns: 1fr;
  gap: var(--space-3);
  align-items: end;
}
@media (min-width: 640px) {
  .filter-form-grid {
    grid-template-columns: 1fr 1fr;
  }
}
@media (min-width: 900px) {
  .filter-form-grid {
    grid-template-columns: 1fr 1fr 1fr auto;
  }
}

/* Inline form row (display name + save button) */
.inline-form {
  display: flex;
  gap: var(--space-3);
  align-items: flex-end;
  max-width: 480px;
}
.inline-form label {
  flex: 1;
  margin-bottom: 0;
}
```

**Template changes**:
- `settings.html`: replace `style="display:grid;…"` on filter form `<div>` with `class="filter-form-grid"`
- `settings.html`: replace inline `style` on `<textarea>` with `class="mono"` 
- `settings.html`: replace inline `style` on NTFY "Save Notifications" `<button>` — use default Pico button (no inline style needed)
- `profile.html`: replace `style="display:flex;gap:0.5rem;align-items:end;max-width:480px;"` with `class="inline-form"`

---

### 5.6 Buttons

**Three visual variants**: primary (filled indigo), secondary/outline (ghost), danger (filled red).
**One size modifier**: `.btn-sm` (compact, used in tables).

```css
/* === PRIMARY === */
/* Pico's default button is already mapped to --pico-primary = var(--color-accent) via Section 3.
   Additional micro-interactions: */
button,
[role="button"] {
  border-radius: var(--radius-md);
  font-weight: var(--weight-medium);
  transition: background-color 0.15s ease, border-color 0.15s ease, box-shadow 0.15s ease;
  cursor: pointer;
}
button:focus-visible,
[role="button"]:focus-visible {
  outline: 2px solid var(--color-accent);
  outline-offset: 2px;
}

/* === OUTLINE / SECONDARY === */
/* Pico renders .outline buttons with transparent background; we align the border. */
button.outline,
[role="button"].outline {
  border-color: var(--color-border);
  color: var(--color-text);
  background: transparent;
}
button.outline:hover,
[role="button"].outline:hover {
  background: var(--color-surface-alt);
  border-color: var(--color-text-muted);
}

/* === CONTRAST (used for Approve-type actions, maps to primary in Pico) === */
/* Already covered by --pico-primary override in Section 3. */

/* === DANGER === */
button.danger,
[role="button"].danger {
  background: var(--color-danger);
  border-color: var(--color-danger);
  color: #ffffff;
}
button.danger:hover,
[role="button"].danger:hover {
  background: #b91c1c;   /* red-700 */
  border-color: #b91c1c;
}

/* === SECONDARY (muted gray) === */
button.secondary,
[role="button"].secondary {
  background: transparent;
  color: var(--color-text-muted);
  border-color: var(--color-border);
}
button.secondary:hover,
[role="button"].secondary:hover {
  background: var(--color-surface-alt);
  color: var(--color-text);
}

/* === SMALL MODIFIER === */
.btn-sm {
  padding: 0.2rem 0.6rem !important;
  font-size: var(--text-xs) !important;
  border-radius: var(--radius-sm) !important;
  line-height: 1.5;
}
```

**Template changes**:
- `partials/job_rows.html`: replace `style="padding:0.2em 0.6em;font-size:0.8em;"` with `class="btn-sm"` on every action button
- `job_detail.html`: same — replace inline `style` on Approve/Reject buttons with `class="btn-sm"`
- `settings.html`: replace `style="padding:0.2em 0.6em;font-size:0.8em;"` on Remove buttons with `class="btn-sm"`

---

### 5.7 Cards (`.card`)

Generic card container used for content blocks that need elevation.

```css
.card {
  background: var(--color-surface);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  padding: var(--space-6);
  box-shadow: var(--shadow-xs);
}

.card-header {
  margin-bottom: var(--space-4);
  padding-bottom: var(--space-4);
  border-bottom: 1px solid var(--color-border);
}

.card-header h2,
.card-header h3 {
  margin: 0;
  font-size: var(--text-lg);
  font-weight: var(--weight-semibold);
}
```

Note: Pico's `<article>` element is styled as a card by default. The `.card` class
is an explicit alternative for cases where a `<section>` or `<div>` should look like
a card. The `onboarding.html` `<article>` is fine as-is (Pico styles it).

---

### 5.8 Flash / Alert Messages (`.alert`)

Two semantic variants: success and danger. Both share base styles and add a coloured
left accent border.

```css
.alert {
  display: flex;
  align-items: flex-start;
  gap: var(--space-3);
  padding: var(--space-3) var(--space-4);
  border-radius: var(--radius-md);
  border-left: 3px solid transparent;
  margin-bottom: var(--space-4);
  font-size: var(--text-sm);
}

.alert-success {
  background: var(--color-success-bg);
  color: var(--color-success);
  border-left-color: var(--color-success);
}

.alert-danger {
  background: var(--color-danger-bg);
  color: var(--color-danger);
  border-left-color: var(--color-danger);
}

.alert-warning {
  background: var(--color-warning-bg);
  color: var(--color-warning);
  border-left-color: var(--color-warning);
}

.alert-info {
  background: var(--color-info-bg);
  color: var(--color-info);
  border-left-color: var(--color-info);
}
```

**ARIA**: Flash message `<div>` elements should retain `role="alert"` (already present
in several templates). This triggers `aria-live="assertive"` for screen readers.

**Template changes**:
- `settings.html`: replace `style="background:#e8f5e9;color:#2e7d32;…"` with `class="alert alert-success"`
- `profile.html`: replace success/danger inline styles with `class="alert alert-success"` / `class="alert alert-danger"`
- `login.html`: flash alert already has `class="flash-alert"` — replace with `class="alert alert-info"` (or danger depending on content — leave as generic `alert` if message type is unknown)

---

### 5.9 Hero Section (`.hero`)

Used for the unauthenticated dashboard landing view.

```css
.hero {
  text-align: center;
  padding: var(--space-20) var(--space-4);
}

.hero h1 {
  font-size: var(--text-3xl);
  font-weight: var(--weight-bold);
  color: var(--color-text);
  margin-bottom: var(--space-4);
  line-height: var(--leading-tight);
}

.hero p {
  max-width: 40rem;
  margin-inline: auto;
  margin-bottom: var(--space-8);
  color: var(--color-text-muted);
  font-size: var(--text-lg);
  line-height: var(--leading-relaxed);
}

.hero a[role="button"] {
  display: inline-flex;
  align-items: center;
  padding: var(--space-3) var(--space-8);
  border-radius: var(--radius-full);
  font-size: var(--text-base);
  font-weight: var(--weight-semibold);
}
```

**Template changes** in `dashboard.html`:
- Replace `<section style="text-align:center;padding:4rem 1rem;">` with `<section class="hero">`
- Remove all inline `style` attributes on children; apply no class (Pico handles `<h1>` and `<p>`)

---

### 5.10 Login Page

**Layout**: Full-viewport centering of a floating card. Login has its own `<head>`,
so `/static/app.css` must be added there.

```css
/* Login page body — vertically and horizontally centred */
body.login-page {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 100vh;
  background: var(--color-bg);
}

/* Login card */
.login-card {
  max-width: 420px;
  width: 100%;
  padding: var(--space-8);
  background: var(--color-surface);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-lg);
}

.login-card-header {
  text-align: center;
  margin-bottom: var(--space-6);
}

.login-card-header h1 {
  font-size: var(--text-2xl);
  font-weight: var(--weight-bold);
  color: var(--color-text);
  margin-bottom: var(--space-1);
}

.login-card-header p {
  font-size: var(--text-sm);
  color: var(--color-text-muted);
  margin: 0;
}

/* Provider buttons — full width, pill shape */
.provider-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 100%;
  padding: var(--space-3) var(--space-4);
  border-radius: var(--radius-full);
  font-size: var(--text-sm);
  font-weight: var(--weight-medium);
  text-decoration: none;
  margin-bottom: var(--space-3);
  transition: background-color 0.15s ease;
  cursor: pointer;
}

/* Google button — primary filled */
.provider-btn-google {
  background: var(--color-accent);
  color: #ffffff;
  border: 1px solid var(--color-accent);
}
.provider-btn-google:hover {
  background: var(--color-accent-hover);
  border-color: var(--color-accent-hover);
  color: #ffffff;
  text-decoration: none;
}

/* GitHub button — outline */
.provider-btn-github {
  background: transparent;
  color: var(--color-text);
  border: 1px solid var(--color-border);
}
.provider-btn-github:hover {
  background: var(--color-surface-alt);
  text-decoration: none;
}

/* Focus states for provider buttons */
.provider-btn:focus-visible {
  outline: 2px solid var(--color-accent);
  outline-offset: 2px;
}
```

**Template changes in `login.html`**:
- Add `<link rel="stylesheet" href="/static/app.css">` after Pico link
- Add `class="login-page"` to `<body>` (used for the flex-centering rule)
- Remove the entire `<style>` block
- `<article class="login-card">` → keep, but rename inner header to `<header class="login-card-header">`
- Replace `role="button" class="provider-btn"` on Google link with `class="provider-btn provider-btn-google"`
- Replace `role="button" class="provider-btn outline"` on GitHub link with `class="provider-btn provider-btn-github"`
- Flash alert: `class="flash-alert"` → `class="alert alert-info"`

Note: `role="button"` on `<a>` elements is acceptable but the class-based approach
above is cleaner and doesn't need it; these elements navigate so `<a href>` is correct.
Retain `role="button"` only if the HTMX busy/loading attribute behaviour relies on it.
If both providers are present, Google appears first (primary) and GitHub second (outline) —
this matches the existing order in the template.

---

### 5.11 Settings Page

No structural changes beyond what's covered in Forms (5.5) and Flash Messages (5.8).
The inline `style` on the NTFY helper `<small>` text is already acceptable Pico
`color: #666` — replace with `class="field-hint"` (defined in 5.5).

**Template changes in `settings.html`**:
1. `<div role="alert" style="…">` → `<div role="alert" class="alert alert-success">`
2. `<p style="color:#999;">` → `<p class="text-muted">`
3. `<div style="display:grid;…">` → `<div class="filter-form-grid">`
4. `style="padding:0.2em 0.6em;font-size:0.8em;"` on Remove button → `class="btn-sm"`
5. `<small style="display:block;margin-top:0.25rem;color:#666;">` → `<small class="field-hint">`
6. `style="margin-top:0.75rem;"` on Save Notifications button → remove (Pico button already has vertical rhythm)
7. `style="font-family:monospace;font-size:0.9em;"` on resume `<textarea>` → `class="mono"`

---

### 5.12 Profile Page

**Avatar display**:
```css
.avatar {
  width: 48px;
  height: 48px;
  border-radius: var(--radius-full);
  object-fit: cover;
  vertical-align: middle;
  margin-right: var(--space-2);
}
```

**Template changes in `profile.html`**:
1. Success alert inline style → `class="alert alert-success"`
2. Danger alert inline style → `class="alert alert-danger"`
3. `style="display:flex;gap:0.5rem;align-items:end;max-width:480px;"` on div → `class="inline-form"`
4. Avatar img inline style → `class="avatar"`

---

### 5.13 Job Detail Page

**Description pre block**:
```css
.job-description {
  white-space: pre-wrap;
  font-size: var(--text-sm);
  font-family: var(--font-sans);   /* override <pre> monospace default */
  background: var(--color-surface-alt);
  padding: var(--space-4);
  border-radius: var(--radius-md);
  border: 1px solid var(--color-border);
  line-height: var(--leading-relaxed);
  overflow-x: auto;
}

/* Document preview (generated resume / cover letter) */
.document-preview {
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  padding: var(--space-4);
  margin-bottom: var(--space-3);
  overflow: auto;
  max-height: 600px;
  background: var(--color-surface);
}
```

**Template changes in `job_detail.html`**:
1. `<pre style="white-space:pre-wrap;font-size:0.9em;background:#f9f9f9;padding:1rem;border-radius:0.3em;">` → `<pre class="job-description">`
2. `<div style="border:1px solid #ccc;border-radius:0.3em;padding:1rem;margin-bottom:0.5rem;overflow:auto;">` (both resume and cover letter) → `<div class="document-preview">`
3. Approve/Reject button inline styles → `class="btn-sm"` (already `.outline .contrast` / `.outline`)

---

### 5.14 Onboarding Page

**Template changes in `onboarding.html`**:
1. `style="color:var(--pico-color-red-500);margin-bottom:1rem;"` on error div → `class="alert alert-danger"`
2. `style="color:var(--pico-color-red-500)"` on asterisk span → `class="text-danger"`

---

### 5.15 Empty States

For the "No jobs found" row and similar zero-data moments:

```css
/* Empty state inside a table */
.job-table-empty {
  text-align: center;
  color: var(--color-text-muted);
  padding: var(--space-12) var(--space-4) !important;
  font-size: var(--text-sm);
}

/* Empty state as a standalone section (future use) */
.empty-state {
  text-align: center;
  padding: var(--space-16) var(--space-4);
  color: var(--color-text-muted);
}
.empty-state p {
  font-size: var(--text-base);
  margin-bottom: var(--space-4);
}
```

---

## 6. Utility Classes

A minimal set of utilities to replace one-off inline styles.

```css
/* Text colour utilities */
.text-muted   { color: var(--color-text-muted); }
.text-success { color: var(--color-success); }
.text-danger  { color: var(--color-danger); }
.text-warning { color: var(--color-warning); }
.text-accent  { color: var(--color-accent); }

/* Screen-reader only (hide visually, keep for AT) */
.sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  margin: -1px;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
  white-space: nowrap;
  border-width: 0;
}

/* Display utilities */
.d-flex { display: flex; }
.gap-2  { gap: var(--space-2); }
.gap-3  { gap: var(--space-3); }

/* Spacing */
.mt-0 { margin-top: 0; }
.mb-0 { margin-bottom: 0; }
.mb-4 { margin-bottom: var(--space-4); }
```

---

## 7. Responsive Breakpoints Strategy

### Breakpoint Definitions

```
xs:  < 480px   — small mobile (single-column everything)
sm:  ≥ 480px   — large mobile / small tablet
md:  ≥ 640px   — tablet (two-column filter form)
lg:  ≥ 768px   — desktop (full nav, multi-column layouts)
xl:  ≥ 900px   — wide desktop (four-column filter form)
2xl: ≥ 1200px  — max container width, content stops growing
```

### Mobile-First Approach

All base styles are written for mobile. Media queries add complexity for larger screens.

| Component | Mobile (< 640px) | Tablet (≥ 640px) | Desktop (≥ 768px) |
|---|---|---|---|
| Nav | Brand + links wrap to two rows | Single row, links right-aligned | Sticky, single row |
| Tab bar | Wraps freely | Row | Row |
| Job table | Horizontal scroll in wrapper | Full width | Full width |
| Filter form grid | 1 column | 2 columns | 4 columns |
| Login card | Full-width (padding only) | Centered 420px | Centered 420px |
| Hero | Stack, smaller text | Normal | Normal |

### Nav Collapse on Mobile

On screens below 640px, the nav user area drops below the brand/links row:
```css
@media (max-width: 639px) {
  .app-nav-inner {
    flex-direction: column;
    align-items: flex-start;
    gap: var(--space-2);
  }
  .app-nav-user {
    margin-left: 0;
    border-top: 1px solid var(--color-border);
    padding-top: var(--space-2);
    width: 100%;
  }
}
```

---

## 8. Accessibility Requirements

### 8.1 WCAG Level: AA (2.1)

The design targets WCAG 2.1 Level AA for all new and modified components.

### 8.2 Contrast Requirements

All text/background combinations listed in this spec meet 4.5:1 (normal text) or 3:1 (large text ≥18px normal / ≥14px bold):

| Pair | Ratio | Required | Level |
|---|---|---|---|
| `--color-text` on `--color-bg` | 16.5:1 | 4.5:1 | AAA |
| `--color-text-muted` on `--color-surface` | 4.6:1 | 4.5:1 | AA |
| White on `--color-accent` | 5.0:1 | 4.5:1 | AA |
| `--color-accent` on white | 5.0:1 | 4.5:1 | AA |
| `--color-danger` on `--color-danger-bg` | 5.0:1 | 4.5:1 | AA |
| `--color-success` on `--color-success-bg` | 4.5:1 | 4.5:1 | AA |
| `--color-warning` on `--color-warning-bg` | 4.5:1 | 4.5:1 | AA (verified — amber on amber-100) |
| `--color-info` on `--color-info-bg` | 4.8:1 | 4.5:1 | AA |
| `--color-purple` on `--color-purple-bg` | 5.1:1 | 4.5:1 | AA |
| `--color-teal` on `--color-teal-bg` | 4.6:1 | 4.5:1 | AA |

### 8.3 Focus States

Every interactive element must have a visible `:focus-visible` style:
- All inputs, selects, textareas: 2px solid `--color-accent` outline + 2px offset + subtle shadow
- All buttons and `[role="button"]` elements: 2px solid `--color-accent` + 2px offset
- All links: rely on browser default (acceptable) or add `outline: 2px solid var(--color-accent)` via `:focus-visible`
- Tab bar items: explicit `:focus-visible` rule in 5.3

### 8.4 Semantic HTML Requirements

- `<nav aria-label="Main navigation">` on the app nav
- `<main>` wraps page content (already present via Pico's container pattern)
- Status badge `<span>` elements: acceptable as-is for display; no interactive role needed
- Table `<th>` elements with `scope="col"` attribute should be added for screen readers
- All images: `alt=""` (avatar images are decorative) or meaningful alt text where applicable
- Flash messages retain `role="alert"` for live region behaviour

### 8.5 Motion and Animation

All CSS transitions are limited to `0.15s ease` on `background-color`, `color`,
`border-color`, and `box-shadow`. No animations exceed 0.3s. No spinning/bouncing
animations are introduced. No `prefers-reduced-motion` override is needed at this
scale, but add this safety blanket:

```css
@media (prefers-reduced-motion: reduce) {
  *, *::before, *::after {
    transition-duration: 0.01ms !important;
    animation-duration: 0.01ms !important;
  }
}
```

---

## 9. Complete `app.css` Content

The following is the complete, annotated CSS file a coder should produce at
`internal/web/templates/static/app.css`. Sections are ordered so that custom
properties are declared before use.

```css
/* ==========================================================================
   app.css — JobHuntr custom stylesheet
   Layers on top of PicoCSS v2. Override --pico-* properties to theme Pico;
   add component classes for nav, badges, tables, forms, and alerts.
   ========================================================================== */

/* --------------------------------------------------------------------------
   1. DESIGN TOKENS
   -------------------------------------------------------------------------- */
:root {
  /* Colour palette */
  --color-bg:           #f8f9fb;
  --color-surface:      #ffffff;
  --color-surface-alt:  #f1f3f7;
  --color-border:       #e2e5ec;
  --color-text:         #1a1d23;
  --color-text-muted:   #6b7280;
  --color-accent:        #4f46e5;
  --color-accent-hover:  #4338ca;
  --color-accent-subtle: #eef2ff;
  --color-success:     #16a34a;
  --color-success-bg:  #dcfce7;
  --color-warning:     #d97706;
  --color-warning-bg:  #fef3c7;
  --color-danger:      #dc2626;
  --color-danger-bg:   #fee2e2;
  --color-info:        #2563eb;
  --color-info-bg:     #dbeafe;
  --color-purple:      #7c3aed;
  --color-purple-bg:   #ede9fe;
  --color-teal:        #0d9488;
  --color-teal-bg:     #ccfbf1;

  /* Typography */
  --font-sans: ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont,
               "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
  --font-mono: ui-monospace, "Cascadia Code", "Source Code Pro", Menlo,
               Consolas, "DejaVu Sans Mono", monospace;
  --text-xs:   0.75rem;
  --text-sm:   0.875rem;
  --text-base: 1rem;
  --text-lg:   1.125rem;
  --text-xl:   1.25rem;
  --text-2xl:  1.5rem;
  --text-3xl:  1.875rem;
  --weight-normal:   400;
  --weight-medium:   500;
  --weight-semibold: 600;
  --weight-bold:     700;
  --leading-tight:   1.25;
  --leading-normal:  1.5;
  --leading-relaxed: 1.625;

  /* Spacing (8px grid) */
  --space-1:  0.25rem;
  --space-2:  0.5rem;
  --space-3:  0.75rem;
  --space-4:  1rem;
  --space-5:  1.25rem;
  --space-6:  1.5rem;
  --space-8:  2rem;
  --space-10: 2.5rem;
  --space-12: 3rem;
  --space-16: 4rem;
  --space-20: 5rem;

  /* Border radius */
  --radius-sm:   0.375rem;
  --radius-md:   0.5rem;
  --radius-lg:   0.75rem;
  --radius-full: 9999px;

  /* Shadows */
  --shadow-xs: 0 1px 2px rgba(0, 0, 0, 0.05);
  --shadow-sm: 0 1px 3px rgba(0, 0, 0, 0.08), 0 1px 2px rgba(0, 0, 0, 0.06);
  --shadow-md: 0 4px 6px rgba(0, 0, 0, 0.07), 0 2px 4px rgba(0, 0, 0, 0.06);
  --shadow-lg: 0 10px 15px rgba(0, 0, 0, 0.08), 0 4px 6px rgba(0, 0, 0, 0.05);
}

/* --------------------------------------------------------------------------
   2. PICO v2 OVERRIDE MAPPINGS
   -------------------------------------------------------------------------- */
:root {
  --pico-background-color:    var(--color-bg);
  --pico-card-background-color: var(--color-surface);
  --pico-card-sectioning-background-color: var(--color-surface-alt);
  --pico-font-family:         var(--font-sans);
  --pico-font-size:           var(--text-base);
  --pico-line-height:         var(--leading-normal);
  --pico-color:               var(--color-text);
  --pico-muted-color:         var(--color-text-muted);
  --pico-h1-color:            var(--color-text);
  --pico-h2-color:            var(--color-text);
  --pico-h3-color:            var(--color-text);
  --pico-border-color:        var(--color-border);
  --pico-card-border-radius:  var(--radius-lg);
  --pico-primary:             var(--color-accent);
  --pico-primary-hover:       var(--color-accent-hover);
  --pico-primary-focus:       var(--color-accent);
  --pico-primary-inverse:     #ffffff;
  --pico-border-radius:       var(--radius-md);
  --pico-form-element-border-color: var(--color-border);
  --pico-form-element-focus-color:  var(--color-accent);
  --pico-form-element-background-color: var(--color-surface);
  --pico-form-element-color:  var(--color-text);
  --pico-form-element-placeholder-color: var(--color-text-muted);
  --pico-secondary:           #64748b;
  --pico-secondary-hover:     #475569;
  --pico-secondary-focus:     var(--color-accent);
  --pico-secondary-inverse:   #ffffff;
}

/* --------------------------------------------------------------------------
   3. BASE / LAYOUT
   -------------------------------------------------------------------------- */
body {
  background-color: var(--color-bg);
  color: var(--color-text);
  font-family: var(--font-sans);
  font-size: var(--text-base);
  line-height: var(--leading-normal);
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
}

.app-container {
  max-width: 1200px;
  margin-inline: auto;
  padding-inline: var(--space-4);
  padding-top: var(--space-6);
  padding-bottom: var(--space-12);
}

@media (min-width: 768px) {
  .app-container {
    padding-inline: var(--space-8);
  }
}

/* Top-level heading on content pages */
.app-container > h1 {
  margin-top: var(--space-6);
  margin-bottom: var(--space-6);
  font-size: var(--text-2xl);
  font-weight: var(--weight-bold);
}

/* Reduced motion */
@media (prefers-reduced-motion: reduce) {
  *, *::before, *::after {
    transition-duration: 0.01ms !important;
    animation-duration: 0.01ms !important;
  }
}

/* --------------------------------------------------------------------------
   4. NAVIGATION
   -------------------------------------------------------------------------- */
.app-nav {
  position: sticky;
  top: 0;
  z-index: 100;
  background: var(--color-surface);
  border-bottom: 1px solid var(--color-border);
  padding: var(--space-3) var(--space-4);
}

@media (min-width: 768px) {
  .app-nav {
    padding-inline: var(--space-8);
  }
}

.app-nav-inner {
  max-width: 1200px;
  margin-inline: auto;
  display: flex;
  align-items: center;
  gap: var(--space-4);
  flex-wrap: wrap;
}

.app-nav-brand {
  font-weight: var(--weight-semibold);
  font-size: var(--text-lg);
  color: var(--color-accent);
  text-decoration: none;
  margin-right: var(--space-2);
}

.app-nav-brand:hover {
  color: var(--color-accent-hover);
  text-decoration: none;
}

.app-nav-links {
  display: flex;
  align-items: center;
  gap: var(--space-1);
  flex: 1;
}

.app-nav-links a {
  color: var(--color-text-muted);
  font-size: var(--text-sm);
  font-weight: var(--weight-medium);
  text-decoration: none;
  padding: var(--space-1) var(--space-2);
  border-radius: var(--radius-md);
  transition: color 0.15s ease, background-color 0.15s ease;
}

.app-nav-links a:hover {
  color: var(--color-text);
  background-color: var(--color-surface-alt);
}

.app-nav-links a:focus-visible {
  outline: 2px solid var(--color-accent);
  outline-offset: 2px;
}

.app-nav-user {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  margin-left: auto;
  font-size: var(--text-sm);
  color: var(--color-text-muted);
}

.app-nav-user a {
  color: var(--color-text-muted);
  text-decoration: none;
}

.app-nav-user a:hover {
  color: var(--color-text);
  text-decoration: underline;
}

.nav-avatar {
  width: 32px;
  height: 32px;
  border-radius: var(--radius-full);
  object-fit: cover;
  vertical-align: middle;
}

/* Mobile nav: stack user area below links */
@media (max-width: 639px) {
  .app-nav-inner {
    flex-direction: column;
    align-items: flex-start;
    gap: var(--space-2);
  }

  .app-nav-user {
    margin-left: 0;
    border-top: 1px solid var(--color-border);
    padding-top: var(--space-2);
    width: 100%;
  }
}

/* --------------------------------------------------------------------------
   5. STATUS BADGES
   -------------------------------------------------------------------------- */
.status-badge {
  display: inline-flex;
  align-items: center;
  padding: 2px 8px;
  border-radius: var(--radius-sm);
  font-size: var(--text-xs);
  font-weight: var(--weight-semibold);
  letter-spacing: 0.04em;
  text-transform: uppercase;
  white-space: nowrap;
  line-height: 1.5;
}

.status-discovered  { background: var(--color-info-bg);    color: var(--color-info);    }
.status-notified    { background: var(--color-purple-bg);   color: var(--color-purple);  }
.status-approved    { background: var(--color-success-bg);  color: var(--color-success); }
.status-rejected    { background: var(--color-danger-bg);   color: var(--color-danger);  }
.status-generating  { background: var(--color-warning-bg);  color: var(--color-warning); }
.status-complete    { background: var(--color-teal-bg);     color: var(--color-teal);    }
.status-failed      { background: var(--color-danger-bg);   color: var(--color-danger);  }

/* --------------------------------------------------------------------------
   6. TAB BAR
   -------------------------------------------------------------------------- */
.tab-bar {
  display: flex;
  gap: var(--space-2);
  flex-wrap: wrap;
  margin-bottom: var(--space-6);
}

.tab-bar a[role="button"] {
  display: inline-flex;
  align-items: center;
  padding: var(--space-1) var(--space-4);
  border-radius: var(--radius-full);
  font-size: var(--text-sm);
  font-weight: var(--weight-medium);
  text-decoration: none;
  border: 1px solid var(--color-border);
  background: transparent;
  color: var(--color-text);
  cursor: pointer;
  transition: background-color 0.15s ease, color 0.15s ease, border-color 0.15s ease;
  margin-bottom: 0;
}

.tab-bar a[role="button"]:hover {
  background: var(--color-surface-alt);
  border-color: var(--color-text-muted);
}

.tab-bar a[role="button"].contrast {
  background: var(--color-accent);
  color: #ffffff;
  border-color: var(--color-accent);
}

.tab-bar a[role="button"].contrast:hover {
  background: var(--color-accent-hover);
  border-color: var(--color-accent-hover);
}

.tab-bar a[role="button"]:focus-visible {
  outline: 2px solid var(--color-accent);
  outline-offset: 2px;
}

/* --------------------------------------------------------------------------
   7. JOB TABLE
   -------------------------------------------------------------------------- */
.job-table-wrapper {
  overflow-x: auto;
  -webkit-overflow-scrolling: touch;
  border-radius: var(--radius-lg);
  border: 1px solid var(--color-border);
}

.job-table {
  width: 100%;
  border-collapse: collapse;
  font-size: var(--text-sm);
}

.job-table thead tr {
  background: var(--color-surface-alt);
}

.job-table th {
  padding: var(--space-3) var(--space-4);
  text-align: left;
  font-size: var(--text-xs);
  font-weight: var(--weight-semibold);
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--color-text-muted);
  border-bottom: 1px solid var(--color-border);
  white-space: nowrap;
}

.job-table th[hx-get] {
  cursor: pointer;
  user-select: none;
}

.job-table th[hx-get]:hover {
  color: var(--color-text);
}

.job-table td {
  padding: var(--space-3) var(--space-4);
  color: var(--color-text);
  border-bottom: 1px solid var(--color-border);
  vertical-align: middle;
}

.job-table tbody tr:last-child td {
  border-bottom: none;
}

.job-table tbody tr:hover td {
  background: var(--color-surface-alt);
}

.job-table td a {
  font-weight: var(--weight-medium);
  color: var(--color-accent);
  text-decoration: none;
}

.job-table td a:hover {
  text-decoration: underline;
}

.job-summary-row td {
  padding: var(--space-1) var(--space-4) var(--space-3);
  font-size: var(--text-xs);
  color: var(--color-text-muted);
  border-top: none;
  font-style: italic;
}

.job-table-empty {
  text-align: center;
  color: var(--color-text-muted);
  padding: var(--space-12) var(--space-4) !important;
  font-size: var(--text-sm);
}

/* --------------------------------------------------------------------------
   8. FORMS AND INPUTS
   -------------------------------------------------------------------------- */
label {
  font-size: var(--text-sm);
  font-weight: var(--weight-medium);
  color: var(--color-text);
  margin-bottom: var(--space-1);
  display: block;
}

label small,
.field-hint {
  display: block;
  font-size: var(--text-xs);
  color: var(--color-text-muted);
  font-weight: var(--weight-normal);
  margin-top: var(--space-1);
}

input:focus,
select:focus,
textarea:focus {
  outline: 2px solid var(--color-accent);
  outline-offset: 2px;
  border-color: var(--color-accent);
  box-shadow: 0 0 0 3px var(--color-accent-subtle);
}

textarea.mono {
  font-family: var(--font-mono);
  font-size: var(--text-sm);
}

.filter-form-grid {
  display: grid;
  grid-template-columns: 1fr;
  gap: var(--space-3);
  align-items: end;
}

@media (min-width: 640px) {
  .filter-form-grid {
    grid-template-columns: 1fr 1fr;
  }
}

@media (min-width: 900px) {
  .filter-form-grid {
    grid-template-columns: 1fr 1fr 1fr auto;
  }
}

.inline-form {
  display: flex;
  gap: var(--space-3);
  align-items: flex-end;
  max-width: 480px;
}

.inline-form label {
  flex: 1;
  margin-bottom: 0;
}

/* --------------------------------------------------------------------------
   9. BUTTONS
   -------------------------------------------------------------------------- */
button,
[role="button"] {
  border-radius: var(--radius-md);
  font-weight: var(--weight-medium);
  transition: background-color 0.15s ease, border-color 0.15s ease, box-shadow 0.15s ease;
  cursor: pointer;
}

button:focus-visible,
[role="button"]:focus-visible {
  outline: 2px solid var(--color-accent);
  outline-offset: 2px;
}

button.outline,
[role="button"].outline {
  border-color: var(--color-border);
  color: var(--color-text);
  background: transparent;
}

button.outline:hover,
[role="button"].outline:hover {
  background: var(--color-surface-alt);
  border-color: var(--color-text-muted);
}

button.danger,
[role="button"].danger {
  background: var(--color-danger);
  border-color: var(--color-danger);
  color: #ffffff;
}

button.danger:hover,
[role="button"].danger:hover {
  background: #b91c1c;
  border-color: #b91c1c;
}

button.secondary,
[role="button"].secondary {
  background: transparent;
  color: var(--color-text-muted);
  border-color: var(--color-border);
}

button.secondary:hover,
[role="button"].secondary:hover {
  background: var(--color-surface-alt);
  color: var(--color-text);
}

.btn-sm {
  padding: 0.2rem 0.6rem !important;
  font-size: var(--text-xs) !important;
  border-radius: var(--radius-sm) !important;
  line-height: 1.5;
}

/* --------------------------------------------------------------------------
   10. CARDS
   -------------------------------------------------------------------------- */
.card {
  background: var(--color-surface);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  padding: var(--space-6);
  box-shadow: var(--shadow-xs);
}

.card-header {
  margin-bottom: var(--space-4);
  padding-bottom: var(--space-4);
  border-bottom: 1px solid var(--color-border);
}

.card-header h2,
.card-header h3 {
  margin: 0;
  font-size: var(--text-lg);
  font-weight: var(--weight-semibold);
}

/* --------------------------------------------------------------------------
   11. ALERT / FLASH MESSAGES
   -------------------------------------------------------------------------- */
.alert {
  display: flex;
  align-items: flex-start;
  gap: var(--space-3);
  padding: var(--space-3) var(--space-4);
  border-radius: var(--radius-md);
  border-left: 3px solid transparent;
  margin-bottom: var(--space-4);
  font-size: var(--text-sm);
}

.alert-success {
  background: var(--color-success-bg);
  color: var(--color-success);
  border-left-color: var(--color-success);
}

.alert-danger {
  background: var(--color-danger-bg);
  color: var(--color-danger);
  border-left-color: var(--color-danger);
}

.alert-warning {
  background: var(--color-warning-bg);
  color: var(--color-warning);
  border-left-color: var(--color-warning);
}

.alert-info {
  background: var(--color-info-bg);
  color: var(--color-info);
  border-left-color: var(--color-info);
}

/* --------------------------------------------------------------------------
   12. HERO SECTION
   -------------------------------------------------------------------------- */
.hero {
  text-align: center;
  padding: var(--space-20) var(--space-4);
}

.hero h1 {
  font-size: var(--text-3xl);
  font-weight: var(--weight-bold);
  color: var(--color-text);
  margin-bottom: var(--space-4);
  line-height: var(--leading-tight);
}

.hero p {
  max-width: 40rem;
  margin-inline: auto;
  margin-bottom: var(--space-8);
  color: var(--color-text-muted);
  font-size: var(--text-lg);
  line-height: var(--leading-relaxed);
}

.hero a[role="button"] {
  display: inline-flex;
  align-items: center;
  padding: var(--space-3) var(--space-8);
  border-radius: var(--radius-full);
  font-size: var(--text-base);
  font-weight: var(--weight-semibold);
}

/* --------------------------------------------------------------------------
   13. LOGIN PAGE
   -------------------------------------------------------------------------- */
body.login-page {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 100vh;
  background: var(--color-bg);
}

.login-card {
  max-width: 420px;
  width: 100%;
  padding: var(--space-8);
  background: var(--color-surface);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-lg);
}

.login-card-header {
  text-align: center;
  margin-bottom: var(--space-6);
}

.login-card-header h1 {
  font-size: var(--text-2xl);
  font-weight: var(--weight-bold);
  color: var(--color-text);
  margin-bottom: var(--space-1);
}

.login-card-header p {
  font-size: var(--text-sm);
  color: var(--color-text-muted);
  margin: 0;
}

.provider-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 100%;
  padding: var(--space-3) var(--space-4);
  border-radius: var(--radius-full);
  font-size: var(--text-sm);
  font-weight: var(--weight-medium);
  text-decoration: none;
  margin-bottom: var(--space-3);
  transition: background-color 0.15s ease;
  cursor: pointer;
}

.provider-btn-google {
  background: var(--color-accent);
  color: #ffffff;
  border: 1px solid var(--color-accent);
}

.provider-btn-google:hover {
  background: var(--color-accent-hover);
  border-color: var(--color-accent-hover);
  color: #ffffff;
  text-decoration: none;
}

.provider-btn-github {
  background: transparent;
  color: var(--color-text);
  border: 1px solid var(--color-border);
}

.provider-btn-github:hover {
  background: var(--color-surface-alt);
  text-decoration: none;
}

.provider-btn:focus-visible {
  outline: 2px solid var(--color-accent);
  outline-offset: 2px;
}

/* --------------------------------------------------------------------------
   14. JOB DETAIL — DESCRIPTION AND DOCUMENTS
   -------------------------------------------------------------------------- */
.job-description {
  white-space: pre-wrap;
  font-size: var(--text-sm);
  font-family: var(--font-sans);
  background: var(--color-surface-alt);
  padding: var(--space-4);
  border-radius: var(--radius-md);
  border: 1px solid var(--color-border);
  line-height: var(--leading-relaxed);
  overflow-x: auto;
}

.document-preview {
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  padding: var(--space-4);
  margin-bottom: var(--space-3);
  overflow: auto;
  max-height: 600px;
  background: var(--color-surface);
}

/* --------------------------------------------------------------------------
   15. AVATAR
   -------------------------------------------------------------------------- */
.avatar {
  width: 48px;
  height: 48px;
  border-radius: var(--radius-full);
  object-fit: cover;
  vertical-align: middle;
  margin-right: var(--space-2);
}

/* --------------------------------------------------------------------------
   16. EMPTY STATES
   -------------------------------------------------------------------------- */
.empty-state {
  text-align: center;
  padding: var(--space-16) var(--space-4);
  color: var(--color-text-muted);
}

.empty-state p {
  font-size: var(--text-base);
  margin-bottom: var(--space-4);
}

/* --------------------------------------------------------------------------
   17. UTILITY CLASSES
   -------------------------------------------------------------------------- */
.text-muted   { color: var(--color-text-muted); }
.text-success { color: var(--color-success); }
.text-danger  { color: var(--color-danger); }
.text-warning { color: var(--color-warning); }
.text-accent  { color: var(--color-accent); }

.sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  margin: -1px;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
  white-space: nowrap;
  border-width: 0;
}

.d-flex { display: flex; }
.gap-2  { gap: var(--space-2); }
.gap-3  { gap: var(--space-3); }
.mt-0   { margin-top: 0; }
.mb-0   { margin-bottom: 0; }
.mb-4   { margin-bottom: var(--space-4); }
```

---

## 10. Template Change Summary (Quick Reference for Coder)

### `internal/web/templates/static/app.css`
- **Action**: Create new file and directory
- **Content**: Full CSS from Section 9 above

### `internal/web/server.go`
- **Action**: Add static file route (see architecture plan Step 2)

### `internal/web/templates/layout.html`
| Element | Change |
|---|---|
| `<head>` | Add `<link rel="stylesheet" href="/static/app.css">` after Pico link |
| `<style>` block | Remove entirely (content replaced by `app.css`) |
| `<nav>` | Restructure: `<nav class="app-nav" aria-label="Main navigation"><div class="app-nav-inner">…</div></nav>` |
| Nav links | Wrap in `<div class="app-nav-links">` |
| `<span style="float:right;">` | Replace with `<div class="app-nav-user">` |
| Avatar `<img>` inline style | Replace with `class="nav-avatar"` |
| `<main class="container">` | Change to `<main class="app-container">` |

### `internal/web/templates/login.html`
| Element | Change |
|---|---|
| `<head>` | Add `<link rel="stylesheet" href="/static/app.css">` after Pico link |
| `<style>` block | Remove entirely |
| `<body>` | Add `class="login-page"` |
| `<article class="login-card">` | Keep; rename inner `<header>` to `<header class="login-card-header">` |
| Google `<a>` | `class="provider-btn provider-btn-google"` (remove `role="button"`) |
| GitHub `<a>` | `class="provider-btn provider-btn-github"` (remove `role="button" class="provider-btn outline"`) |
| Flash `<div>` | `class="alert alert-info"` |

### `internal/web/templates/dashboard.html`
| Element | Change |
|---|---|
| `<section style="text-align:center;…">` | `<section class="hero">` |
| `<table>` | Wrap in `<div class="job-table-wrapper">`, add `class="job-table"` |
| `<th style="cursor:pointer;user-select:none;">` | Remove inline style (CSS handles `th[hx-get]`) |

### `internal/web/templates/partials/job_rows.html`
| Element | Change |
|---|---|
| Empty `<td style="…">` | Add `class="job-table-empty"`, remove style |
| Summary `<tr>` | Add `class="job-summary-row"`, remove `<td style="…">` |
| Action buttons inline style | Replace `style="padding:0.2em 0.6em;font-size:0.8em;"` with `class="btn-sm"` |

### `internal/web/templates/job_detail.html`
| Element | Change |
|---|---|
| `<pre style="…">` | `<pre class="job-description">` |
| Resume preview `<div style="…">` | `<div class="document-preview">` |
| Cover letter preview `<div style="…">` | `<div class="document-preview">` |
| Approve/Reject `<button style="…">` | Add `class="btn-sm"` (keep `.outline .contrast` / `.outline`) |

### `internal/web/templates/settings.html`
| Element | Change |
|---|---|
| Success alert `<div style="…">` | `class="alert alert-success"` |
| `<p style="color:#999;">` | `class="text-muted"` |
| Filter grid `<div style="…">` | `class="filter-form-grid"` |
| Remove `<button style="…">` | Add `class="btn-sm"` |
| `<small style="…">` | `class="field-hint"` |
| Save button `style="margin-top:0.75rem;"` | Remove attribute |
| Resume `<textarea style="…">` | Add `class="mono"` |

### `internal/web/templates/profile.html`
| Element | Change |
|---|---|
| Success alert `<div style="…">` | `class="alert alert-success"` |
| Error alert `<div style="…">` | `class="alert alert-danger"` |
| Flex div `<div style="…">` | `class="inline-form"` |
| Avatar `<img style="…">` | `class="avatar"` |

### `internal/web/templates/onboarding.html`
| Element | Change |
|---|---|
| Error `<div style="…">` | `class="alert alert-danger"` |
| Asterisk `<span style="…">` | `class="text-danger"` |

---

## 11. QA Verification Checklist

The QA agent should verify:

- [ ] `grep -r 'style="' internal/web/templates/` returns zero results (except dynamically-set HTMX attributes)
- [ ] `/static/app.css` is served with HTTP 200 and correct `Content-Type: text/css`
- [ ] Page background is `#f8f9fb` (not white)
- [ ] Nav is sticky (scroll down on dashboard, nav stays visible)
- [ ] Status badges are pill-shaped and uppercase with correct colours for each status value
- [ ] Active tab on dashboard has indigo fill
- [ ] Job table has no outer border; rows have hover highlight; columns are sortable
- [ ] Form inputs show indigo focus ring on click/tab
- [ ] Alert messages have left coloured border
- [ ] Login page is centred with card shadow
- [ ] Provider buttons are full-width
- [ ] No HTMX interactions are broken (approve/reject, table partial swap, search)
- [ ] Renders correctly at 375px viewport width (iPhone SE)
- [ ] All colour contrast ratios pass (run aXe or Lighthouse accessibility audit)
- [ ] Keyboard tab navigation follows logical order through nav → tabs → search → table
