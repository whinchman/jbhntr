# Task: auth-task1-model

- **Type**: coder
- **Status**: pending
- **Branch**: feature/auth-task1-model
- **Source Item**: Full Sign-In / Sign-Up Flow
- **Dependencies**: none

## Description

Add the `onboarding_complete` boolean column to the `users` table via a new migration, extend the `models.User` struct, update `store.UpsertUser` and `scanUser` to handle the new column, and add two new store methods: `UpdateUserOnboarding` and `UpdateUserDisplayName`. Extend the `UserStore` interface in `internal/web/server.go` with both new methods.

## Acceptance Criteria

- [ ] `internal/store/migrations/005_add_onboarding_complete.sql` is created with the ALTER TABLE statement and the backfill UPDATE
- [ ] Migration applies cleanly on a fresh DB and on an existing DB with existing users
- [ ] `models.User.OnboardingComplete bool` field is added to `internal/models/user.go`
- [ ] `scanUser` in `internal/store/user.go` includes `onboarding_complete` in the SELECT and scans into `&u.OnboardingComplete`
- [ ] `UpsertUser` INSERT explicitly sets `onboarding_complete = 0`; ON CONFLICT UPDATE does NOT include `onboarding_complete`
- [ ] New users inserted via `UpsertUser` have `OnboardingComplete == false`
- [ ] Existing users with a non-empty `display_name` are backfilled to `onboarding_complete = 1`
- [ ] `UpdateUserOnboarding(ctx, userID, displayName, resume)` sets `display_name`, `resume_markdown`, and `onboarding_complete = 1`
- [ ] `UpdateUserDisplayName(ctx, userID, displayName)` updates `display_name` only
- [ ] `UserStore` interface in `internal/web/server.go` declares both new methods

## Notes

From the architecture plan:

Migration file: `internal/store/migrations/005_add_onboarding_complete.sql`

```sql
ALTER TABLE users ADD COLUMN onboarding_complete INTEGER NOT NULL DEFAULT 0;
UPDATE users SET onboarding_complete = 1 WHERE display_name != '' AND display_name IS NOT NULL;
```

SQLite uses INTEGER for booleans (0/1). The backfill marks users with an existing display_name as already onboarded; users with an empty display_name will be routed through onboarding on next login — this is intentional.

New store method signatures:
```go
func (s *Store) UpdateUserOnboarding(ctx context.Context, userID int64, displayName string, resume string) error
func (s *Store) UpdateUserDisplayName(ctx context.Context, userID int64, displayName string) error
```

The `UserStore` interface (in `internal/web/server.go`) must be extended before Tasks 4 and 5 can compile.
