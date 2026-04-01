# Bugs

Known bugs discovered by QA and Code Reviewer agents. Each bug should have
enough detail for a Coder agent to reproduce and fix it.

Bugs here follow the same approval flow as features — the stakeholder moves
approved fixes to TODO.md (removing them from this file).

---

## BUG-001: Legacy UNIQUE(external_id, source) prevents per-user job dedup

**Severity:** Medium
**File:** `internal/store/migrations/003_add_user_id_to_jobs.sql`, `internal/store/store.go` (baseline schema)
**Related task:** task1-schema-migration
**Found by:** QA agent

### Description

The baseline schema defines `UNIQUE(external_id, source)` on the `jobs` table.
Migration 003 adds a new `UNIQUE INDEX(user_id, external_id, source)` for
per-user dedup. However, SQLite cannot drop the old two-column constraint via
`ALTER TABLE`. Since `CreateJob` uses `INSERT OR IGNORE`, the old constraint
fires first and silently drops inserts where the same `external_id` + `source`
combination already exists — even if the `user_id` is different.

This means two different users cannot independently discover the same job listing.
The second user's insert is silently ignored.

### Reproduction

1. Create two users (u1, u2).
2. `CreateJob(ctx, u1.ID, job)` with `external_id="X", source="serpapi"` -- succeeds.
3. `CreateJob(ctx, u2.ID, job)` with same `external_id="X", source="serpapi"` -- returns `inserted=false` (silently dropped).

### Expected behavior

Each user should have their own copy of the job. The three-column unique index
`(user_id, external_id, source)` should be the only dedup constraint.

### Fix

A new migration must rebuild the `jobs` table without the old two-column
`UNIQUE(external_id, source)` constraint, using the standard SQLite
create-new / copy / drop-old / rename approach. This is a data migration
that should be done carefully with tests.

## BUG-002: Settings handlers panic if filterStore is nil

**Severity:** Low
**File:** `internal/web/server.go` (lines 515-618)
**Related task:** task3-peruser-routes
**Found by:** Code Reviewer agent

### Description

The settings handlers (`handleSettings`, `handleSaveResume`, `handleAddFilter`,
`handleRemoveFilter`) call methods on `s.filterStore` without checking if it is
nil. If a `Server` is constructed via `NewServer()` or `NewServerWithConfig()`
with a nil `FilterStore`, navigating to `/settings` causes a nil pointer
dereference panic.

Currently no code path triggers this because:
- Production (`main.go`) always passes `db` as the filter store.
- Tests either use `newSettingsServer` (which provides a mock) or `newServer`
  (which never hits settings routes).

### Fix

Add a nil guard at the start of each settings handler:
```go
if s.filterStore == nil {
    http.Error(w, "settings not configured", http.StatusServiceUnavailable)
    return
}
```

Or register settings routes conditionally (only when `filterStore != nil`).

## BUG-003: Migration 004 PRAGMA foreign_keys is no-op inside transaction

**Severity:** Low
**File:** `internal/store/migrations/004_rebuild_jobs_unique_constraint.sql`, `internal/store/migrate.go`
**Related task:** task4-multiuser-scraper
**Found by:** Code Reviewer agent

### Description

Migration 004 includes `PRAGMA foreign_keys = OFF` at the start and
`PRAGMA foreign_keys = ON` at the end. However, `migrate.go:runMigration`
executes all migration SQL inside a database transaction (`tx.Exec`). SQLite
ignores `PRAGMA foreign_keys` changes inside a transaction -- the pragma value
is unchanged.

This is currently harmless because no tables reference `jobs` via foreign keys,
so `DROP TABLE jobs` succeeds regardless. But if a future migration adds FK
references to the `jobs` table, a table-rebuild migration would fail unless the
migrate runner is updated to handle PRAGMAs outside the transaction.

### Impact

None currently. The migration works correctly because no FKs reference `jobs`.

### Fix options

1. Remove the PRAGMAs from migration 004 (they do nothing) and add a comment
   explaining why they are not needed.
2. Or update `migrate.go:runMigration` to detect and execute PRAGMAs outside
   the transaction boundary before beginning the migration transaction.

## BUG-004: runFilter exceeds 50-line function guideline

**Severity:** Low (code standard)
**File:** `internal/scraper/scheduler.go` (lines 130-210)
**Related task:** pre-existing (before task4)
**Found by:** Code Reviewer agent

### Description

The `runFilter` method is 81 lines, exceeding the project code standard of
"keep functions under 50 lines where practical." This was pre-existing
(83 lines before task4) and not introduced by task4.

### Fix

Extract the summarization loop (lines 181-195) and the notification loop
(lines 197-207) into separate helper methods like `summarizeNewJobs` and
`notifyNewJobs`.

