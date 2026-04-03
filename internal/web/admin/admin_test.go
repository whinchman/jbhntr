package admin_test

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/whinchman/jobhuntr/internal/models"
	"github.com/whinchman/jobhuntr/internal/store"
	"github.com/whinchman/jobhuntr/internal/web/admin"
)

// ─── mock AdminStore ──────────────────────────────────────────────────────────

type mockAdminStore struct{}

func (m *mockAdminStore) ListAllUsers(_ context.Context) ([]models.User, error) {
	return []models.User{}, nil
}
func (m *mockAdminStore) BanUser(_ context.Context, _ int64) error   { return nil }
func (m *mockAdminStore) UnbanUser(_ context.Context, _ int64) error { return nil }
func (m *mockAdminStore) SetPasswordHash(_ context.Context, _ int64, _ string) error {
	return nil
}
func (m *mockAdminStore) ListAllFilters(_ context.Context) ([]store.AdminFilter, error) {
	return []store.AdminFilter{}, nil
}
func (m *mockAdminStore) GetAdminStats(_ context.Context) (store.AdminStats, error) {
	return store.AdminStats{TotalUsers: 1, TotalJobs: 2, TotalFilters: 3, NewUsersLast7d: 0}, nil
}

// basicAuth returns an HTTP Basic Auth header value.
func basicAuth(user, pass string) string {
	creds := base64.StdEncoding.EncodeToString([]byte(user + ":" + pass))
	return "Basic " + creds
}

// TestAdminAuth verifies that the adminAuth middleware correctly enforces
// HTTP Basic Auth credentials.
func TestAdminAuth(t *testing.T) {
	const correctPassword = "s3cr3t"

	handler := admin.New(&mockAdminStore{}, correctPassword)
	srv := httptest.NewServer(handler.Routes())
	defer srv.Close()

	tests := []struct {
		name       string
		authHeader string
		wantStatus int
	}{
		{
			name:       "no auth header",
			authHeader: "",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "wrong username",
			authHeader: basicAuth("notadmin", correctPassword),
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "wrong password",
			authHeader: basicAuth("admin", "wrongpass"),
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "correct credentials",
			authHeader: basicAuth("admin", correctPassword),
			wantStatus: http.StatusOK,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, srv.URL+"/", nil)
			if err != nil {
				t.Fatalf("failed to build request: %v", err)
			}
			if tc.authHeader != "" {
				req.Header.Set("Authorization", tc.authHeader)
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != tc.wantStatus {
				t.Errorf("got status %d, want %d", resp.StatusCode, tc.wantStatus)
			}
			if tc.wantStatus == http.StatusUnauthorized {
				wwwAuth := resp.Header.Get("WWW-Authenticate")
				if wwwAuth == "" {
					t.Error("expected WWW-Authenticate header on 401 response, got none")
				}
			}
		})
	}
}

// TestAdminAuthEmptyPassword verifies that an empty password always returns 401.
func TestAdminAuthEmptyPassword(t *testing.T) {
	handler := admin.New(&mockAdminStore{}, "")
	srv := httptest.NewServer(handler.Routes())
	defer srv.Close()

	// Even with "correct" admin/empty credentials, empty password must reject.
	req, err := http.NewRequest(http.MethodGet, srv.URL+"/", nil)
	if err != nil {
		t.Fatalf("failed to build request: %v", err)
	}
	req.Header.Set("Authorization", basicAuth("admin", ""))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("got status %d, want %d (empty password should always reject)", resp.StatusCode, http.StatusUnauthorized)
	}
}

// TestAdminDashboardRendersOK verifies that GET /admin with valid auth returns 200.
func TestAdminDashboardRendersOK(t *testing.T) {
	handler := admin.New(&mockAdminStore{}, "pass")
	srv := httptest.NewServer(handler.Routes())
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/", nil)
	req.Header.Set("Authorization", basicAuth("admin", "pass"))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("dashboard: got status %d, want 200", resp.StatusCode)
	}
}

// TestAdminUsersRendersOK verifies that GET /admin/users with valid auth returns 200.
func TestAdminUsersRendersOK(t *testing.T) {
	handler := admin.New(&mockAdminStore{}, "pass")
	srv := httptest.NewServer(handler.Routes())
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/users", nil)
	req.Header.Set("Authorization", basicAuth("admin", "pass"))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("users: got status %d, want 200", resp.StatusCode)
	}
}

// TestAdminFiltersRendersOK verifies that GET /admin/filters with valid auth returns 200.
func TestAdminFiltersRendersOK(t *testing.T) {
	handler := admin.New(&mockAdminStore{}, "pass")
	srv := httptest.NewServer(handler.Routes())
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/filters", nil)
	req.Header.Set("Authorization", basicAuth("admin", "pass"))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("filters: got status %d, want 200", resp.StatusCode)
	}
}

// TestAdminBanUserRedirects verifies that POST /admin/users/{id}/ban redirects.
func TestAdminBanUserRedirects(t *testing.T) {
	handler := admin.New(&mockAdminStore{}, "pass")
	// Use a non-redirect-following client.
	client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	srv := httptest.NewServer(handler.Routes())
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/users/1/ban", nil)
	req.Header.Set("Authorization", basicAuth("admin", "pass"))
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("ban: got status %d, want 303", resp.StatusCode)
	}
	loc := resp.Header.Get("Location")
	if loc != "/admin/users" {
		t.Errorf("ban: redirect location %q, want /admin/users", loc)
	}
}

// TestAdminUnbanUserRedirects verifies that POST /admin/users/{id}/unban redirects.
func TestAdminUnbanUserRedirects(t *testing.T) {
	handler := admin.New(&mockAdminStore{}, "pass")
	client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	srv := httptest.NewServer(handler.Routes())
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/users/2/unban", nil)
	req.Header.Set("Authorization", basicAuth("admin", "pass"))
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("unban: got status %d, want 303", resp.StatusCode)
	}
}

// TestAdminResetPasswordRendersPage verifies that POST /admin/users/{id}/reset-password
// re-renders the users page (200) rather than redirecting.
func TestAdminResetPasswordRendersPage(t *testing.T) {
	handler := admin.New(&mockAdminStore{}, "pass")
	srv := httptest.NewServer(handler.Routes())
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/users/1/reset-password", srv.URL), nil)
	req.Header.Set("Authorization", basicAuth("admin", "pass"))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("reset-password: got status %d, want 200", resp.StatusCode)
	}
}

// TestAdminInvalidUserID verifies that a non-numeric {id} returns 400.
func TestAdminInvalidUserID(t *testing.T) {
	handler := admin.New(&mockAdminStore{}, "pass")
	srv := httptest.NewServer(handler.Routes())
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/users/notanumber/ban", nil)
	req.Header.Set("Authorization", basicAuth("admin", "pass"))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("invalid id: got status %d, want 400", resp.StatusCode)
	}
}
