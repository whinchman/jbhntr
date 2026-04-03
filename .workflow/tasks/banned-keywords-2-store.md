# Task: banned-keywords-2-store

- **Type**: coder
- **Status**: pending
- **Parallel Group**: 2
- **Branch**: feature/banned-keywords-2-store
- **Source Item**: Banned Keywords / Companies feature (plans/banned-keywords.md)
- **Dependencies**: banned-keywords-1-migration

## Description

Add the banned-term CRUD store methods to `internal/store/user.go` and extend `ListJobsFilter` in `internal/store/store.go` to support SQL-level banned-term filtering.

These two files are independent of each other (both depend only on the model from task 1) and should be implemented together in this task.

## Acceptance Criteria

- [ ] `internal/store/user.go` contains `ErrDuplicateBannedTerm` sentinel error
- [ ] `CreateUserBannedTerm(ctx, userID, term)` inserts a row; returns `ErrDuplicateBannedTerm` on duplicate via `isUniqueViolation` helper
- [ ] `ListUserBannedTerms(ctx, userID)` returns all terms for the user ordered by `created_at DESC`
- [ ] `DeleteUserBannedTerm(ctx, userID, termID)` deletes the row only if it belongs to the given user; returns an error if not found or mismatched
- [ ] `scanUserBannedTerm` unexported helper correctly scans one `user_banned_terms` row
- [ ] `internal/store/store.go` `ListJobsFilter` has a `BannedTerms []string` field
- [ ] `ListJobs` with non-empty `BannedTerms` generates `NOT ILIKE` clauses for `title`, `company`, and `description`
- [ ] All existing tests continue to pass

## Interface Contracts

Produced by this task — consumed by banned-keywords-3-scheduler and banned-keywords-4-web:

```go
// internal/store/user.go
var ErrDuplicateBannedTerm = errors.New("store: banned term already exists for user")

func (s *Store) CreateUserBannedTerm(ctx context.Context, userID int64, term string) (*models.UserBannedTerm, error)
func (s *Store) ListUserBannedTerms(ctx context.Context, userID int64) ([]models.UserBannedTerm, error)
func (s *Store) DeleteUserBannedTerm(ctx context.Context, userID int64, termID int64) error

// internal/store/store.go — ListJobsFilter addition
BannedTerms []string  // excludes jobs matching any term (case-insensitive substring)
```

## Context

Files to modify:
- `internal/store/user.go` — follow the existing `CreateUserFilter` / `ListUserFilters` / `DeleteUserFilter` patterns; use `isUniqueViolation` (already present at bottom of file) to detect duplicate inserts
- `internal/store/store.go` — `ListJobsFilter` struct and `ListJobs` query builder

`ListJobs` banned-term WHERE clause to add after existing filter construction (from plan):

```go
for _, t := range f.BannedTerms {
    like := "%" + t + "%"
    where = append(where, fmt.Sprintf(
        "(title NOT ILIKE $%d AND company NOT ILIKE $%d AND description NOT ILIKE $%d)",
        argN, argN+1, argN+2))
    args = append(args, like, like, like)
    argN += 3
}
```

`DeleteUserBannedTerm` must scope the delete to the given `userID` to prevent one user from deleting another's terms:

```sql
DELETE FROM user_banned_terms WHERE id = $1 AND user_id = $2
```

## Notes

