# Task: admin-panel-3-admin-package

- **Type**: coder
- **Status**: pending
- **Branch**: feature/admin-panel-3-admin-package
- **Source Item**: Admin Panel for jobhuntr (admin-panel.md)
- **Parallel Group**: 3
- **Dependencies**: admin-panel-2-store-methods

## Description

Build the complete `internal/web/admin` sub-package and wire it into the existing server. This is the main implementation task for the admin panel. It covers:

1. Creating the `internal/web/admin/` package directory with:
   - `admin.go` — `adminHandler` struct, `New()` constructor, `adminAuth` Basic Auth middleware, `Routes()` method
   - `store.go` — `AdminStore` interface definition
   - `handlers.go` — all six HTTP handler methods plus template data types plus `generateTempPassword()`
   - `templates/admin_layout.html` — minimal HTML shell with PicoCSS CDN and admin nav
   - `templates/admin_dashboard.html` — stats cards
   - `templates/admin_users.html` — user table with ban/unban/reset actions and temp-password flash
   - `templates/admin_filters.html` — filter table with joined user email
2. Wiring the admin router into `internal/web/server.go` under `/admin`.
3. Adding `.WithAdminStore(db)` call to `cmd/jobhuntr/main.go`.

## Acceptance Criteria

- [ ] `internal/web/admin/store.go` defines `AdminStore` interface with all six methods (signatures matching the Interface Contracts section)
- [ ] `internal/web/admin/admin.go` defines `adminHandler`, `New()`, `adminAuth` middleware, and `Routes()`
- [ ] `adminAuth` uses `subtle.ConstantTimeCompare` for both username and password; returns 401 with `WWW-Authenticate` header on failure
- [ ] When `password` field is empty, `adminAuth` returns 401 on every request (no panic)
- [ ] `internal/web/admin/handlers.go` implements all six handlers: `handleAdminDashboard`, `handleAdminUsers`, `handleAdminBanUser`, `handleAdminUnbanUser`, `handleAdminResetPassword`, `handleAdminFilters`
- [ ] `handleAdminResetPassword` generates a 12-character alphanumeric temp password via `crypto/rand`, hashes it with bcrypt cost 12, calls `SetPasswordHash`, then re-renders the users page with `TempPassword` set
- [ ] Ban and unban handlers parse `{id}` URL parameter, call the appropriate store method, and redirect to `/admin/users`
- [ ] All four HTML templates exist under `internal/web/admin/templates/` and are embedded with `//go:embed templates`
- [ ] Templates parse without error (verified via `template.Must` in constructor)
- [ ] `internal/web/server.go` mounts the admin sub-router at `/admin` via `r.Mount`
- [ ] `Server` struct has an `adminStore admin.AdminStore` field and a `WithAdminStore(as admin.AdminStore) *Server` method
- [ ] `cmd/jobhuntr/main.go` chains `.WithAdminStore(db)` when building the server
- [ ] Unit test `TestAdminAuth` (table-driven: no header / wrong user / wrong pass / correct creds) passes using `httptest.NewRecorder`
- [ ] `go test ./internal/web/admin/...` passes
- [ ] `go build ./...` succeeds

## Interface Contracts

**Consumed** (from admin-panel-2-store-methods):

The `AdminStore` interface defined in this package must match exactly the method signatures that `*store.Store` implements:

```go
// AdminStore is the subset of store.Store used by the admin panel.
// Defined in internal/web/admin/store.go.
type AdminStore interface {
    ListAllUsers(ctx context.Context) ([]models.User, error)
    BanUser(ctx context.Context, userID int64) error
    UnbanUser(ctx context.Context, userID int64) error
    SetPasswordHash(ctx context.Context, userID int64, hash string) error
    ListAllFilters(ctx context.Context) ([]store.AdminFilter, error)
    GetAdminStats(ctx context.Context) (store.AdminStats, error)
}
```

Types consumed from store package:
- `store.AdminFilter` — has embedded `models.UserSearchFilter` plus `UserEmail string`
- `store.AdminStats` — has `TotalUsers`, `TotalJobs`, `TotalFilters`, `NewUsersLast7d int`
- `models.User.BannedAt *time.Time` — nil = not banned; non-nil = banned (use in templates to show banned state)

**Produced** (used by server wiring and QA task):

Routes registered under `/admin`:
```
GET  /admin                          → handleAdminDashboard
GET  /admin/users                    → handleAdminUsers
POST /admin/users/{id}/ban           → handleAdminBanUser
POST /admin/users/{id}/unban         → handleAdminUnbanUser
POST /admin/users/{id}/reset-password → handleAdminResetPassword
GET  /admin/filters                  → handleAdminFilters
```

All routes protected by `adminAuth` middleware (HTTP Basic Auth, username `"admin"`, password from `cfg.Admin.Password`).

## Context

### `internal/web/admin/admin.go` skeleton

```go
package admin

import (
    "crypto/subtle"
    "embed"
    "html/template"
    "net/http"

    "github.com/go-chi/chi/v5"
    "github.com/whinchman/jobhuntr/internal/store"
)

//go:embed templates
var adminTemplateFS embed.FS

type adminHandler struct {
    store    AdminStore
    password string
    tmpl     *template.Template
}

func New(st AdminStore, password string) *adminHandler {
    tmpl := template.Must(template.New("admin_layout.html").ParseFS(adminTemplateFS,
        "templates/admin_layout.html",
        "templates/admin_dashboard.html",
        "templates/admin_users.html",
        "templates/admin_filters.html",
    ))
    return &adminHandler{store: st, password: password, tmpl: tmpl}
}

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
        if !ok ||
            subtle.ConstantTimeCompare([]byte(user), []byte("admin")) != 1 ||
            subtle.ConstantTimeCompare([]byte(pass), []byte(h.password)) != 1 {
            w.Header().Set("WWW-Authenticate", `Basic realm="JobHuntr Admin"`)
            http.Error(w, "Unauthorized", http.StatusUnauthorized)
            return
        }
        next.ServeHTTP(w, r)
    })
}
```

### `internal/web/server.go` wiring

Add field to `Server` struct:
```go
adminStore admin.AdminStore
```

Add method:
```go
func (s *Server) WithAdminStore(as admin.AdminStore) *Server {
    s.adminStore = as
    return s
}
```

In `Handler()`, after existing route groups:
```go
// Admin panel — mounted at /admin, protected by HTTP Basic Auth.
if s.cfg != nil && s.adminStore != nil {
    adminH := admin.New(s.adminStore, s.cfg.Admin.Password)
    r.Mount("/admin", adminH.Routes())
}
```

### `cmd/jobhuntr/main.go` wiring

Chain `.WithAdminStore(db)` on the server builder:
```go
webSrv := web.NewServerWithConfig(db, db, db, cfg).
    WithAdminStore(db).
    WithLastScrapeFn(sched.LastScrapeAt).
    ...
```

### Password generation helper

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

### Template data types (in `handlers.go`)

```go
type adminDashboardData struct {
    Stats store.AdminStats
}

type adminUsersData struct {
    Users        []models.User
    TempPassword string
    ResetUserID  int64
}

type adminFiltersData struct {
    Filters []store.AdminFilter
}
```

### Template structure

`admin_layout.html` — minimal HTML shell using PicoCSS from CDN (same CDN as existing `layout.html`). Provides `{{block "title" .}}` and `{{block "content" .}}` blocks. Admin nav links: Dashboard (`/admin`), Users (`/admin/users`), Filters (`/admin/filters`).

`admin_dashboard.html` — defines `{{block "content" .}}` showing four stat cards.

`admin_users.html` — defines `{{block "content" .}}` with a table. For each user: show ID, Email, DisplayName, CreatedAt, whether banned (`BannedAt != nil`). Actions: ban form or unban form depending on state; always show reset-password form. If `TempPassword` is non-empty, show a flash/alert block at the top with the generated password so the operator can copy it.

`admin_filters.html` — defines `{{block "content" .}}` with a table: ID, UserEmail, Keywords, Location, MinSalary, MaxSalary, Title, CreatedAt.

### Existing codebase references

- Look at `internal/web/templates/` for PicoCSS CDN URL and layout pattern to replicate in admin templates.
- Look at `internal/web/server.go` for how existing route groups are mounted and how `NewServerWithConfig` is defined — add `WithAdminStore` as a chainable setter (do NOT add a 5th positional parameter).
- Look at `cmd/jobhuntr/main.go` for the existing server builder chain to find where to insert `.WithAdminStore(db)`.

## Notes

