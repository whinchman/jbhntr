# Task: tinder-mobile-code-review

- **Type**: code-reviewer
- **Status**: pending
- **Parallel Group**: 2
- **Branch**: feature/tinder-mobile-backend (review both feature branches)
- **Source Item**: Tinder-Style Mobile Job Review UI
- **Dependencies**: tinder-mobile-backend, tinder-mobile-frontend

## Description

Review all code changes introduced by the `tinder-mobile-backend` and `tinder-mobile-frontend` tasks. Both branches touch the same repository so review them together. Check for correctness, security issues (CSRF handling, auth bypass risk on the new endpoint), accessibility of the new HTML, JS correctness, and Go code quality.

## Acceptance Criteria

- [ ] `handleJobCardsPartial` correctly handles unauthenticated requests (returns 200 empty, not a redirect)
- [ ] `respondJobAction` card-deck branch does not regress table behaviour (verify `HX-Target: job-table-body` still renders `job_rows`)
- [ ] CSRF token is correctly included in both approve and reject forms in `job_cards.html`
- [ ] `hx-swap="innerHTML"` (not `outerHTML`) is used on action forms — prevents deck element from being removed after first action
- [ ] `job_cards.html` uses `$.CSRFToken` (root context access) inside `{{with}}` blocks
- [ ] `job_cards.html` uses `{{if not .Jobs}}` correctly (nil slice and empty slice both evaluate truthy for `not`)
- [ ] Ghost cards have `aria-hidden="true"` and `inert` attributes
- [ ] Active card has `role="article"`, `tabindex="0"`, and descriptive `aria-label`
- [ ] `swipe-cards.js` uses Pointer Events API (not deprecated Touch Events)
- [ ] `swipe-cards.js` checks `prefers-reduced-motion` before applying CSS transitions
- [ ] `swipe-cards.js` gracefully degrades when `window.htmx` is not available
- [ ] CSS does not modify any rules outside the new `/* 11. SWIPE CARDS */` section
- [ ] CSS breakpoint rules correctly show/hide table vs card deck at 639/640px boundary
- [ ] `go build ./...` passes cleanly on the backend branch
- [ ] No security issues with the new route (e.g., data leaked to unauthenticated callers)
- [ ] All findings logged in Notes section; critical and warning findings added to `.workflow/BUGS.md`

## Interface Contracts

No cross-repo contracts. Review that internal contracts between backend and frontend are honored:
- Template named `"job_cards"` in `job_cards.html` matches the name called in `handleJobCardsPartial` and `respondJobAction`
- `jobRowsData` struct fields `.Jobs` and `.CSRFToken` are accessed correctly in the template
- Route `/partials/job-cards` matches the `hx-get` attribute in `dashboard.html`

## Context

Branches to review:
- `feature/tinder-mobile-backend` — changes in `internal/web/server.go`
- `feature/tinder-mobile-frontend` — new files `internal/web/templates/partials/job_cards.html`, `internal/web/templates/static/swipe-cards.js`; modified files `internal/web/templates/static/app.css`, `internal/web/templates/dashboard.html`

Architecture plan: `.workflow/plans/tinder-style-mobile.md`
Design spec: `.workflow/plans/tinder-style-mobile-design.md`

Key risks to check:
1. **Template name collision** — ensure `"job_cards"` does not conflict with any existing defined template name
2. **HX-Target detection** — the header value is `job-card-deck` (no `#` prefix, despite the form using `hx-target="#job-card-deck"`)
3. **Double polling on desktop** — the card deck polls even on desktop (it's just CSS-hidden). Verify the plan acknowledges this and it's intentional (it is — see plan §8)
4. **`{{block "scripts"}}` override** — `dashboard.html` overrides the layout's empty scripts block; verify `layout.html` has the corresponding `{{block "scripts" .}}{{end}}`

## Notes

<!-- reviewer fills in findings and verdict when complete -->
