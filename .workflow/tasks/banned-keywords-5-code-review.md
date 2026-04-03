# Task: banned-keywords-5-code-review

- **Type**: code-reviewer
- **Status**: done
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

### Review Summary

Reviewed all four branches: migration, store, scheduler, web. No critical
security issues found. Two warning-level findings and two info-level observations.

**Verdict: approve** (no blocking issues; warnings are low-risk but should
be addressed in a follow-up or the current sprint.)

---

## Findings

### [WARNING] internal/web/server.go ~line 1197 — Fragile duplicate-term error detection

`handleAddBannedTerm` detects the duplicate-banned-term error with a string
comparison:

```go
if strings.Contains(err.Error(), "already exists") {
```

`store.ErrDuplicateBannedTerm` is a sentinel error (`errors.New(...)`) that is
returned unwrapped from `CreateUserBannedTerm`. The `store` package is already
imported in `server.go`. The check should be:

```go
if errors.Is(err, store.ErrDuplicateBannedTerm) {
```

Using `strings.Contains` will match any error whose message contains "already
exists", including unrelated database errors, and will silently swallow them
as duplicates rather than reporting them to the user.

Fix: replace the string check with `errors.Is(err, store.ErrDuplicateBannedTerm)`.

---

### [WARNING] internal/web/server.go ~line 503 — Silent error discard in handleDashboard

`ListUserBannedTerms` is called with `bannedTerms, _ :=` — the error is
silently discarded with no log entry. If the database is unavailable, the
dashboard will show all jobs with no banned-term filtering applied, and the
user receives no indication.

The scheduler uses the same non-fatal approach, but it at least logs the error
via `s.logger.Error(...)`. The dashboard handler has no equivalent log.

Fix: replace the blank identifier with a named error variable and log it:

```go
bannedTerms, btErr := s.filterStore.ListUserBannedTerms(r.Context(), userID)
if btErr != nil {
    slog.Warn("failed to load banned terms for dashboard", "user_id", userID, "error", btErr)
}
bannedTermStrings = bannedTermsToStrings(bannedTerms)
```

---

### [INFO] internal/store/user.go ~line 558 — CreateUserBannedTerm uses client-side timestamp

`CreateUserBannedTerm` only returns `id` via `RETURNING id`, then sets
`bt.CreatedAt = time.Now().UTC()` on the client side instead of reading the
database-assigned value. This means the returned struct's `CreatedAt` can
drift slightly from what is stored, and could be incorrect in tests that
stub time or compare against the DB value.

This is consistent with other patterns in the codebase and is low risk since
`created_at` is only used for display ordering. No action required unless
precision matters; consider `RETURNING id, created_at` in a follow-up.

---

### [INFO] internal/scraper/scheduler.go ~line 178 — In-place slice mutation in filterBannedJobs

`out := jobs[:0]` reuses the backing array of the input slice. This is safe
at the only current call site (fresh `results` from `Search()`, reassigned
and not held by any other reference), but may be surprising to future
maintainers. The in-place approach avoids an allocation which is the likely
intent. No change required unless the function is reused in contexts where
the original slice might be observed after the call.

---

## Acceptance Criteria Check

- [x] All changed files reviewed against acceptance criteria in each task file
- [x] No critical or high-severity security issues (SQL injection, missing auth checks, CSRF gaps)
  - SQL: `NOT ILIKE` clauses use parameterized `$N` placeholders; user data is
    placed only in `args`, never interpolated into the SQL string. Safe.
  - Auth: `DeleteUserBannedTerm` SQL is `WHERE id = $1 AND user_id = $2` —
    ownership enforced at the DB layer. Safe.
  - CSRF: `hx-post` Remove buttons use the HTMX global `hx-headers` mechanism
    (set in layout.html via `document.body.setAttribute('hx-headers', ...)`)
    to send `X-CSRF-Token` on every HTMX POST. gorilla/csrf checks this header
    by default. Safe. The Add form additionally has an explicit hidden input.
- [x] `DeleteUserBannedTerm` verified to scope by `userID` — confirmed via SQL and test
- [x] `NOT ILIKE` clause builder uses parameterised arguments — confirmed
- [x] Mock implementations of `UserFilterReader` and `FilterStore` updated everywhere
  - `mockUserFilterReader` in `scheduler_test.go`: `ListUserBannedTerms` added
  - `mockUserFilterReaderWithBannedError` variant: `ListUserBannedTerms` added
  - `filteredTermsReader` (new type): all three methods implemented
  - `mockFilterStore` in `server_test.go`: all three methods added
- [x] Findings written to Notes section; critical/warning items added to `.workflow/BUGS.md`
- [x] Verdict: **approve**

