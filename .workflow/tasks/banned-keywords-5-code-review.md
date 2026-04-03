# Task: banned-keywords-5-code-review

- **Type**: code-reviewer
- **Status**: pending
- **Parallel Group**: 4
- **Branch**: feature/banned-keywords-4-web
- **Source Item**: Banned Keywords / Companies feature (plans/banned-keywords.md)
- **Dependencies**: banned-keywords-2-store, banned-keywords-3-scheduler, banned-keywords-4-web

## Description

Review all code changes introduced by the banned-keywords feature across the following branches and files:

- `feature/banned-keywords-1-migration` — migration SQL + model struct
- `feature/banned-keywords-2-store` — store CRUD methods + `ListJobsFilter` extension
- `feature/banned-keywords-3-scheduler` — `UserFilterReader` interface extension + `filterBannedJobs`
- `feature/banned-keywords-4-web` — web layer handlers, routes, template

Check for bugs, security issues, logic errors, and code-standard violations. Pay particular attention to:
- SQL injection safety in the dynamic `NOT ILIKE` clause builder
- User-scoping on `DeleteUserBannedTerm` (must not allow cross-user deletes)
- Interface breaking changes — all mocks must implement the new method
- Error handling consistency (duplicate term, blank term, missing term on delete)

## Acceptance Criteria

- [ ] All changed files reviewed against acceptance criteria in each task file
- [ ] No critical or high-severity security issues (SQL injection, missing auth checks, CSRF gaps)
- [ ] `DeleteUserBannedTerm` verified to scope by `userID`
- [ ] `NOT ILIKE` clause builder uses parameterised arguments (no string interpolation of user data)
- [ ] Mock implementations of `UserFilterReader` and `FilterStore` are updated everywhere
- [ ] Findings written to Notes section below; critical/warning items added to `.workflow/BUGS.md`
- [ ] Verdict: `approve` or `request-changes`

## Interface Contracts

N/A — this task reviews, does not produce interfaces.

## Context

Architecture plan: `plans/banned-keywords.md`

Key files to review:
- `internal/store/migrations/011_add_user_banned_terms.sql`
- `internal/models/user.go`
- `internal/store/user.go`
- `internal/store/store.go`
- `internal/scraper/scheduler.go`
- `internal/web/server.go`
- `internal/web/templates/settings.html`

## Notes

