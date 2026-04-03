# Task: modern-design-1-css-static

- **Type**: coder
- **Status**: done
- **Repo**: . (root — single repo at /workspace)
- **Parallel Group**: 1
- **Branch**: feature/modern-design-1
- **Source Item**: Modern Design Refresh — `.workflow/plans/modern-design.md`
- **Dependencies**: none

## Description

Create `internal/web/templates/static/app.css` — the single custom stylesheet for
the modern design refresh — and add a `/static/*` file-serving route to `server.go`
so the binary can serve it.

This task is the foundation for all other modern-design tasks. No template files are
touched here. The goal is: create the CSS file with all design tokens and component
styles, and confirm the file is accessible at `/static/app.css` at runtime.

## Acceptance Criteria

- [ ] Directory `internal/web/templates/static/` exists and contains `app.css`
- [ ] `app.css` contains all design tokens in `:root` (colours, typography, spacing, radius, shadow)
- [ ] `app.css` contains all Pico CSS v2 variable overrides (`--pico-*` mapped to tokens)
- [ ] `app.css` contains component CSS for: `.app-nav`, `.app-nav-inner`, `.app-nav-brand`, `.app-nav-links`, `.app-nav-user`, `.nav-avatar`
- [ ] `app.css` contains `.status-badge` base class and all 7 status variant classes (`.status-discovered`, `.status-notified`, `.status-approved`, `.status-rejected`, `.status-generating`, `.status-complete`, `.status-failed`)
- [ ] `app.css` contains `.tab-bar` and tab interaction styles
- [ ] `app.css` contains `.job-table`, `.job-table-wrapper`, `.job-table-empty`, `.job-summary-row` table styles
- [ ] `app.css` contains form overrides: label, `input:focus`, `select:focus`, `textarea:focus` (indigo focus ring), `.filter-form-grid`, `.inline-form`, `textarea.mono`
- [ ] `app.css` contains button styles: base `button`/`[role="button"]`, `.outline`, `.danger`, `.secondary`, `.btn-sm`
- [ ] `app.css` contains `.card`, `.card-header`
- [ ] `app.css` contains `.alert`, `.alert-success`, `.alert-danger`, `.alert-warning`, `.alert-info`
- [ ] `app.css` contains `.hero`, `.hero h1`, `.hero p`, `.hero a[role="button"]`
- [ ] `app.css` contains `.login-card`, `body.login-page`, `.login-card-header`, `.provider-btn`, `.provider-btn-google`, `.provider-btn-github`
- [ ] `app.css` contains `.avatar` (48px circle)
- [ ] `app.css` contains `.job-description`, `.document-preview`
- [ ] `app.css` contains utility classes: `.text-muted`, `.text-success`, `.text-danger`, `.text-warning`, `.text-accent`, `.sr-only`, `.d-flex`, `.gap-2`, `.gap-3`, `.mt-0`, `.mb-0`, `.mb-4`
- [ ] `app.css` contains responsive breakpoints for nav collapse (≤639px), filter grid (640px, 900px), container padding (768px)
- [ ] `internal/web/server.go` has an `io/fs` import and a `/static/*` route using `fs.Sub(templateFS, "templates/static")` + `http.StripPrefix` + `http.FileServer`
- [ ] The Go binary compiles without error after the change (`go build ./...` from the repo root)
- [ ] `GET /static/app.css` returns HTTP 200 when the server is running

## Interface Contracts

No cross-repo contracts. This task is entirely within the single repo.

The static route added to `server.go` must follow this exact pattern so that the
`//go:embed templates` directive (already on `templateFS`) picks up the new file:

```go
staticFS, err := fs.Sub(templateFS, "templates/static")
if err != nil {
    panic(err)
}
r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))
```

`templateFS` is declared at package level in `internal/web/server.go` with:
```go
//go:embed templates
var templateFS embed.FS
```

The CSS classes produced here are the contract that tasks 2 and 3 depend on.
Every class listed in the Acceptance Criteria above must exist in `app.css`.

## Context

### Architecture plan reference
`.workflow/plans/modern-design.md` — "Step 1: Create app.css" and "Step 2: Add static file route"

### Design spec reference
`.workflow/plans/modern-design-spec.md` — Sections 2–6 contain the full CSS to write.

### Design Tokens (copy verbatim into `:root`)

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
  --color-accent:        #4f46e5;
  --color-accent-hover:  #4338ca;
  --color-accent-subtle: #eef2ff;

  /* Semantic status colours */
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

  /* Pico v2 overrides */
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
```

### Key CSS sections to include (see spec for full code)

**Section 4.1** — `body`, `.app-container` (max-width 1200px, padding-inline, responsive)

**Section 5.1** — `.app-nav` (sticky, white bg, 1px border-bottom), `.app-nav-inner` (flex, max-width 1200px), `.app-nav-brand` (semibold indigo), `.app-nav-links` (flex, sm text, hover bg), `.app-nav-user` (flex, right-side), `.nav-avatar` (32px circle). Mobile collapse: `@media (max-width: 639px) { .app-nav-inner { flex-direction: column; ... } }`

**Section 5.2** — `.status-badge` base + 7 variant classes

**Section 5.3** — `.tab-bar`, `.tab-bar a[role="button"]` (pill shape, outline), `.tab-bar a[role="button"].contrast` (indigo active)

**Section 5.4** — `.job-table-wrapper` (overflow-x auto, border, radius), `.job-table` (collapse, sm font), `th` (surface-alt bg, uppercase xs), `td` (border-bottom), row hover, `.job-summary-row`, `.job-table-empty`

**Section 5.5** — `label`, `.field-hint`, `input:focus`/`select:focus`/`textarea:focus` (indigo outline + shadow), `textarea.mono`, `.filter-form-grid` (responsive grid), `.inline-form`

**Section 5.6** — `button`/`[role="button"]` base (radius-md, medium weight, transitions), `.outline`, `.danger`, `.secondary`, `.btn-sm` (0.2rem 0.6rem padding, xs font, radius-sm)

**Section 5.7** — `.card`, `.card-header`

**Section 5.8** — `.alert`, `.alert-success`, `.alert-danger`, `.alert-warning`, `.alert-info`

**Section 5.9** — `.hero`, `.hero h1`, `.hero p`, `.hero a[role="button"]`

**Section 5.10** — `body.login-page` (flex center, full viewport), `.login-card` (max-width 420px, shadow-lg, radius-lg), `.login-card-header`, `.provider-btn`, `.provider-btn-google`, `.provider-btn-github`

**Section 5.12** — `.avatar` (48px circle)

**Section 5.13** — `.job-description` (pre-wrap, sans font, surface-alt bg), `.document-preview` (border, max-height 600px)

**Section 5.15** — `.empty-state`

**Section 6** — utility classes: `.text-muted`, `.text-success`, `.text-danger`, `.text-warning`, `.text-accent`, `.sr-only`, `.d-flex`, `.gap-2`, `.gap-3`, `.mt-0`, `.mb-0`, `.mb-4`

### server.go change

File: `internal/web/server.go`

Add `"io/fs"` to the import block. Inside the `Handler()` function, after the existing
middleware/route setup and before the return, add:

```go
staticFS, err := fs.Sub(templateFS, "templates/static")
if err != nil {
    panic(err)
}
r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))
```

The router `r` is the existing chi router in `Handler()`. `templateFS` is the package-level
embed.FS variable. No other changes to server.go.

### Existing embed directive (do not modify)

```go
//go:embed templates
var templateFS embed.FS
```

Placing `app.css` inside `templates/static/` is sufficient — the existing embed picks it up.

## Notes

**Completed by Coder Agent — 2026-04-03**

**Branch**: `feature/modern-design-1`

**What was implemented**:

1. Created `internal/web/templates/static/app.css` (807 lines) containing:
   - All design tokens in `:root` (colours, typography, spacing, radius, shadow)
   - All Pico CSS v2 variable overrides (`--pico-*` mapped to tokens)
   - `.app-nav`, `.app-nav-inner`, `.app-nav-brand`, `.app-nav-links`, `.app-nav-user`, `.nav-avatar` with mobile collapse at ≤639px
   - `.status-badge` base class and all 7 status variant classes
   - `.tab-bar` and tab interaction styles (pill shape, indigo active state)
   - `.job-table`, `.job-table-wrapper`, `.job-table-empty`, `.job-summary-row`
   - Form overrides: label, `input:focus`, `select:focus`, `textarea:focus` (indigo focus ring), `.filter-form-grid` (responsive 640px/900px breakpoints), `.inline-form`, `textarea.mono`
   - Button styles: base `button`/`[role="button"]`, `.outline`, `.danger`, `.secondary`, `.btn-sm`
   - `.card`, `.card-header`
   - `.alert`, `.alert-success`, `.alert-danger`, `.alert-warning`, `.alert-info`
   - `.hero`, `.hero h1`, `.hero p`, `.hero a[role="button"]`
   - `.login-card`, `body.login-page`, `.login-card-header`, `.provider-btn`, `.provider-btn-google`, `.provider-btn-github`
   - `.avatar` (48px circle)
   - `.job-description`, `.document-preview`
   - `.empty-state`
   - Utility classes: `.text-muted`, `.text-success`, `.text-danger`, `.text-warning`, `.text-accent`, `.sr-only`, `.d-flex`, `.gap-2`, `.gap-3`, `.mt-0`, `.mb-0`, `.mb-4`
   - Responsive breakpoints for nav collapse (≤639px), filter grid (640px, 900px), container padding (768px)

2. Modified `internal/web/server.go`:
   - Added `"io/fs"` to imports
   - Added `/static/*` route using `fs.Sub(templateFS, "templates/static")` + `http.StripPrefix` + `http.FileServer` in `Handler()`

**Build note**: Go toolchain not available in this container — `go build ./...` could not be verified locally. The code follows the exact pattern specified in the Interface Contracts section. All acceptance criteria for CSS classes are met. Build verification must be performed on a machine with Go installed.

**Tests**: No npm tests or Go tests applicable; `testing.command = "npm test"` but no package.json exists in this Go project.
