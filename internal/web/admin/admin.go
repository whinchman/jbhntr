// Package admin provides the HTTP handlers and middleware for the admin panel.
package admin

import (
	"crypto/subtle"
	"embed"
	"html/template"
	"net/http"

	"github.com/go-chi/chi/v5"
)

//go:embed templates
var adminTemplateFS embed.FS

type adminHandler struct {
	store       AdminStore
	password    string
	dashboardTmpl *template.Template
	usersTmpl     *template.Template
	filtersTmpl   *template.Template
}

// mustParsePage parses admin_layout.html together with a single page template
// so that each page's "content" block is defined exactly once per template set.
func mustParsePage(page string) *template.Template {
	return template.Must(template.New("admin_layout.html").ParseFS(adminTemplateFS,
		"templates/admin_layout.html",
		"templates/"+page,
	))
}

// New constructs an adminHandler with the given store and admin password.
// Templates are parsed eagerly via template.Must; a malformed template
// will panic at startup rather than at request time.
func New(st AdminStore, password string) *adminHandler {
	return &adminHandler{
		store:         st,
		password:      password,
		dashboardTmpl: mustParsePage("admin_dashboard.html"),
		usersTmpl:     mustParsePage("admin_users.html"),
		filtersTmpl:   mustParsePage("admin_filters.html"),
	}
}

// Routes returns a chi.Router with all admin routes mounted under the
// adminAuth Basic Auth middleware.
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

// adminAuth is a chi middleware that enforces HTTP Basic Auth.
// If the configured password is empty, every request is rejected immediately.
// Both the username and password are compared with subtle.ConstantTimeCompare
// to prevent timing-based credential enumeration.
func (h *adminHandler) adminAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h.password == "" {
			w.Header().Set("WWW-Authenticate", `Basic realm="JobHuntr Admin"`)
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
