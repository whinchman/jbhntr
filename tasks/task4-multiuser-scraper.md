# Task: task4-multiuser-scraper

**Type:** coder
**Status:** done
**Priority:** 2
**Epic:** oauth-multi-user
**Depends On:** task1-schema-migration

## Description

Update the scheduler/scraper to iterate per-user search filters instead of global config. New jobs are created with the correct `user_id`. Deduplication becomes per-user.

### What to build:

1. **Scheduler updates** (`internal/scraper/scheduler.go`):
   - Instead of reading filters from global config, query all users' search filters from DB
   - For each user's filter set, run the scraper and create jobs with that user's ID
   - Replace all `userID=0` TODO(task4) call sites with the real user ID

2. **Deduplication**:
   - Per-user dedup: same external job can exist for multiple users
   - Address BUG-001: the legacy `UNIQUE(external_id, source)` constraint blocks per-user dedup
   - May need a migration to drop the old constraint or work around it

3. **Worker updates** (`internal/generator/worker.go`):
   - Replace `userID=0` TODO call sites if any remain
   - Worker processes jobs regardless of user (userID=0 sentinel is correct here)

4. **Main.go wiring** (`cmd/jobhuntr/main.go`):
   - Pass store to scheduler for user-filter lookup

## Acceptance Criteria

- [x] Scheduler queries per-user filters from DB
- [x] Scheduler creates jobs with correct `user_id` per user
- [x] Deduplication is per-user (same external job can exist for different users)
- [x] BUG-001 addressed (legacy UNIQUE constraint handled)
- [x] Worker correctly processes jobs across all users
- [x] No remaining `userID=0` TODO(task4) comments
- [x] `go build ./...` succeeds
- [x] `go test ./...` passes

## Context

- Existing scheduler: `internal/scraper/scheduler.go`
- Existing worker: `internal/generator/worker.go`
- User filter store methods from task1: `internal/store/user.go`
- BUG-001: Legacy `UNIQUE(external_id, source)` blocks per-user dedup
- See full plan: `plans/oauth-multi-user.md` (section 4)

## Design

Full design document: `plans/task4-multiuser-scraper-design.md`

### Summary of Approach

1. **BUG-001 fix:** New migration `004_rebuild_jobs_unique_constraint.sql` rebuilds the `jobs` table without the legacy `UNIQUE(external_id, source)` constraint using SQLite's create-new/copy/drop/rename pattern. Also update baseline schema in `store.go` for consistency.

2. **New store method:** `ListActiveUserIDs()` returns distinct user IDs from `user_search_filters` (only users with filters get scraped).

3. **New interface:** `UserFilterReader` (in scheduler.go) with `ListActiveUserIDs` and `ListUserFilters` methods.

4. **Scheduler rewrite:** Replace static `[]models.SearchFilter` field with `UserFilterReader`. `RunOnce` iterates active users, gets their filters, converts `UserSearchFilter` to `SearchFilter`, and passes `userID` through to all store calls. Error handling changes to continue-on-error (one user's failure does not block others).

5. **Worker:** No logic changes. Replace `TODO(task3)` comments with explanatory comments about why `userID=0` is intentional (unscoped worker query).

6. **main.go:** Remove config-based filter construction. Pass `db` as both `StoreWriter` and `UserFilterReader` to the scheduler constructor.

### Files Changed

- `internal/store/migrations/004_rebuild_jobs_unique_constraint.sql` (new)
- `internal/store/store.go` (baseline schema update)
- `internal/store/user.go` (add `ListActiveUserIDs`)
- `internal/scraper/scheduler.go` (major refactor)
- `internal/scraper/scheduler_test.go` (update mocks + new tests)
- `internal/generator/worker.go` (comment cleanup only)
- `cmd/jobhuntr/main.go` (wiring changes)

## Notes

Implementation completed on branch `feature/task4-multiuser-scraper` (commit a0cba7d).

### Changes made (10 files, +390 -75 lines):
- **Migration 004**: Table rebuild removes legacy `UNIQUE(external_id, source)`, fixes BUG-001
- **store.go**: Baseline schema updated (removed inline UNIQUE)
- **user.go**: Added `ListActiveUserIDs` method
- **scheduler.go**: Added `UserFilterReader` interface, rewrote `RunOnce` for per-user iteration, added `userFilterToSearchFilter` helper, replaced all `TODO(task4)` with real userID
- **scheduler_test.go**: New mock `UserFilterReader`, 7 test cases covering per-user dedup, continue-on-error, userID passthrough
- **worker.go**: Replaced `TODO(task3)` with explanatory `userID=0` comments (no logic changes)
- **main.go**: Removed config-based filter construction, passes `db` as `UserFilterReader`
- **migrate_test.go**: Updated expected migrations list
- **store_test.go**: Updated BUG-001 test to assert fix
- **user_test.go**: Added `TestListActiveUserIDs` with 3 subtests

### Behavioral changes:
- `RunOnce` no longer returns early on individual filter/user errors; it logs and continues
- Scheduler reads filters from DB instead of config; `SearchFilters` config section unused by scheduler
- Optional ScrapeRun user_id enhancement deferred (section 7 of design doc)
