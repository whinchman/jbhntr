# Task: banned-keywords-3-scheduler

- **Type**: coder
- **Status**: pending
- **Parallel Group**: 3
- **Branch**: feature/banned-keywords-3-scheduler
- **Source Item**: Banned Keywords / Companies feature (plans/banned-keywords.md)
- **Dependencies**: banned-keywords-2-store

## Description

Extend the scraper scheduler in `internal/scraper/scheduler.go` to:

1. Add `ListUserBannedTerms` to the `UserFilterReader` interface
2. Fetch each user's banned terms once per scrape run (in `RunOnce`)
3. Pass banned terms through to `runFilter`
4. Add the pure helper function `filterBannedJobs` that drops jobs matching any banned term (case-insensitive substring)

This is the scrape-time filtering layer — it prevents banned jobs from ever being stored in the database.

## Acceptance Criteria

- [ ] `UserFilterReader` interface includes `ListUserBannedTerms(ctx context.Context, userID int64) ([]models.UserBannedTerm, error)`
- [ ] `RunOnce` calls `ListUserBannedTerms` per user after `ListUserFilters`; errors are logged and treated as non-fatal (proceed with `nil` terms)
- [ ] `runFilter` signature accepts `bannedTerms []models.UserBannedTerm` as the last argument
- [ ] `filterBannedJobs` is called inside `runFilter` before the `CreateJob` loop
- [ ] `filterBannedJobs` performs case-insensitive substring matching on `Title`, `Company`, and `Description`
- [ ] When `bannedTerms` is empty, `filterBannedJobs` returns the input slice unchanged
- [ ] All existing mock implementations of `UserFilterReader` (in test files) are updated to add the new method
- [ ] All existing tests continue to pass

## Interface Contracts

Consumed from banned-keywords-2-store:

```go
// Store method signature (internal/store/user.go)
func (s *Store) ListUserBannedTerms(ctx context.Context, userID int64) ([]models.UserBannedTerm, error)

// Model (internal/models/user.go)
type UserBannedTerm struct {
    ID        int64
    UserID    int64
    Term      string
    CreatedAt time.Time
}
```

Produced by this task (interface extension — all mocks must implement this):

```go
type UserFilterReader interface {
    ListActiveUserIDs(ctx context.Context) ([]int64, error)
    ListUserFilters(ctx context.Context, userID int64) ([]models.UserSearchFilter, error)
    // NEW:
    ListUserBannedTerms(ctx context.Context, userID int64) ([]models.UserBannedTerm, error)
}
```

## Context

File to modify: `internal/scraper/scheduler.go`

This is a breaking interface change — search for all files that implement `UserFilterReader` (mock structs in test files) and add the `ListUserBannedTerms` stub to each.

`filterBannedJobs` pure helper (from plan — implement exactly as specified):

```go
func filterBannedJobs(jobs []models.Job, terms []models.UserBannedTerm) []models.Job {
    if len(terms) == 0 {
        return jobs
    }
    lower := make([]string, len(terms))
    for i, t := range terms {
        lower[i] = strings.ToLower(t.Term)
    }
    out := jobs[:0]
    for _, j := range jobs {
        titleL := strings.ToLower(j.Title)
        companyL := strings.ToLower(j.Company)
        descL := strings.ToLower(j.Description)
        banned := false
        for _, t := range lower {
            if strings.Contains(titleL, t) ||
                strings.Contains(companyL, t) ||
                strings.Contains(descL, t) {
                banned = true
                break
            }
        }
        if !banned {
            out = append(out, j)
        }
    }
    return out
}
```

Note: `strings` is already imported in the scheduler. The `out := jobs[:0]` idiom reuses the underlying array to avoid an allocation.

## Notes

