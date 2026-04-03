# Architecture Plan: Admin Panel

## Overview

A web-based admin interface mounted at `/admin` and protected by HTTP Basic Auth
using a single `ADMIN_PASSWORD` environment variable (username hardcoded to `"admin"`).
The panel provides user management (list, ban/unban, manual password reset) and
analytics (search filter listing, aggregate stats).

No database schema changes are required for the core feature, with one exception:
the users table needs a `banned_at` nullable timestamp column added via a new
migration to represent the banned state.

---

## Architecture Overview

### Approach: Inline admin package inside `internal/web`

The admin panel is a self-contained sub-package (`internal/web/admin`) with its own
handler struct, store interface, templates, and middleware. It is wired into the
existing `chi` router in `internal/web/server.go` under the `/admin` route group.

**Why not a separate package at `internal/admin`?**  
The existing `internal/web` package already owns the chi router, template FS, and
all HTTP plumbing. Keeping admin as a sub-package of `web` lets it share the chi
middleware chain and the existing `store.Store` concrete type without introducing
circular imports. The sub-package pattern (`internal/web/admin`) mirrors how Go
standard library packages subdivide (`net/http`, `net/url`, etc.) and keeps admin
code visually isolated without coupling concerns.

### HTTP Basic Auth middleware

Go's `net/http` has no built-in Basic Auth middleware; the existing codebase does
not use one either. We implement a small `adminAuth` middleware inline in the admin
package — it reads `Authorization: Basic ...` header, base64-decodes it, and
compares credentials using `subtle.ConstantTimeCompare` to prevent timing attacks.
The password is pulled from `cfg.Admin.Password` (see Config Changes below).

If `ADMIN_PASSWORD` is empty at startup, the server logs a warning and the `/admin`
routes are still registered but every request returns 401 — preventing accidental
open access while not crashing the server.

### Store interface additions

The admin panel needs several read-only and mutation queries that do not exist yet.
These are added to `internal/store/user.go` (and one aggregation to `store.go`) and
exposed via a new `AdminStore` interface defined in the admin package.

### Template strategy

Admin templates live in `internal/web/admin/templates/` and are embedded with their
own `//go:embed` directive on the `adminHandler` struct. They do **not** extend the
existing `layout.html` (which loads user session state, has a user nav bar, etc.).
Instead, the admin templates use a minimal `admin_layout.html` that includes PicoCSS
from CDN (same as the existing layout) plus a small admin nav. This avoids polluting
the user-facing template namespace and keeps admin HTML self-contained.

No CSRF protection is applied to the admin routes. HTTP Basic Auth is a
credential-per-request scheme and admin form submissions are protected by the
password requirement on every request. Adding gorilla/csrf would require injecting
the session secret and a cookie, which is unnecessary complexity for a single-operator
admin panel.

### Password reset (temp password)

"Manually reset a user's password" means:
1. Generate a cryptographically random 12-character alphanumeric temp password.
2. Hash it with bcrypt (cost 12, same as existing auth).
3. Call `SetPasswordHash(ctx, userID, hash)` — a new store method.
4. Display the plaintext temp password on a result page (single render, not stored).
5. Clear any existing reset_token / reset_expires_at for the user so the old token
   can no longer be used.

The user must change this password on next login — no "force change" mechanism is
implemented (out of scope); the operator is expected to communicate the temp password
to the user through a side channel.

---

## Files Affected

### New files

| File | Purpose |
|------|---------|
| `internal/web/admin/admin.go` | `adminHandler` struct, constructor, `adminAuth` middleware, route registration |
| `internal/web/admin/handlers.go` | All HTTP handler methods |
| `internal/web/admin/store.go` | `AdminStore` interface definition |
| `internal/web/admin/templates/admin_layout.html` | Minimal admin HTML shell |
| `internal/web/admin/templates/admin_dashboard.html` | Stats + quick-links landing page |
| `internal/web/admin/templates/admin_users.html` | User list table |
| `internal/web/admin/templates/admin_filters.html` | Filters list table |
| `internal/store/migrations/010_add_banned_at_to_users.sql` | Add `banned_at TIMESTAMPTZ` column |

### Modified files

| File | Change |
|------|--------|
| `internal/store/user.go` | Add `ListAllUsers`, `BanUser`, `UnbanUser`, `SetPasswordHash`, `ListAllFilters` methods |
| `internal/store/store.go` | Add `AdminStats` struct and `GetAdminStats` method |
| `internal/config/config.go` | Add `AdminConfig` struct and `Admin AdminConfig` field to `Config` |
| `internal/web/server.go` | Mount admin sub-router under `/admin` in `Handler()` |
| `config.yaml.example` | Document new `admin.password` config key |

---

## New Routes

All routes are prefixed with `/admin` and protected by the `adminAuth` middleware.

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/admin` | `handleAdminDashboard` | Stats landing page |
| GET | `/admin/users` | `handleAdminUsers` | List all users |
| POST | `/admin/users/{id}/ban` | `handleAdminBanUser` | Ban a user |
| POST | `/admin/users/{id}/unban` | `handleAdminUnbanUser` | Unban a user |
| POST | `/admin/users/{id}/reset-password` | `handleAdminResetPassword` | Generate temp password |
| GET | `/admin/filters` | `handleAdminFilters` | List all search filters across all users |

---

## Store Interface Additions

### New interface in `internal/web/admin/store.go`

```go
// AdminStore is the subset of store.Store used by the admin panel.
type AdminStore interface {
    ListAllUsers(ctx context.Context) ([]models.User, error)
    BanUser(ctx context.Context, userID int64) error
    UnbanUser(ctx context.Context, userID int64) error
    SetPasswordHash(ctx context.Context, userID int64, hash string) error
    ListAllFilters(ctx context.Context) ([]AdminFilter, error)
    GetAdminStats(ctx context.Context) (store.AdminStats, error)
}

// AdminFilter is a filter row joined with the owning user's email.
type AdminFilter struct {
    models.UserSearchFilter
    UserEmail string
}
```

### New methods on `*store.Store` in `internal/store/user.go`

```go
func (s *Store) ListAllUsers(ctx context.Context) ([]models.User, error)
// SELECT <userSelectCols> FROM users ORDER BY created_at DESC

func (s *Store) BanUser(ctx context.Context, userID int64) error
// UPDATE users SET banned_at = NOW() WHERE id = $1

func (s *Store) UnbanUser(ctx context.Context, userID int64) error
// UPDATE users SET banned_at = NULL WHERE id = $1

func (s *Store) SetPasswordHash(ctx context.Context, userID int64, hash string) error
// UPDATE users SET password_hash = $1, reset_token = NULL, reset_expires_at = NULL WHERE id = $2
```

### New method on `*store.Store` in `internal/store/store.go`

```go
type AdminStats struct {
    TotalUsers      int
    TotalJobs       int
    TotalFilters    int
    NewUsersLast7d  int
}

func (s *Store) GetAdminStats(ctx context.Context) (AdminStats, error)
// Four scalar queries in one function (can be a single SQL block with CTEs or four separate queries)
```

### New method on `*store.Store` in `internal/store/user.go`

```go
func (s *Store) ListAllFilters(ctx context.Context) ([]AdminFilter, error)
// SELECT usf.id, usf.user_id, usf.keywords, usf.location, usf.min_salary, usf.max_salary, usf.title, usf.created_at, u.email
// FROM user_search_filters usf
// JOIN users u ON u.id = usf.user_id
// ORDER BY usf.created_at DESC
```

Note: `AdminFilter` is a value type defined in the admin package; the store method
returns `[]admin.AdminFilter` — but to avoid the import cycle (`store` importing
`admin`), we define a parallel type `store.AdminFilter` in `store/user.go` and have
the admin package convert or alias it. The cleanest approach is to define
`AdminFilter` in `internal/store/user.go` since it is a store-layer concern and the
admin package simply consumes it.

Revised: `store.AdminFilter` is defined in `internal/store/user.go`:

```go
// AdminFilter is a UserSearchFilter joined with the owning user's email,
// used by the admin panel.
type AdminFilter struct {
    models.UserSearchFilter
    UserEmail string
}
```

The `AdminStore` interface in the admin package references `store.AdminFilter`.

---

## Models Change

### Migration: `internal/store/migrations/010_add_banned_at_to_users.sql`

```sql
ALTER TABLE users ADD COLUMN IF NOT EXISTS banned_at TIMESTAMPTZ;
```

### `models.User` struct — add `BannedAt` field

Add to `internal/models/user.go`:

```go
BannedAt *time.Time // nil = not banned
```

### `scanUser` in `internal/store/user.go`

- Add `banned_at` to `userSelectCols`:
  ```
  ..., reset_token, reset_expires_at, banned_at
  ```
- Add a `var bannedAt sql.NullString` and scan/parse it, setting `u.BannedAt`.

---

## Config Changes

### `internal/config/config.go`

Add:

```go
// AdminConfig holds admin panel settings.
type AdminConfig struct {
    // Password is the HTTP Basic Auth password for the /admin panel.
    // Set via the ADMIN_PASSWORD environment variable.
    // Username is hardcoded to "admin".
    // If empty, the admin panel returns 401 on every request.
    Password string `yaml:"password"`
}
```

Add to `Config` struct:

```go
Admin AdminConfig `yaml:"admin"`
```

### `config.yaml.example`

Add section:

```yaml
admin:
  password: "${ADMIN_PASSWORD}"  # HTTP Basic Auth password for /admin panel; username is "admin"
```

### `cmd/jobhuntr/main.go`

No change to startup code needed — the `cfg` struct is passed to `web.NewServerWithConfig`
and the admin router is wired in `Handler()`. The admin password comes from
`cfg.Admin.Password`.

---

## Template Structure

### `internal/web/admin/templates/admin_layout.html`

Minimal HTML shell:

```html
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>{{block "title" .}}Admin — JobHuntr{{end}}</title>
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/@picocss/pico@2/css/pico.min.css">
</head>
<body>
  <nav>
    <ul>
      <li><strong>JobHuntr Admin</strong></li>
    </ul>
    <ul>
      <li><a href="/admin">Dashboard</a></li>
      <li><a href="/admin/users">Users</a></li>
      <li><a href="/admin/filters">Filters</a></li>
    </ul>
  </nav>
  <main class="container">
    {{block "content" .}}{{end}}
  </main>
</body>
</html>
```

### Template data types (defined in `internal/web/admin/handlers.go`)

```go
type adminDashboardData struct {
    Stats store.AdminStats
}

type adminUsersData struct {
    Users   []models.User
    TempPassword string // non-empty only immediately after a reset
    ResetUserID  int64
}

type adminFiltersData struct {
    Filters []store.AdminFilter
}
```

### `admin_dashboard.html`

Shows four stat cards (Total Users, Total Jobs, Total Filters, New Users (7d)) plus
nav links to Users and Filters tables.

### `admin_users.html`

Table: ID | Email | Display Name | Created At | Banned | Actions

Actions per row:
- If not banned: `<form method="POST" action="/admin/users/{id}/ban"><button>Ban</button></form>`
- If banned: `<form method="POST" action="/admin/users/{id}/unban"><button>Unban</button></form>`
- Always: `<form method="POST" action="/admin/users/{id}/reset-password"><button>Reset Password</button></form>`

After a password reset, the page re-renders with a flash block showing the temp
password. Template receives `TempPassword` string (non-empty = show banner).

### `admin_filters.html`

Table: ID | User Email | Keywords | Location | Min Salary | Max Salary | Title | Created At

---

## Handler Implementation Details

### `internal/web/admin/admin.go`

```go
package admin

import (
    "crypto/subtle"
    "encoding/base64"
    "net/http"
    "strings"

    "github.com/go-chi/chi/v5"
    "github.com/whinchman/jobhuntr/internal/store"
)

type adminHandler struct {
    store    AdminStore
    password string // from cfg.Admin.Password
    tmpl     *template.Template
}

func New(st AdminStore, password string) *adminHandler {
    // parse embedded templates
    ...
}

// Routes returns a chi.Router with all /admin sub-routes attached.
func (h *adminHandler) Routes() chi.Router {
    r := chi.NewRouter()
    r.Use(h.adminAuth)
    r.Get("/", h.handleAdminDashboard)
    r.Get("/users", h.handleAdminUsers)
    r.Post("/users/{id}/ban", h.handleAdminBanUser)
    r.Post("/users/{id}/unban", h.handleAdminUnbanUser)
    r.Post("/users/{id}/reset-password", h.handleAdminResetPassword)
    r.Get("/filters", h.handleAdminFilters)
    return r
}

func (h *adminHandler) adminAuth(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if h.password == "" {
            http.Error(w, "Admin panel not configured", http.StatusUnauthorized)
            return
        }
        user, pass, ok := r.BasicAuth()
        if !ok || subtle.ConstantTimeCompare([]byte(user), []byte("admin")) != 1 ||
            subtle.ConstantTimeCompare([]byte(pass), []byte(h.password)) != 1 {
            w.Header().Set("WWW-Authenticate", `Basic realm="JobHuntr Admin"`)
            http.Error(w, "Unauthorized", http.StatusUnauthorized)
            return
        }
        next.ServeHTTP(w, r)
    })
}
```

### `internal/web/server.go` — wiring

In `Handler()`, after the existing route groups, add:

```go
// Admin panel — mounted at /admin, protected by HTTP Basic Auth.
if s.cfg != nil {
    adminH := admin.New(s.adminStore, s.cfg.Admin.Password)
    r.Mount("/admin", adminH.Routes())
}
```

The `Server` struct needs a new field:
```go
adminStore admin.AdminStore
```

And `NewServerWithConfig` needs an additional `adminStore admin.AdminStore` parameter
**or** the concrete `*store.Store` passed as `st` can be accepted via type assertion
since `*store.Store` implements all required methods.

**Preferred approach**: cast the `st` argument. Since `*store.Store` is the concrete
type always passed in production, add an optional setter:

```go
func (s *Server) WithAdminStore(as AdminStore) *Server {
    s.adminStore = as
    return s
}
```

And in `main.go`:

```go
webSrv := web.NewServerWithConfig(db, db, db, cfg).
    WithAdminStore(db).   // <-- new
    WithLastScrapeFn(sched.LastScrapeAt).
    ...
```

This avoids adding a 5th positional parameter to `NewServerWithConfig`.

### Password generation in `handleAdminResetPassword`

```go
func generateTempPassword() (string, error) {
    const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
    b := make([]byte, 12)
    if _, err := rand.Read(b); err != nil {
        return "", err
    }
    for i, v := range b {
        b[i] = charset[int(v)%len(charset)]
    }
    return string(b), nil
}
```

Hash with `bcrypt.GenerateFromPassword([]byte(tempPwd), 12)`.
Call `store.SetPasswordHash(ctx, userID, string(hash))`.
Re-render `admin_users.html` with `TempPassword` set so operator can copy it.

---

## Step-by-Step Implementation Plan

### Step 1 — Migration and model update

1. Create `internal/store/migrations/010_add_banned_at_to_users.sql`:
   ```sql
   ALTER TABLE users ADD COLUMN IF NOT EXISTS banned_at TIMESTAMPTZ;
   ```
2. In `internal/models/user.go`, add `BannedAt *time.Time` field to `User`.
3. In `internal/store/user.go`:
   - Append `, banned_at` to `userSelectCols`.
   - In `scanUser`: add `var bannedAtStr sql.NullString`, scan it, and parse into `u.BannedAt`.

**Acceptance**: `go test ./internal/store/...` passes (existing tests unaffected because
new column is nullable with no default constraint changes).

### Step 2 — Config change

1. Add `AdminConfig` struct to `internal/config/config.go`.
2. Add `Admin AdminConfig` to `Config`.
3. Update `config.yaml.example` with `admin.password: "${ADMIN_PASSWORD}"`.

**Acceptance**: `go test ./internal/config/...` passes.

### Step 3 — New store methods

Add to `internal/store/user.go`:
- `ListAllUsers(ctx) ([]models.User, error)`
- `BanUser(ctx, userID int64) error`
- `UnbanUser(ctx, userID int64) error`
- `SetPasswordHash(ctx, userID int64, hash string) error`
- `ListAllFilters(ctx) ([]store.AdminFilter, error)` — plus `AdminFilter` type

Add to `internal/store/store.go`:
- `AdminStats` struct
- `GetAdminStats(ctx) (AdminStats, error)` — counts via one CTE or four queries

**Acceptance**: Unit tests in `internal/store/user_test.go` (new table-driven tests
for each method). Stats test can use a fresh DB with known seed data.

### Step 4 — Admin package skeleton + auth middleware

Create `internal/web/admin/` directory.

Files to create:
- `admin.go` — `adminHandler` struct, `New()` constructor, `adminAuth` middleware, `Routes()` method
- `store.go` — `AdminStore` interface

**Acceptance**: Package compiles. `TestAdminAuth` unit test (table-driven: no header,
wrong user, wrong pass, correct creds) using `httptest.NewRecorder`.

### Step 5 — Templates

Create `internal/web/admin/templates/`:
- `admin_layout.html`
- `admin_dashboard.html`
- `admin_users.html`
- `admin_filters.html`

Embed with `//go:embed templates` in `admin.go`.

Parse in `New()`:
```go
tmpl := template.Must(template.New("admin_layout.html").ParseFS(adminTemplateFS,
    "templates/admin_layout.html",
    "templates/admin_dashboard.html",
    "templates/admin_users.html",
    "templates/admin_filters.html",
))
```

**Acceptance**: Templates parse without error (checked in constructor via `Must`).

### Step 6 — Handler implementations

Create `internal/web/admin/handlers.go` with:
- `handleAdminDashboard` — calls `GetAdminStats`, renders dashboard
- `handleAdminUsers` — calls `ListAllUsers`, renders users table
- `handleAdminBanUser` — parses `{id}`, calls `BanUser`, redirects to `/admin/users`
- `handleAdminUnbanUser` — parses `{id}`, calls `UnbanUser`, redirects to `/admin/users`
- `handleAdminResetPassword` — generates temp password, hashes, calls `SetPasswordHash`,
  fetches updated user list, re-renders users template with `TempPassword` set
- `handleAdminFilters` — calls `ListAllFilters`, renders filters table

**Acceptance**: Integration tests (httptest) covering each handler: auth rejected,
dashboard renders stats, ban/unban toggles, reset shows temp password, filters list.

### Step 7 — Wire into Server

1. Add `adminStore admin.AdminStore` field to `Server`.
2. Add `WithAdminStore(as admin.AdminStore) *Server` method.
3. In `Handler()`, conditionally mount admin sub-router.
4. In `cmd/jobhuntr/main.go`, chain `.WithAdminStore(db)`.

**Acceptance**: `go build ./...` succeeds. Integration test: GET `/admin` with no
credentials returns 401; GET `/admin` with correct credentials returns 200.

### Step 8 — End-to-end tests

Add `internal/web/admin/admin_test.go` with tests:
- `TestAdminRequiresAuth` — 401 without credentials
- `TestAdminDashboard` — 200 with correct credentials, stats rendered
- `TestAdminBanUnban` — POST ban → user appears banned; POST unban → unbanned
- `TestAdminResetPassword` — POST reset → temp password shown in response
- `TestAdminFilters` — filter list renders with joined user email

---

## Trade-offs and Alternatives

### Alternative A: Standalone binary / separate service

Run a second `cmd/admin/main.go` binary. Pro: complete isolation. Con: duplicates
the entire DB connection + config loading; adds operational complexity (two processes,
two ports). Rejected — over-engineering for a single-operator tool.

### Alternative B: Admin flag on the users table

Add an `is_admin BOOLEAN` column and reuse the existing session/cookie auth. Pro:
multi-admin capable. Con: the requirement explicitly states "no admin flag on users
table". Rejected.

### Alternative C: Embed admin under existing `internal/web` (same package, no sub-package)

Add admin handlers directly into `internal/web/` files. Pro: less ceremony. Con:
pollutes the user-facing handler namespace, makes testing harder (all handlers share
one `Server` struct), and makes it impossible to enforce that admin routes are never
accessible without the Basic Auth middleware being applied. Rejected.

### Chosen approach: `internal/web/admin` sub-package with its own `adminHandler`

Clean interface boundary, independently testable, no import cycles, admin auth is
locally enforced.

---

## Acceptance Criteria

- [ ] `GET /admin` returns `401 Unauthorized` when no `Authorization` header is provided.
- [ ] `GET /admin` returns `401 Unauthorized` when username or password is incorrect.
- [ ] `GET /admin` returns `200 OK` with admin dashboard HTML when correct credentials are provided.
- [ ] `GET /admin/users` lists all users with email, created_at, and banned status.
- [ ] `POST /admin/users/{id}/ban` sets `banned_at` on the user and redirects back to user list.
- [ ] `POST /admin/users/{id}/unban` clears `banned_at` and redirects back to user list.
- [ ] `POST /admin/users/{id}/reset-password` generates a temp password, updates `password_hash`,
      clears `reset_token`, and displays the plaintext temp password in the response.
- [ ] `GET /admin/filters` lists all filters across all users with the owning user's email.
- [ ] `GET /admin` displays total users, total jobs, total filters, and new users in last 7 days.
- [ ] When `ADMIN_PASSWORD` is empty, every `/admin/*` request returns `401` (no panic).
- [ ] All existing tests continue to pass (`go test ./...`).
- [ ] `go build ./...` succeeds with no new compiler warnings or vet failures.
- [ ] The `admin.password` config key is documented in `config.yaml.example`.
- [ ] The new migration `010_add_banned_at_to_users.sql` applies cleanly against
      an existing database (uses `ALTER TABLE ... ADD COLUMN IF NOT EXISTS`).

---

## Dependencies and Prerequisites

No new external libraries needed. All required packages are already in `go.mod`:
- `github.com/go-chi/chi/v5` — sub-router and URL parameter extraction
- `golang.org/x/crypto/bcrypt` — bcrypt hashing for temp password
- `crypto/subtle` — constant-time comparison for Basic Auth
- `database/sql` — existing store pattern
- `html/template` + `embed` — template rendering, same pattern as existing templates

---

## Notes / Assumptions

1. **No CSRF on admin forms.** HTTP Basic Auth on every request provides equivalent
   CSRF protection for a single-operator panel. If in future a multi-session admin
   model is needed, revisit.
2. **No pagination on user/filter lists.** The panel is for a single-operator SaaS
   and user counts are expected to be in the hundreds, not millions.
3. **Banned users ARE blocked from logging in.** Enforcement is in scope:
   - `handleLoginPost` must check `banned_at IS NOT NULL` after credential verification and return a "Your account has been suspended" flash error.
   - `handleOAuthCallback` must check `banned_at IS NOT NULL` after the user upsert and return the same error.
   - The `requireAuth` middleware should also check the session user's `banned_at` and redirect to `/login` with a flash if set (handles sessions that were active at ban time).
4. **Temp password is shown once.** It is not stored anywhere in plaintext. The
   operator must note it before navigating away.
5. **No audit log.** Out of scope for this iteration.
