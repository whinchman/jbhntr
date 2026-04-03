# Bugs

Known bugs discovered by QA and Code Reviewer agents. Each bug should have
enough detail for a Coder agent to reproduce and fix it.

Bugs here follow the same approval flow as features — the stakeholder moves
approved fixes to TODO.md (removing them from this file).

---

## BUG-016: [email-auth] SMTPMailer sends HTML email with Content-Type: text/plain

- File: `internal/mailer/mailer.go`, line 88
- Severity: warning
- Description: `SMTPMailer.SendMail` hardcodes `Content-Type: text/plain; charset=UTF-8` in the MIME headers but the email body passed to it is a rendered HTML document (from `templates/email/verify_email.html` and `templates/email/reset_password.html`). Most email clients will render the HTML as raw escaped text rather than rendered HTML. Users will see `<h1>` tags and inline styles in the email body instead of a rendered button.
- Reproduction: Configure SMTP, register a new user, inspect the verification email received — the body is shown as raw HTML source.
- Fix: Change the `Content-Type` header in `SMTPMailer.SendMail` from `text/plain` to `text/html`:
  ```go
  msg := fmt.Sprintf(
      "From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n%s",
      m.from, to, subject, body,
  )
  ```

---

## BUG-017: [email-auth] isUniqueViolation uses fragile string matching instead of pgx error type

- File: `internal/store/user.go`, line 449–451
- Severity: warning
- Description: `isUniqueViolation` detects PostgreSQL unique constraint violations by calling `strings.Contains(err.Error(), "23505") || strings.Contains(err.Error(), "unique")`. The second clause (`"unique"`) is overly broad — it would match any error message containing the word "unique" (e.g., a constraint named "unique_user_email" in an unrelated error message). The correct approach with pgx/v5 is to use the typed error: `var pgErr *pgconn.PgError; errors.As(err, &pgErr) && pgErr.Code == "23505"`. This is both precise and resistant to error-message formatting changes.
- Reproduction: N/A (latent — currently works but fragile).
- Fix: Import `github.com/jackc/pgx/v5/pgconn` and replace the string-matching implementation with:
  ```go
  func isUniqueViolation(err error) bool {
      var pgErr *pgconn.PgError
      return errors.As(err, &pgErr) && pgErr.Code == "23505"
  }
  ```

---

## BUG-018: [email-auth] migrate_test.go does not include migration 009

- File: `internal/store/migrate_test.go` (line count of expected slice)
- Severity: warning
- Description: The existing `TestMigrate/applies_all_migrations` test (BUG-014 noted migration 008 was missing) will also be missing `009_add_email_auth.sql` now that that migration has been added. When `TEST_DATABASE_URL` is set the test will fail with `migrations applied = 9, want N` (where N is the hardcoded expected count).
- Reproduction: Set `TEST_DATABASE_URL`, run `go test ./internal/store/... -run TestMigrate`.
- Fix: Add `"009_add_email_auth.sql"` to the `expected` slice in `TestMigrate`.

---

## BUG-019: [email-auth] CSS duplicate definition — .login-card-footer defined twice in app.css

- File: `internal/web/templates/static/app.css`, lines 628–708 (Section 12) and lines 801–874 (Section 12 additions)
- Severity: info
- Description: The login page CSS is split into two blocks both labelled "Section 12". The first block (lines 628–708) defines `.login-card`, `.login-card-header`, `.provider-btn`, etc. The second block (lines 801–874, labelled "Section 12 — LOGIN PAGE additions") re-defines `.login-card-footer`, `.login-forgot-link`, `.field-error`, `.verify-email-body`, and the responsive breakpoint. The `.login-card-footer` rules in particular appear in both blocks (the second block overrides the first). While this produces correct final output due to CSS cascade, the duplication is confusing and should be consolidated into one Section 12 block.
- Reproduction: N/A (visual/cosmetic).
- Fix: Merge the "Section 12 — LOGIN PAGE additions" block into the original "12. LOGIN PAGE" block and remove the duplicate `.login-card-footer` definition from the first block.

---

## BUG-014: migrate_test.go hardcodes 7 migrations but migration 008 now exists

**Severity:** Warning (test failure — will break `go test ./internal/store/...` on a live DB)
**File:** `internal/store/migrate_test.go`, lines 41–58
**Found by:** QA agent (resume-export QA pass)
**Branch:** feature/resume-export-3-routes

### Description

`TestMigrate/applies_all_migrations` builds an expected list of 7 migration file names (001–007) and asserts `len(names) == len(expected)`. Migration `008_add_markdown_columns.sql` was added by the foundation task (feature/resume-export-1-foundation) but `migrate_test.go` was NOT updated to include it. When this test runs against a live PostgreSQL database (`TEST_DATABASE_URL` set), it will fail with:
```
migrations applied = 8, want 7
```

### Fix

Add `"008_add_markdown_columns.sql"` to the `expected` slice in `TestMigrate` (line 58 becomes line 59):

```go
expected := []string{
    "001_create_users.sql",
    "002_create_user_search_filters.sql",
    "003_add_user_id_to_jobs.sql",
    "004_rebuild_jobs_unique_constraint.sql",
    "005_add_onboarding_complete.sql",
    "006_rebuild_jobs_drop_legacy_unique.sql",
    "007_add_ntfy_topic_to_users.sql",
    "008_add_markdown_columns.sql",
}
```

---

## BUG-015: job_detail.html inline style= regressions — CSS classes replaced with inline styles

**Severity:** Warning (code style regression; inline styles bypass the design system)
**File:** `internal/web/templates/job_detail.html`, lines 25–26, 29–30, 36, 42, 56
**Found by:** QA agent (resume-export QA pass)
**Branch:** feature/resume-export-3-routes

### Description

The coder replaced CSS classes with inline `style=` attributes in job_detail.html as part of the resume-export-3-routes implementation. Specifically:

1. **Lines 25–26, 29–30**: Approve and Reject buttons changed from `class="outline contrast btn-sm"` / `class="outline btn-sm"` to `class="outline contrast" style="padding:0.3em 0.8em;"` / `class="outline" style="padding:0.3em 0.8em;"`. The `btn-sm` utility class was removed and replaced with a hardcoded inline style.

2. **Line 36**: The job description `<pre>` changed from `class="job-description"` to `style="white-space:pre-wrap;font-size:0.9em;background:#f9f9f9;padding:1rem;border-radius:0.3em;"`. 

3. **Lines 42, 56**: The content preview `<div>` changed from `class="document-preview"` to `style="border:1px solid #ccc;border-radius:0.3em;padding:1rem;margin-bottom:0.5rem;overflow:auto;"`.

These changes inline visual styling into the template, making it harder to restyle globally and inconsistent with the rest of the codebase which uses `app.css` classes.

### Fix

Revert inline styles in job_detail.html to class-based styling. The CSS classes `btn-sm`, `job-description`, and `document-preview` may need to be added/restored in `app.css` if they were removed. Alternatively, add the styles as utility classes to `app.css`.

---

## BUG-013: Test panic — body[:4] accessed when len(body) < 4 in DOCX download tests

**Severity:** Warning
**File:** `internal/web/server_test.go`, lines 1225 and 1287
**Found by:** Code Reviewer agent (resume-export-3-routes review)
**Branch:** feature/resume-export-3-routes

### Description

In `TestDownloadResumeDocx` and `TestDownloadCoverDocx`, the magic-bytes check guards with `len(body) < 2` but the `t.Errorf` format string contains `body[:4]`. If `len(body)` is 0, 1, 2, or 3 — which happens whenever the server returns a short error body — the slice expression `body[:4]` panics with an index-out-of-range, killing the test process instead of reporting a clean test failure.

### Reproduction

1. Make the `exporter.ToDocx` function return an error (e.g. by temporarily stubbing it).
2. Run `go test ./internal/web/... -run TestDownloadResumeDocx`.
3. Observe a panic: `runtime error: slice bounds out of range [:4] with length <n>` (where n < 4).

### Fix

Change line 1224 (and the equivalent at line 1286) from:
```go
if len(body) < 2 || body[0] != 'P' || body[1] != 'K' {
    t.Errorf("body does not look like a DOCX/ZIP file (first bytes: %v)", body[:4])
}
```
to:
```go
if len(body) < 4 || body[0] != 'P' || body[1] != 'K' {
    t.Errorf("body does not look like a DOCX/ZIP file (first bytes: %v)", body)
}
```
This aligns the guard with the format verb and avoids the out-of-bounds access.

---

## BUG-012: parseInline treats `_` as italic delimiter inside words, corrupting identifiers with underscores

**Severity:** Warning
**File:** `internal/exporter/docx.go`, lines 133–141
**Found by:** Code Reviewer agent (resume-export-2-exporter review)
**Branch:** feature/resume-export-2-exporter

### Description

`parseInline` does not check word boundaries before treating `_` as an italic delimiter. It searches for any subsequent `_` and treats the text between them as italic. This silently italicises portions of plain text that happen to contain two underscores, which is common in technical resume content (package names, environment variables, CLI flags, etc.).

### Reproduction

Call `parseInline("Technologies: node_modules, my_project")`. The result contains an italic span with text `"modules, my"`. The text `"node"` and `"_project"` are rendered as plain text; `"modules, my"` is rendered italic. The DOCX output will display `"Technologies: node_` *modules, my* `_project"`.

Similar examples:
- `"role_arn and secret_access_key"` → "arn and secret" italicised
- `"snake_case_name"` → "case" italicised

### Fix

Before treating `_` at position `i` as an opening italic delimiter, verify it is at a word boundary. Simplest approach: add a guard `i == 0 || !isAlphaNum(text[i-1])` where `isAlphaNum` checks `(c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')`. This matches common Markdown rules (CommonMark requires that `_`-delimiters cannot be left-flanking when preceded by a Unicode alphanumeric character).

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

