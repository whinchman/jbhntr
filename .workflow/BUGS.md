# Bugs

Known bugs discovered by QA and Code Reviewer agents. Each bug should have
enough detail for a Coder agent to reproduce and fix it.

Bugs here follow the same approval flow as features — the stakeholder moves
approved fixes to TODO.md (removing them from this file).

---

## BUG-011: godocx added as unused indirect dependency — will be stripped by go mod tidy

**Severity:** Warning
**File:** `go.mod`, `go.sum`
**Found by:** Code Reviewer agent (resume-export-1-foundation review)
**Branch:** feature/resume-export-1-foundation

### Description

`github.com/gomutex/godocx v0.1.5` (and its transitive test deps `stretchr/testify v1.9.0`, `davecgh/go-spew v1.1.1`, `pmezard/go-difflib v1.0.0`) were added to `go.mod` / `go.sum` as `// indirect` entries in this task. No source or test file in the codebase imports these packages. Running `go mod tidy` will remove all four entries, causing a diff on the next toolchain run and potentially a CI failure if tidy is enforced.

### Reproduction

1. Ensure Go toolchain is available.
2. Run `go mod tidy` on `feature/resume-export-1-foundation`.
3. Observe that the four new entries are removed from `go.mod` and their corresponding lines removed from `go.sum`.

### Fix

Remove `github.com/gomutex/godocx`, `github.com/stretchr/testify`, `github.com/davecgh/go-spew`, and `github.com/pmezard/go-difflib` from `go.mod` and `go.sum` on this branch. Re-add `gomutex/godocx` (with `go get github.com/gomutex/godocx`) in the future task that first imports it for DOCX generation.

---

## BUG-010: `.providers-section` used in login.html but not defined in app.css

**Severity:** Low (minor visual spacing regression)
**File:** `internal/web/templates/login.html` (line 23), `internal/web/templates/static/app.css`
**Found by:** QA agent (modern-design feature)
**Branch:** feature/modern-design-2

### Description

`login.html` uses `<div class="providers-section">` as a wrapper around the OAuth provider buttons. The original `login.html` had an inline `<style>` with `.providers-section { margin-top: 1rem; }`. That inline style was removed as part of task modern-design-2, but the `.providers-section` class rule was not migrated to `app.css`.

As a result the provider buttons appear with slightly tighter top margin than the original design (they render directly below the flash-alert / card header with no explicit top spacing). The buttons are functional and render correctly otherwise.

### Reproduction

1. Navigate to `/login`
2. Observe the gap between the card header / flash alert and the "Sign in with Google" / "Sign in with GitHub" buttons — it is smaller than intended

### Fix

Add one of the following to `app.css` in the Login Page section (around line 682):

```css
.providers-section {
  margin-top: var(--space-4);
}
```

Or, alternatively, remove the `.providers-section` wrapper `<div>` from `login.html` entirely and rely on `.provider-btn`'s own `margin-bottom` spacing between buttons.

---

## ~~BUG-008: CSRF token missing from settings.html plain-HTML forms~~ FIXED

**Severity:** High (blocks core functionality)
**File:** `internal/web/templates/settings.html` (lines 49, 72)
**Found by:** User

### Description

The "Add Filter" form (`POST /settings/filters`) and "Save Resume" form (`POST /settings/resume`) are plain HTML `<form>` elements with no CSRF hidden field. The `hx-headers` CSRF injection in `layout.html` only applies to HTMX requests — it does not inject into standard form submissions. gorilla/csrf rejects both POSTs with "Forbidden - CSRF token not found in request".

### Reproduction

1. Log in and navigate to `/settings`
2. Fill in the Add Filter form and submit — `403 Forbidden`
3. Edit the resume and submit — `403 Forbidden`

### Fix

Add `<input type="hidden" name="gorilla.csrf.Token" value="{{.CSRFToken}}">` inside both `<form>` elements. The `CSRFToken` field is already populated in the settings template data struct.

---

## ~~BUG-009: Status filter resets after Approve/Reject on dashboard~~ FIXED

**Severity:** Medium
**File:** `internal/web/templates/dashboard.html`
**Found by:** User

### Description

Clicking a status filter tab uses HTMX to swap `#job-table-body` without pushing
a URL change. The polling div's `hx-get` URL is baked at server-render time
(`status={{.ActiveStatus}}`), so it always points to the initially-loaded filter
(empty = all). When Approve/Reject swaps a `<tr>` inside the polling div, HTMX
resets the polling interval and it fires with the stale unfiltered URL, replacing
`#job-table-body` with all jobs and discarding the active filter.

### Fix

1. Add `hx-push-url="true"` to each filter tab `<a>` so the URL stays in sync
   with the active filter.
2. Update the polling div to derive query params from the current URL at poll time
   using `hx-vals="js:{...}"` instead of server-rendered static values.

---

## ~~BUG-001: Legacy UNIQUE(external_id, source) prevents per-user job dedup~~ FIXED

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

## ~~BUG-002: Settings handlers panic if filterStore is nil~~ FIXED

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

## ~~BUG-003: Migration 004 PRAGMA foreign_keys is no-op inside transaction~~ FIXED

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

## ~~BUG-004: runFilter exceeds 50-line function guideline~~ FIXED

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

---

## ~~BUG-005: TestProtectedRoutes_Unauthenticated tests stale route expectations~~ FIXED

**Severity:** Warning
**File:** `internal/web/auth_test.go` (lines 681–721)
**Related task:** auth-task6-dashboard-auth
**Found by:** Code Reviewer agent

### Description

`TestProtectedRoutes_Unauthenticated` asserts that unauthenticated `GET /` and
`GET /partials/job-table` return a `303 → /login` redirect. However,
auth-task6 moved both routes from the `requireAuth` group to the `optionalAuth`
group. Unauthenticated requests to `/` now return `200` with a hero section, and
`/partials/job-table` returns `200` with an empty fragment. Both test cases will
fail when the test suite is run.

### Reproduction

Run `npm test` (or `go test ./internal/web/...`). The sub-tests:
- `GET / redirects to /login`
- `GET /partials/job-table redirects to /login`
will report `status = 200, want 303`.

### Expected behavior

Tests should reflect the new optionalAuth behavior:
- `GET /` unauthenticated → `200 OK` (hero section)
- `GET /partials/job-table` unauthenticated → `200 OK` (empty fragment)

### Fix

Remove `{http.MethodGet, "/"}` and `{http.MethodGet, "/partials/job-table"}`
from the `routes` slice in `TestProtectedRoutes_Unauthenticated`. Add two new
test cases (or extend `TestPublicRoutes_NoAuth`) asserting that each returns
`200 OK` without a session cookie.

---

## ~~BUG-006: OAuth state token not cleared from session on error paths~~ FIXED

**Severity:** Warning
**File:** `internal/web/auth.go` (lines 297–318)
**Related task:** auth-task2-login-polish / auth-task3-return-to
**Found by:** Code Reviewer agent

### Description

In `handleOAuthCallback`, the oauth state is deleted from the in-memory session
object on line 297 (`delete(sess.Values, oauthStateName)`), but the session is
never saved back to the response cookie on any error path before `setSession` is
called. The affected error paths are:

1. Provider returned `error` query param (lines 300–308)
2. Authorization code exchange failed (lines 313–318)
3. Provider user-info fetch failed (lines 322–327)

On these paths the session cookie still contains the `oauth_state` value. An
attacker or a replayed request could reuse the same state parameter on a
subsequent callback request.

In practice exploitation requires the attacker to also hold the original session
cookie, so the actual risk is low. But the intent of deleting the state is to
prevent reuse and that intent is not honored on failure paths.

### Reproduction

1. Start an OAuth flow to store state in session.
2. Hit `/auth/google/callback?state=<valid-state>&error=access_denied`.
3. Observe the session cookie — the `oauth_state` value is still present.
4. A second request to the callback with the same state will pass state validation.

### Fix

After `delete(sess.Values, oauthStateName)`, save the session to the response
before any early return:

```go
delete(sess.Values, oauthStateName)
sess.Options.Path = "/"
_ = sess.Save(r, w) // consume the state token before any early return
```

---

## ~~BUG-007: error_description from OAuth provider used directly as flash message~~ FIXED

**Severity:** Warning
**File:** `internal/web/auth.go` (lines 303–305)
**Related task:** auth-task2-login-polish
**Found by:** Code Reviewer agent

### Description

The `error_description` query parameter received from the OAuth provider in the
callback URL is set verbatim as the flash message and then rendered in the login
template. While Go's `html/template` auto-escapes the value before rendering,
the content is still fully attacker-controlled (any party who controls the
redirect URL could craft a misleading or phishing message).

More importantly, if any future code path treats the flash message as trusted
HTML (e.g. `template.HTML(msg)`), this becomes a direct XSS vector. The current
code is safe, but the pattern is fragile.

### Fix

Do not use provider-supplied `error_description` as a user-facing message.
Use the fixed string `"Sign-in was cancelled or denied. Please try again."` in
all cases, and log the provider's description at the `Warn` level only (which
is already done on line 302).

```go
if errMsg := r.URL.Query().Get("error"); errMsg != "" {
    slog.Warn("oauth error from provider", "provider", providerName, "error", errMsg,
        "description", r.URL.Query().Get("error_description"))
    s.setFlash(w, r, "Sign-in was cancelled or denied. Please try again.")
    http.Redirect(w, r, "/login", http.StatusSeeOther)
    return
}
```

