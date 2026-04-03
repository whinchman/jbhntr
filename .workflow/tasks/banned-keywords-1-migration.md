# Task: banned-keywords-1-migration

- **Type**: coder
- **Status**: pending
- **Parallel Group**: 1
- **Branch**: feature/banned-keywords-1-migration
- **Source Item**: Banned Keywords / Companies feature (plans/banned-keywords.md)
- **Dependencies**: none

## Description

Create the database migration for the `user_banned_terms` table, and add the `UserBannedTerm` model to `internal/models/user.go`.

These two steps are grouped together because the model is trivially small and all downstream tasks (store CRUD, scheduler, web layer) depend on both the migration and the model existing before they can proceed.

## Acceptance Criteria

- [ ] Migration file `internal/store/migrations/012_add_user_banned_terms.sql` exists and is correct
- [ ] Migration creates table `user_banned_terms` with columns: `id BIGSERIAL PK`, `user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE`, `term TEXT NOT NULL`, `created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`
- [ ] Migration creates `UNIQUE (user_id, term)` constraint
- [ ] Migration creates index `idx_user_banned_terms_user` on `user_id`
- [ ] `internal/models/user.go` contains `UserBannedTerm` struct with fields: `ID int64`, `UserID int64`, `Term string`, `CreatedAt time.Time`
- [ ] All existing tests continue to pass

## Interface Contracts

None — this task produces the foundational schema and model that all other tasks consume.

## Context

File to create:
- `internal/store/migrations/012_add_user_banned_terms.sql` (NEW)

File to modify:
- `internal/models/user.go` — add `UserBannedTerm` alongside the existing `UserSearchFilter` struct

Migration SQL (exact content from plan):

```sql
CREATE TABLE IF NOT EXISTS user_banned_terms (
    id         BIGSERIAL PRIMARY KEY,
    user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    term       TEXT   NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, term)
);
CREATE INDEX IF NOT EXISTS idx_user_banned_terms_user ON user_banned_terms(user_id);
```

Model struct to add to `internal/models/user.go`:

```go
// UserBannedTerm is one entry in a user's banned-keywords list.
type UserBannedTerm struct {
    ID        int64
    UserID    int64
    Term      string
    CreatedAt time.Time
}
```

The migrate.go / migrate_test.go files embed and run migrations automatically — dropping the SQL file in the migrations directory is sufficient; no changes to migrate.go are needed.

## Notes

