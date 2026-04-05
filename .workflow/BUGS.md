# Bugs

## [WARNING] `internal/web/server.go` ~line 1211 — Double `ListJobs` call in `respondJobAction` for card-deck requests

**Feature**: tinder-style-mobile
**Task**: tinder-mobile-backend
**Branch**: feature/tinder-mobile-backend
**Severity**: warning (performance/clarity; output is correct)

**Description**:
When `HX-Target` is `job-card-deck`, `respondJobAction` calls `s.store.ListJobs` twice:
1. Once before the card-deck branch, without `ExcludeStatuses` (result is assigned to `data` but then overwritten).
2. Once inside the card-deck branch, with `f.ExcludeStatuses = [StatusRejected]`.

The first call's result is discarded. The rendered output is correct (only the second call's results are used), but the wasted DB round-trip could cause a visible delay on slower connections, and the code is misleading.

**Reproduction**:
Approve or reject a job on mobile (HTMX POST with `HX-Target: job-card-deck`). Two `ListJobs` queries execute where one would suffice.

**Suggested fix**:
Move the `HX-Target` check before the first `ListJobs` call. Set `f.ExcludeStatuses` conditionally:

```go
f := store.ListJobsFilter{ ... }
if r.Header.Get("HX-Target") == "job-card-deck" {
    f.ExcludeStatuses = []models.JobStatus{models.StatusRejected}
}
jobs, err := s.store.ListJobs(r.Context(), userID, f)
// ... error handling ...
if r.Header.Get("HX-Target") == "job-card-deck" {
    s.templates.ExecuteTemplate(w, "job_cards", ...)
    return
}
s.templates.ExecuteTemplate(w, "job_rows", ...)
```

**Priority**: low — correct behavior, minor inefficiency.
