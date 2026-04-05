package admin_test

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode"

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

// ─── recordingAdminStore tracks calls for verification ───────────────────────

type recordingAdminStore struct {
	mu sync.Mutex

	users   []models.User
	filters []store.AdminFilter
	stats   store.AdminStats

	// recorded calls
	bannedIDs         []int64
	unbannedIDs       []int64
	passwordResets    []passwordResetCall
}

type passwordResetCall struct {
	userID int64
	hash   string
}

func newRecordingStore(users []models.User, filters []store.AdminFilter, stats store.AdminStats) *recordingAdminStore {
	return &recordingAdminStore{users: users, filters: filters, stats: stats}
}

func (s *recordingAdminStore) ListAllUsers(_ context.Context) ([]models.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]models.User(nil), s.users...), nil
}

func (s *recordingAdminStore) BanUser(_ context.Context, id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.bannedIDs = append(s.bannedIDs, id)
	return nil
}

func (s *recordingAdminStore) UnbanUser(_ context.Context, id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.unbannedIDs = append(s.unbannedIDs, id)
	return nil
}

func (s *recordingAdminStore) SetPasswordHash(_ context.Context, id int64, hash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.passwordResets = append(s.passwordResets, passwordResetCall{userID: id, hash: hash})
	return nil
}

func (s *recordingAdminStore) ListAllFilters(_ context.Context) ([]store.AdminFilter, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]store.AdminFilter(nil), s.filters...), nil
}

func (s *recordingAdminStore) GetAdminStats(_ context.Context) (store.AdminStats, error) {
	return s.stats, nil
}

func (s *recordingAdminStore) lastBannedID() (int64, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.bannedIDs) == 0 {
		return 0, false
	}
	return s.bannedIDs[len(s.bannedIDs)-1], true
}

func (s *recordingAdminStore) lastUnbannedID() (int64, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.unbannedIDs) == 0 {
		return 0, false
	}
	return s.unbannedIDs[len(s.unbannedIDs)-1], true
}

func (s *recordingAdminStore) lastPasswordReset() (passwordResetCall, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.passwordResets) == 0 {
		return passwordResetCall{}, false
	}
	return s.passwordResets[len(s.passwordResets)-1], true
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

// ─── new tests covering store-call verification and HTML content ──────────────

// TestAdminDashboardContainsStats verifies that GET /admin renders stat values
// from the store into the HTML response body.
func TestAdminDashboardContainsStats(t *testing.T) {
	stats := store.AdminStats{TotalUsers: 42, TotalJobs: 99, TotalFilters: 7, NewUsersLast7d: 3}
	rs := newRecordingStore(nil, nil, stats)
	handler := admin.New(rs, "pass")
	srv := httptest.NewServer(handler.Routes())
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/", nil)
	req.Header.Set("Authorization", basicAuth("admin", "pass"))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("dashboard request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("dashboard: got %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)
	for _, want := range []string{"42", "99", "7", "3"} {
		if !strings.Contains(bodyStr, want) {
			t.Errorf("dashboard body missing stat value %q", want)
		}
	}
}

// TestAdminUsersListContainsUser verifies that GET /admin/users lists seeded
// users by rendering their email addresses in the HTML table.
func TestAdminUsersListContainsUser(t *testing.T) {
	users := []models.User{
		{ID: 1, Email: "alice@example.com", DisplayName: "Alice", CreatedAt: time.Now()},
		{ID: 2, Email: "bob@example.com", DisplayName: "Bob", CreatedAt: time.Now()},
	}
	rs := newRecordingStore(users, nil, store.AdminStats{})
	handler := admin.New(rs, "pass")
	srv := httptest.NewServer(handler.Routes())
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/users", nil)
	req.Header.Set("Authorization", basicAuth("admin", "pass"))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("users request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("users: got %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)
	for _, email := range []string{"alice@example.com", "bob@example.com"} {
		if !strings.Contains(bodyStr, email) {
			t.Errorf("users body missing email %q", email)
		}
	}
}

// TestAdminBanCallsStore verifies that POST /admin/users/{id}/ban calls
// the store's BanUser method with the correct user ID.
func TestAdminBanCallsStore(t *testing.T) {
	rs := newRecordingStore(nil, nil, store.AdminStats{})
	handler := admin.New(rs, "pass")
	client := &http.Client{CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	srv := httptest.NewServer(handler.Routes())
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/users/7/ban", nil)
	req.Header.Set("Authorization", basicAuth("admin", "pass"))
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("ban request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("ban: got %d, want 303", resp.StatusCode)
	}
	id, ok := rs.lastBannedID()
	if !ok {
		t.Fatal("BanUser was not called on the store")
	}
	if id != 7 {
		t.Errorf("BanUser called with id=%d, want 7", id)
	}
}

// TestAdminUnbanCallsStore verifies that POST /admin/users/{id}/unban calls
// the store's UnbanUser method with the correct user ID.
func TestAdminUnbanCallsStore(t *testing.T) {
	rs := newRecordingStore(nil, nil, store.AdminStats{})
	handler := admin.New(rs, "pass")
	client := &http.Client{CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	srv := httptest.NewServer(handler.Routes())
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/users/9/unban", nil)
	req.Header.Set("Authorization", basicAuth("admin", "pass"))
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("unban request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("unban: got %d, want 303", resp.StatusCode)
	}
	id, ok := rs.lastUnbannedID()
	if !ok {
		t.Fatal("UnbanUser was not called on the store")
	}
	if id != 9 {
		t.Errorf("UnbanUser called with id=%d, want 9", id)
	}
}

// TestAdminResetPasswordCallsStoreAndShowsTempPassword verifies that
// POST /admin/users/{id}/reset-password calls SetPasswordHash and renders
// the 12-character alphanumeric temp password in the HTML response body.
func TestAdminResetPasswordCallsStoreAndShowsTempPassword(t *testing.T) {
	rs := newRecordingStore(nil, nil, store.AdminStats{})
	handler := admin.New(rs, "pass")
	srv := httptest.NewServer(handler.Routes())
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/users/5/reset-password", srv.URL), nil)
	req.Header.Set("Authorization", basicAuth("admin", "pass"))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("reset-password request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("reset-password: got %d, want 200", resp.StatusCode)
	}

	// Verify SetPasswordHash was called for the correct user.
	call, ok := rs.lastPasswordReset()
	if !ok {
		t.Fatal("SetPasswordHash was not called on the store")
	}
	if call.userID != 5 {
		t.Errorf("SetPasswordHash called for user %d, want 5", call.userID)
	}
	if call.hash == "" {
		t.Error("SetPasswordHash called with empty hash")
	}

	// The rendered page should contain the temp password (12-char alphanumeric).
	// The handler writes the plaintext temp password into the template, so it
	// appears verbatim in the HTML <code> block.
	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)

	// Extract the temp password from the response by looking for a 12-character
	// alphanumeric run that appears between <code> tags.
	codeIdx := strings.Index(bodyStr, "<code>")
	if codeIdx < 0 {
		t.Fatal("response body missing <code> tag for temp password")
	}
	start := codeIdx + len("<code>")
	end := strings.Index(bodyStr[start:], "</code>")
	if end < 0 {
		t.Fatal("response body missing </code> closing tag")
	}
	tmp := bodyStr[start : start+end]
	if len(tmp) != 12 {
		t.Errorf("temp password length = %d, want 12; got %q", len(tmp), tmp)
	}
	for _, ch := range tmp {
		if !unicode.IsLetter(ch) && !unicode.IsDigit(ch) {
			t.Errorf("temp password %q contains non-alphanumeric character %q", tmp, ch)
		}
	}
}

// TestAdminFiltersContainsFilter verifies that GET /admin/filters lists
// seeded filters by rendering their user emails in the HTML table.
func TestAdminFiltersContainsFilter(t *testing.T) {
	filters := []store.AdminFilter{
		{
			UserSearchFilter: models.UserSearchFilter{
				ID:       1,
				UserID:   1,
				Keywords: "golang",
				Location: "Remote",
				Title:    "Engineer",
				CreatedAt: time.Now(),
			},
			UserEmail: "filter-owner@example.com",
		},
	}
	rs := newRecordingStore(nil, filters, store.AdminStats{})
	handler := admin.New(rs, "pass")
	srv := httptest.NewServer(handler.Routes())
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/filters", nil)
	req.Header.Set("Authorization", basicAuth("admin", "pass"))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("filters request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("filters: got %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)
	if !strings.Contains(bodyStr, "filter-owner@example.com") {
		t.Error("filters body missing user email 'filter-owner@example.com'")
	}
	if !strings.Contains(bodyStr, "golang") {
		t.Error("filters body missing keyword 'golang'")
	}
}
