# Task: modern-design-3-content-templates

- **Type**: coder
- **Status**: pending
- **Repo**: . (root — single repo at /workspace)
- **Parallel Group**: 2
- **Branch**: feature/modern-design-3
- **Source Item**: Modern Design Refresh — `.workflow/plans/modern-design.md`
- **Dependencies**: modern-design-1-css-static

## Description

Update the six content templates — `dashboard.html`, `job_detail.html`,
`partials/job_rows.html`, `settings.html`, `profile.html`, and `onboarding.html` —
to replace inline `style=""` attributes with CSS classes from `app.css`.

No new CSS is written here. All classes are already defined in `app.css` (task 1).
This task is pure template cleanup: add classes, remove inline styles.

This task runs in parallel with task modern-design-2-layout-login; both depend on
task modern-design-1-css-static being complete first.

## Acceptance Criteria

**dashboard.html**
- [ ] `<table>` has `class="job-table"` and is wrapped in `<div class="job-table-wrapper">`
- [ ] Unauthenticated hero section: `<section class="hero">` (no inline `style` on the section or its child `<h1>`, `<p>`)
- [ ] Tab bar container has `class="tab-bar"`
- [ ] Inline `style="cursor:pointer;user-select:none;"` removed from `<th>` elements (handled by `th[hx-get]` CSS)
- [ ] No `style=""` attributes remain

**job_detail.html**
- [ ] Job description `<pre>` uses `class="job-description"` (no inline `style`)
- [ ] Document preview `<div>`s (resume, cover letter sections) use `class="document-preview"` (no inline `style`)
- [ ] Approve/Reject/status-change buttons use `class="btn-sm"` (inline style removed)
- [ ] No `style=""` attributes remain

**partials/job_rows.html**
- [ ] Empty-state `<td>` uses `class="job-table-empty"` (replaces `style="text-align:center;color:#999;"`)
- [ ] Summary row `<tr>` has `class="job-summary-row"` and inline style on its `<td>` is removed
- [ ] Action buttons (Approve, Reject, etc.) have `class="btn-sm"` (inline style removed)
- [ ] No `style=""` attributes remain

**settings.html**
- [ ] Success flash/alert `<div>` uses `class="alert alert-success"` (inline style removed)
- [ ] Muted text `<p style="color:#999">` or equivalent uses `class="text-muted"` (inline style removed)
- [ ] Filter form `<div style="display:grid;…">` uses `class="filter-form-grid"` (inline style removed)
- [ ] Remove filter `<button>` with small-font inline style uses `class="btn-sm"` (inline style removed)
- [ ] NTFY `<small style="…">` hint uses `class="field-hint"` (inline style removed)
- [ ] Resume `<textarea style="font-family:monospace;…">` uses `class="mono"` (inline style removed)
- [ ] "Save Notifications" button: remove `style="margin-top:0.75rem;"` (Pico provides vertical rhythm)
- [ ] No `style=""` attributes remain

**profile.html**
- [ ] Success alert inline style replaced with `class="alert alert-success"`
- [ ] Danger alert inline style replaced with `class="alert alert-danger"`
- [ ] Display-name row `<div style="display:flex;…">` uses `class="inline-form"` (inline style removed)
- [ ] Avatar `<img style="…">` uses `class="avatar"` (inline style removed)
- [ ] No `style=""` attributes remain

**onboarding.html**
- [ ] Error/validation div `style="color:var(--pico-color-red-500);…"` uses `class="alert alert-danger"` (inline style removed)
- [ ] Required-field asterisk span `style="color:var(--pico-color-red-500)"` uses `class="text-danger"` (inline style removed)
- [ ] No `style=""` attributes remain

**Cross-cutting**
- [ ] All HTMX attributes (`hx-get`, `hx-post`, `hx-target`, `hx-swap`, `hx-trigger`, etc.) are preserved exactly as they were
- [ ] All Go template actions (`{{.}}`, `{{range}}`, `{{if}}`, etc.) are preserved exactly
- [ ] The app compiles and all six pages render without visual regression

## Interface Contracts

This task consumes CSS classes from `app.css` produced by task
**modern-design-1-css-static**. Do not add new CSS here — if a class is needed
that is not in `app.css`, surface it in the Notes section.

**Classes used by these templates**:

| Template | Classes consumed |
|---|---|
| dashboard.html | `.job-table`, `.job-table-wrapper`, `.hero`, `.tab-bar` |
| job_detail.html | `.job-description`, `.document-preview`, `.btn-sm` |
| partials/job_rows.html | `.job-table-empty`, `.job-summary-row`, `.btn-sm` |
| settings.html | `.alert`, `.alert-success`, `.text-muted`, `.filter-form-grid`, `.btn-sm`, `.field-hint`, `.mono` |
| profile.html | `.alert`, `.alert-success`, `.alert-danger`, `.inline-form`, `.avatar` |
| onboarding.html | `.alert`, `.alert-danger`, `.text-danger` |

## Context

### Architecture plan reference
`.workflow/plans/modern-design.md` — Steps 5 and 6 (content templates)

### Design spec reference
`.workflow/plans/modern-design-spec.md`:
- Section 5.3 — tab-bar (no template change needed, tab class structure already correct)
- Section 5.4 — job table template changes
- Section 5.5 — form template changes
- Section 5.6 — button template changes
- Section 5.8 — alert template changes
- Section 5.9 — hero template changes
- Section 5.11 — settings template changes
- Section 5.12 — profile template changes
- Section 5.13 — job_detail template changes
- Section 5.14 — onboarding template changes

### Per-file change details

#### dashboard.html (`internal/web/templates/dashboard.html`)

1. Find `<table` and add `class="job-table"`. Wrap the whole `<table>…</table>` in:
   ```html
   <div class="job-table-wrapper">…</div>
   ```

2. Find the tab bar container (the `<div>` or `<nav>` wrapping the filter tab buttons)
   and add `class="tab-bar"`. The existing `class="contrast"` / `class="outline"` on
   individual tab `<a>` elements should be kept — `.tab-bar a[role="button"].contrast`
   in `app.css` handles the active style.

3. Find the unauthenticated hero section (shown when user is not logged in), typically:
   ```html
   <section style="text-align:center;padding:4rem 1rem;">
   ```
   Replace with `<section class="hero">`. Remove inline styles from child `<h1>` and
   `<p>` elements if present — the `.hero h1` and `.hero p` rules in `app.css` handle them.

4. On `<th>` elements that have `style="cursor:pointer;user-select:none;"`, remove the
   inline style. The CSS rule `th[hx-get]` in `app.css` targets sortable headers by
   their HTMX attribute, so no class is needed.

#### job_detail.html (`internal/web/templates/job_detail.html`)

1. Job description block — look for a `<pre style="…">` near the job description field.
   Replace its inline style with `class="job-description"`.

2. Document preview blocks (resume and/or cover letter preview divs) — look for
   `<div style="border:1px solid…">` wrapping iframe/embed or text content.
   Replace with `<div class="document-preview">`.

3. Action buttons (Approve, Reject, or status-change buttons in the detail view) —
   remove inline `style="padding:…;font-size:…;"` and add `class="btn-sm"` alongside
   any existing classes (e.g. `.outline`, `.contrast`).

#### partials/job_rows.html (`internal/web/templates/partials/job_rows.html`)

1. Empty-state row: find the `<td>` with `style="text-align:center;color:#999;"` (or
   similar) and replace with `class="job-table-empty"`.

2. Summary row: find the `<tr>` that holds the AI-generated job summary snippet.
   Add `class="job-summary-row"` to the `<tr>` and remove the inline `style` from its
   `<td>` (the class rule handles padding and font size).

3. Action buttons: remove `style="padding:0.2em 0.6em;font-size:0.8em;"` from each
   action button and add `class="btn-sm"` (keep existing classes like `.outline`).

#### settings.html (`internal/web/templates/settings.html`)

1. Success flash alert div: replace inline background/color style with `class="alert alert-success"`.
   Keep `role="alert"` if present.

2. Muted paragraph: `<p style="color:#999;">` → `<p class="text-muted">`.

3. Filter grid container: `<div style="display:grid;…">` → `<div class="filter-form-grid">`.

4. Remove-filter buttons with small inline styles: add `class="btn-sm"` and remove `style`.

5. NTFY hint `<small style="display:block;margin-top:0.25rem;color:#666;">` →
   `<small class="field-hint">`.

6. Save Notifications button: remove `style="margin-top:0.75rem;"`.

7. Resume textarea: remove `style="font-family:monospace;font-size:0.9em;"` and add
   `class="mono"` (alongside any existing classes).

#### profile.html (`internal/web/templates/profile.html`)

1. Success flash alert: replace inline style with `class="alert alert-success"`.
2. Danger flash alert: replace inline style with `class="alert alert-danger"`.
3. Display-name flex row: `<div style="display:flex;gap:0.5rem;align-items:end;max-width:480px;">` →
   `<div class="inline-form">`.
4. Avatar `<img style="…">` → `<img class="avatar" …>` (keep `src`, `alt`, other attrs).

#### onboarding.html (`internal/web/templates/onboarding.html`)

1. Error div: `style="color:var(--pico-color-red-500);margin-bottom:1rem;"` →
   `class="alert alert-danger"`.
2. Required asterisk span: `style="color:var(--pico-color-red-500)"` → `class="text-danger"`.

## Notes

<!-- Implementing agent fills this in when complete -->
