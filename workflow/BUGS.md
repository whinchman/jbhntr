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

