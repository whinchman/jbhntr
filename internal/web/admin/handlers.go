package admin

import (
	"crypto/rand"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/whinchman/jobhuntr/internal/models"
	"github.com/whinchman/jobhuntr/internal/store"
)

// adminDashboardData is the template data for the admin dashboard page.
type adminDashboardData struct {
	Stats store.AdminStats
}

// adminUsersData is the template data for the admin users page.
type adminUsersData struct {
	Users        []models.User
	TempPassword string
	ResetUserID  int64
}

// adminFiltersData is the template data for the admin filters page.
type adminFiltersData struct {
	Filters []store.AdminFilter
}

// handleAdminDashboard renders the admin dashboard with site-wide stats.
func (h *adminHandler) handleAdminDashboard(w http.ResponseWriter, r *http.Request) {
	stats, err := h.store.GetAdminStats(r.Context())
	if err != nil {
		http.Error(w, "failed to load stats", http.StatusInternalServerError)
		return
	}
	data := adminDashboardData{Stats: stats}
	if err := h.dashboardTmpl.ExecuteTemplate(w, "admin_layout.html", data); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

// handleAdminUsers renders the admin users list.
func (h *adminHandler) handleAdminUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.store.ListAllUsers(r.Context())
	if err != nil {
		http.Error(w, "failed to load users", http.StatusInternalServerError)
		return
	}
	data := adminUsersData{Users: users}
	if err := h.usersTmpl.ExecuteTemplate(w, "admin_layout.html", data); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

// handleAdminBanUser bans a user by ID and redirects to /admin/users.
func (h *adminHandler) handleAdminBanUser(w http.ResponseWriter, r *http.Request) {
	id, err := parseUserID(r)
	if err != nil {
		http.Error(w, "invalid user id", http.StatusBadRequest)
		return
	}
	if err := h.store.BanUser(r.Context(), id); err != nil {
		http.Error(w, "failed to ban user", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
}

// handleAdminUnbanUser unbans a user by ID and redirects to /admin/users.
func (h *adminHandler) handleAdminUnbanUser(w http.ResponseWriter, r *http.Request) {
	id, err := parseUserID(r)
	if err != nil {
		http.Error(w, "invalid user id", http.StatusBadRequest)
		return
	}
	if err := h.store.UnbanUser(r.Context(), id); err != nil {
		http.Error(w, "failed to unban user", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
}

// handleAdminResetPassword generates a random 12-character alphanumeric
// temp password, hashes it with bcrypt cost 12, stores it via SetPasswordHash,
// then re-renders the users page with the plaintext temp password visible
// so the operator can copy it.
func (h *adminHandler) handleAdminResetPassword(w http.ResponseWriter, r *http.Request) {
	id, err := parseUserID(r)
	if err != nil {
		http.Error(w, "invalid user id", http.StatusBadRequest)
		return
	}

	tmp, err := generateTempPassword()
	if err != nil {
		http.Error(w, "failed to generate password", http.StatusInternalServerError)
		return
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(tmp), 12)
	if err != nil {
		http.Error(w, "failed to hash password", http.StatusInternalServerError)
		return
	}

	if err := h.store.SetPasswordHash(r.Context(), id, string(hashed)); err != nil {
		http.Error(w, "failed to reset password", http.StatusInternalServerError)
		return
	}

	// Re-render users page with the temp password displayed.
	users, err := h.store.ListAllUsers(r.Context())
	if err != nil {
		http.Error(w, "failed to load users", http.StatusInternalServerError)
		return
	}
	data := adminUsersData{Users: users, TempPassword: tmp, ResetUserID: id}
	if err := h.usersTmpl.ExecuteTemplate(w, "admin_layout.html", data); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

// handleAdminFilters renders the admin filters list.
func (h *adminHandler) handleAdminFilters(w http.ResponseWriter, r *http.Request) {
	filters, err := h.store.ListAllFilters(r.Context())
	if err != nil {
		http.Error(w, "failed to load filters", http.StatusInternalServerError)
		return
	}
	data := adminFiltersData{Filters: filters}
	if err := h.filtersTmpl.ExecuteTemplate(w, "admin_layout.html", data); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

// parseUserID extracts the {id} URL parameter and converts it to int64.
func parseUserID(r *http.Request) (int64, error) {
	raw := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid id %q: %w", raw, err)
	}
	return id, nil
}

// generateTempPassword returns a 12-character alphanumeric string generated
// from crypto/rand. Each byte is mapped to the charset using modular
// reduction — acceptable for a temporary password where bias is negligible.
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
