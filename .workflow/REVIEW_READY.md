# Review Ready: modern-design

**Date**: 2026-04-03
**Feature**: Update design look to be more modern
**Plan**: .workflow/plans/modern-design.md
**Design spec**: .workflow/plans/modern-design-spec.md

## Summary of Changes

**`internal/web/templates/static/app.css`** (new, 799 lines) — single stylesheet overriding PicoCSS v2 custom properties with a fresh token set. Indigo-600 accent, neutral off-white background, 7 status badge variants, full component coverage (nav, tables, forms, buttons, cards, alerts, login page, hero section).

**`internal/web/server.go`** — added `"io/fs"` import and `/static/*` route serving embedded CSS publicly (before auth subrouters).

**`layout.html` + `login.html`** — inline `<style>` blocks removed, `/static/app.css` link added, all elements updated to semantic CSS classes.

**`dashboard.html`, `job_detail.html`, `partials/job_rows.html`, `settings.html`, `profile.html`, `onboarding.html`** — all `style=""` attributes removed, replaced with CSS classes.

## Validation Summary

| Check | Result |
|-------|--------|
| Code Review (tasks 1, 2, 3) | PASS — 0 critical, 0 warnings |
| QA | PASS |

## Known Issues

**BUG-010** (Low/cosmetic): `.providers-section` class used in `login.html:23` has no rule in `app.css`. Slight top-margin loss above provider buttons on the login page. Fix: add `.providers-section { margin-top: var(--space-4); }` to app.css login section (section 12).

## To Ship

The feature is merged to `development` locally. When satisfied:

```
git push origin development
```
