# Bugs

## [FIXED] `internal/web/server.go` — Card deck shows non-actionable jobs, approve button silently fails

**Feature**: tinder-style-mobile
**Severity**: bug (approve button appears broken)

**Description**:
Card deck `ExcludeStatuses` only excluded `rejected`, so `approved`, `generating`, `complete`, and `failed` jobs appeared in the deck. `handleApproveJob` rejects these with 409 Conflict. HTMX does not swap on non-200 responses, so the button appeared to do nothing.

Also, `dashboard.html` passed the full job list (used by the desktop table) to `job_cards`, so the initial card deck render also showed non-actionable jobs.

**Fix**:
- Added `CardDeck jobRowsData` to `dashboardData` with a separate query excluding all non-actionable statuses
- Updated `dashboard.html` to use `{{template "job_cards" .CardDeck}}`
- Changed `handleJobCardsPartial` filter to exclude all non-actionable statuses
- Changed `respondJobAction` card-deck branch to exclude all non-actionable statuses and clear any status filter from the URL

---

## [WARNING] `internal/web/server.go` ~line 1211 — Double `ListJobs` call in `respondJobAction` for card-deck requests

**Feature**: tinder-style-mobile
**Task**: tinder-mobile-backend
**Branch**: feature/tinder-mobile-backend
**Severity**: warning (performance/clarity; output is correct)

**Description**:
When `HX-Target` is `job-card-deck`, `respondJobAction` calls `s.store.ListJobs` twice:
1. Once before the card-deck branch, without `ExcludeStatuses` (result is assigned to `data` but then overwritten).
2. Once inside the card-deck branch, with `f.ExcludeStatuses = [StatusRejected]`.

The first call's result is discarded. The rendered output is correct (only the second call's results are used), but the wasted DB round-trip could cause a visible delay on slower connections, and the code is misleading.

**Reproduction**:
Approve or reject a job on mobile (HTMX POST with `HX-Target: job-card-deck`). Two `ListJobs` queries execute where one would suffice.

**Suggested fix**:
Move the `HX-Target` check before the first `ListJobs` call. Set `f.ExcludeStatuses` conditionally:

```go
f := store.ListJobsFilter{ ... }
if r.Header.Get("HX-Target") == "job-card-deck" {
    f.ExcludeStatuses = []models.JobStatus{models.StatusRejected}
}
jobs, err := s.store.ListJobs(r.Context(), userID, f)
// ... error handling ...
if r.Header.Get("HX-Target") == "job-card-deck" {
    s.templates.ExecuteTemplate(w, "job_cards", ...)
    return
}
s.templates.ExecuteTemplate(w, "job_rows", ...)
```

**Priority**: low — correct behavior, minor inefficiency.

---

## [INFO] `internal/web/templates/static/swipe-cards.js` — `commitCard` missing `card` argument to `submitAction`

**Feature**: tinder-style-mobile
**Task**: tinder-mobile-qa (JS static review)
**Branch**: feature/tinder-mobile-frontend
**Severity**: info (no functional impact in practice)

**Description**:
`commitCard(card, direction)` calls `submitAction(direction)` without passing `card`. `submitAction` then searches `document.getElementById('job-card-deck')` for `form[data-action="${direction}"]`. Since there is only one active card in the deck at a time and only the active card has `data-action` forms, the selector always finds the correct form. However, if the `#job-card-deck` element were absent from the DOM (e.g., after an unexpected HTMX swap), `submitAction` returns early silently (line 158: `if (!deck) return`) with no error feedback.

**Impact**: None under normal usage. In degraded DOM states, the swipe submission would silently fail with no user feedback.

**Suggested fix** (low priority): Pass `card` to `submitAction` and search within `card.closest('#job-card-deck')` rather than `document.getElementById`.

---

## [INFO] `internal/web/templates/static/swipe-cards.js` — `window.matchMedia` not guarded against undefined in `commitCard`

**Feature**: tinder-style-mobile
**Task**: tinder-mobile-qa (JS static review)
**Branch**: feature/tinder-mobile-frontend
**Severity**: info (very old browsers only)

**Description**:
Line 134: `var reducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches;`

`window.matchMedia` is undefined in very old browsers (IE 9 and below). If called in such a context, this throws `TypeError: window.matchMedia is not a function`, preventing the fly-off animation and form submission entirely.

**Impact**: None for any target mobile browser. The feature is explicitly progressive-enhancement mobile-first. IE 9 is not a supported target.

**Suggested fix**: Guard with `if (window.matchMedia && window.matchMedia('...').matches)`. Low priority.
