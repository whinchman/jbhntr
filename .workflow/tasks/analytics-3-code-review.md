# Task: analytics-3-code-review

- **Type**: code-reviewer
- **Status**: pending
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

