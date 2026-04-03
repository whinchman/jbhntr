# Task: analytics-3-code-review

- **Type**: code-reviewer
- **Status**: done
- **Parallel Group**: 3
- **Branch**: feature/analytics-2-handlers
- **Source Item**: analytics (plans/analytics.md)
- **Dependencies**: analytics-2-handlers

## Description

Review all code changes introduced by the analytics feature across both
implementation tasks (`analytics-1-store` on branch `feature/analytics-1-store`
and `analytics-2-handlers` on branch `feature/analytics-2-handlers`).

Focus areas:
- SQL correctness and injection safety (parameterised queries only)
- Correct column usage: `status` for `approved`/`rejected`, `application_status`
  for `applied`/`interviewing`/`won`/`lost`
- User-scoping: every query must filter by `user_id = $1`
- `MaxWeekly == 0` guard is present to prevent CSS divide-by-zero
- Backfill logic always produces exactly 12 weekly entries
- `StatsStore` interface is properly satisfied by `*store.Store`
- No hardcoded status strings in SQL — constants from `models.go` or equivalent
- Template XSS safety (all values rendered via `{{.}}` not `{{- . | unescaped}}`)
- No new external Go module dependencies introduced
- Existing tests still pass (`go test ./...`)
- CSS additions follow the existing `app.css` token conventions

## Acceptance Criteria

- [ ] All SQL queries use `$N` placeholders — no string interpolation
- [ ] `GetUserJobStats` correctly uses `status` column for `approved`/`rejected` and `application_status` column for `applied`/`interviewing`/`won`/`lost`
- [ ] Every SQL query includes `WHERE user_id = $1` (user-scoping)
- [ ] Handler guards `MaxWeekly` against 0 before passing to template
- [ ] Handler backfills weekly trend to exactly 12 entries
- [ ] `StatsStore` interface in `server.go` matches the method signatures in `stats.go`
- [ ] No new dependencies in `go.mod`
- [ ] Code reviewer verdict recorded in Notes: `approve` or `request-changes`
- [ ] Any critical or warning findings logged to `.workflow/BUGS.md`

## Interface Contracts

None — this is a review task. All contracts are documented in tasks
`analytics-1-store` and `analytics-2-handlers`.

## Context

- Review branches: `feature/analytics-1-store` and `feature/analytics-2-handlers`
- Plan: `plans/analytics.md`
- Acceptance criteria from the plan (section 6) must all be checkable from
  the code diff alone or via `go test ./...`

## Notes

### Code Review — analytics feature (branches: feature/analytics-1-store, feature/analytics-2-handlers)

**Reviewer:** code-reviewer agent
**Date:** 2026-04-03
**Verdict:** approve

---

## Findings

### [INFO] internal/store/stats.go — SQL injection risk: none confirmed

All SQL queries use `$N` positional placeholders exclusively. `GetUserJobStats`
uses `$1`–`$7`, `GetJobsPerWeek` uses `$1`–`$2`. No string interpolation.
The `($2 * INTERVAL '1 week')` construct in `GetJobsPerWeek` uses a parameterised
integer multiplied against a literal interval — PostgreSQL evaluates this safely
without any injection risk.

### [INFO] internal/store/stats.go — Column usage correct

`GetUserJobStats` correctly uses:
- `status` column with `$2`/`$3` → `string(models.StatusApproved)` / `string(models.StatusRejected)`
- `application_status` column with `$4`–`$7` → `string(models.AppStatusApplied)` etc.

All constants are drawn from `models.go` — no raw strings in SQL.

### [INFO] internal/store/stats.go — User-scoping confirmed

Both `GetUserJobStats` and `GetJobsPerWeek` include `WHERE user_id = $1` and
pass `userID` as the first argument. User isolation is enforced.

### [INFO] internal/web/server.go:1684–1698 — MaxWeekly zero guard present

After the 12-entry backfill loop, the handler checks `if maxWeekly == 0 { maxWeekly = 1 }`.
This prevents a CSS `calc(N / 0 * 160px)` degenerate expression in the bar chart.

### [INFO] internal/web/server.go:1671–1698 — Backfill logic: exactly 12 entries

The loop iterates `i := 0; i < 12; i++` with `startMonday = currentMonday - 11*7 days`.
Each iteration appends one `WeeklyJobCount`, giving exactly 12 entries regardless
of database content. Monday-alignment is computed correctly (Sunday weekday
remapped to 7 before `weekday - 1`).

### [INFO] internal/web/server.go:94–97 — StatsStore interface matches implementation

`StatsStore` declares:
- `GetUserJobStats(ctx, userID int64) (store.UserJobStats, error)` ✓
- `GetJobsPerWeek(ctx, userID int64, weeks int) ([]store.WeeklyJobCount, error)` ✓

Both signatures match `*store.Store` exactly.

### [INFO] internal/web/server.go:423 — /stats route under requireAuth

`r.Get("/stats", s.handleStats)` is inside the `r.Group` block that applies
`r.Use(s.requireAuth)` when `s.sessionStore != nil`. The handler also has its
own redundant nil-user guard (`if user == nil → redirect /login`) providing
defence-in-depth.

### [INFO] cmd/jobhuntr/main.go — WithStatsStore wired correctly

`.WithStatsStore(db)` is chained after `.WithAdminStore(db)` in `main.go`.
The concrete `*store.Store` type satisfies `StatsStore` via the two methods
added in `analytics-1-store`.

### [INFO] internal/web/stats_test.go — Mock completeness

`mockStatsStore` in `stats_test.go` implements both `GetUserJobStats` and
`GetJobsPerWeek`, fully satisfying the `web.StatsStore` interface. No methods
are missing.

### [INFO] go.mod — No new dependencies

`git diff development...feature/analytics-2-handlers -- go.mod go.sum` produced
no output. No new external packages were introduced.

### [INFO] internal/web/templates/stats.html — Template variables match statsData struct

Every field accessed in `stats.html`:
- `.Stats.TotalFound`, `.Stats.TotalApproved`, `.Stats.TotalRejected`,
  `.Stats.TotalApplied`, `.Stats.TotalInterviewing`, `.Stats.TotalWon`,
  `.Stats.TotalLost` → all present in `store.UserJobStats`
- `.WeeklyTrend` → `[]store.WeeklyJobCount` field on `statsData`
- `.Count`, `.WeekStart` (inside `range .WeeklyTrend`) → fields on `store.WeeklyJobCount`
- `$.MaxWeekly` → `MaxWeekly int` field on `statsData`
- `.User` → `*models.User` field on `statsData` (used in layout.html nav guard)

No undefined or misspelled field references found.

### [INFO] internal/web/templates/stats.html — XSS safety

All user-visible integer values (`{{.Stats.TotalFound}}` etc.) are rendered
with `html/template` auto-escaping inside text context. The `style` attribute
`calc({{.Count}} / {{$.MaxWeekly}} * 160px)` uses `int` typed values — Go's
`html/template` CSS sanitizer passes numeric types through unchanged (it only
ZeroOutputs suspicious string values). No `template.HTML`, `template.CSS`, or
unescaped pipe functions are used.

The `{{.WeekStart.Format "Jan 2"}}` call produces a short date string containing
only month abbreviation and a day number (e.g. "Apr 3") — no user-controlled
content.

### [WARNING] internal/web/server.go:1650 — No nil guard for s.statsStore

`handleStats` calls `s.statsStore.GetUserJobStats(...)` without first checking
`if s.statsStore == nil`. If `WithStatsStore` is not called (e.g. in a future
test helper that omits the call), this will panic at runtime. All existing call
sites wire it correctly, but defensive nil-check is absent.

**Suggested fix:** Add `if s.statsStore == nil { http.Error(w, "stats unavailable", 503); return }` at the top of `handleStats`, or alternatively make `WithStatsStore` set a no-op implementation by default.

### [INFO] internal/web/templates/static/app.css — CSS token conventions followed

All CSS variables used in the stats section (`--color-success`, `--color-success-bg`,
`--color-danger`, `--color-danger-bg`, `--color-info`, `--color-info-bg`,
`--color-purple`, `--color-purple-bg`, `--color-teal`, `--color-teal-bg`,
`--color-warning`, `--color-warning-bg`, `--color-accent`, `--color-surface`,
`--color-border`, `--color-text`, `--color-text-muted`, `--radius-md`,
`--radius-sm`, `--shadow-xs`, `--space-*`, `--text-*`, `--weight-*`, `--leading-*`)
are all defined in the existing design-token block (section 1 of app.css). No
new tokens or hardcoded colour values introduced.

### [INFO] internal/store/stats_test.go — Test coverage adequate

Tests cover: empty user (zero counts), multi-status aggregation with all 7
status types, user-scoping (two users see only their own data), empty weekly
result, weekly count aggregation, old jobs excluded by date filter, and
WeekStart timestamp validity. Edge cases are well covered.

---

## Summary

| Severity | Count |
|----------|-------|
| Critical | 0 |
| Warning  | 1 |
| Info     | 12 |

**Overall verdict: approve**

The single warning (missing nil guard for `s.statsStore`) is not a runtime
bug in any current code path — `main.go` always wires the store and the test
uses `WithStatsStore`. It is a defensive-coding concern that could be addressed
in a follow-up cleanup. No blocking issues found.

Acceptance criteria checklist:
- [x] All SQL queries use `$N` placeholders — no string interpolation
- [x] `GetUserJobStats` correctly uses `status` for approved/rejected and `application_status` for applied/interviewing/won/lost
- [x] Every SQL query includes `WHERE user_id = $1`
- [x] Handler guards `MaxWeekly` against 0 before passing to template
- [x] Handler backfills weekly trend to exactly 12 entries
- [x] `StatsStore` interface in `server.go` matches the method signatures in `stats.go`
- [x] No new dependencies in `go.mod`
- [x] Code reviewer verdict recorded in Notes: **approve**
- [x] Critical/warning findings logged to `.workflow/BUGS.md`
