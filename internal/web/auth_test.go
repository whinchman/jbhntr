package web_test

import (
	"context"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/sessions"

	"github.com/whinchman/jobhuntr/internal/config"
	"github.com/whinchman/jobhuntr/internal/models"
	"github.com/whinchman/jobhuntr/internal/store"
	"github.com/whinchman/jobhuntr/internal/web"
)

// ─── mock UserStore ─────────────────────────────────────────────────────────

type mockUserStore struct {
	users map[int64]*models.User
}

func newMockUserStore(users ...*models.User) *mockUserStore {
	m := &mockUserStore{users: make(map[int64]*models.User)}
	for _, u := range users {
		m.users[u.ID] = u
	}
	return m
}

func (m *mockUserStore) GetUser(_ context.Context, id int64) (*models.User, error) {
	u, ok := m.users[id]
	if !ok {
		return nil, fmt.Errorf("user %d not found", id)
	}
	return u, nil
}

func (m *mockUserStore) UpsertUser(_ context.Context, user *models.User) (*models.User, error) {
	if user.ID == 0 {
		user.ID = int64(len(m.users) + 1)
	}
	m.users[user.ID] = user
	return user, nil
}

func (m *mockUserStore) UpdateUserOnboarding(_ context.Context, userID int64, displayName string, resume string) error {
	u, ok := m.users[userID]
	if !ok {
		return fmt.Errorf("user %d not found", userID)
	}
	u.DisplayName = displayName
	u.ResumeMarkdown = resume
	u.OnboardingComplete = true
	return nil
}

func (m *mockUserStore) UpdateUserDisplayName(_ context.Context, userID int64, displayName string) error {
	u, ok := m.users[userID]
	if !ok {
		return fmt.Errorf("user %d not found", userID)
	}
	u.DisplayName = displayName
	return nil
}

func (m *mockUserStore) CreateUserWithPassword(_ context.Context, email, displayName, passwordHash, verifyToken string, verifyExpiresAt time.Time) (*models.User, error) {
	// Check for duplicate email.
	for _, existing := range m.users {
		if existing.Email == email {
			return nil, store.ErrEmailTaken
		}
	}
	u := &models.User{
		ID:          int64(len(m.users) + 1),
		Email:       email,
		DisplayName: displayName,
		Provider:    "email",
	}
	m.users[u.ID] = u
	return u, nil
}

func (m *mockUserStore) GetUserByEmail(_ context.Context, email string) (*models.User, error) {
	for _, u := range m.users {
		if u.Email == email {
			return u, nil
		}
	}
	// Return nil, nil (not an error) to match the real store contract and allow
	// handlers that check `user == nil` to work correctly.
	return nil, nil
}

func (m *mockUserStore) SetResetToken(_ context.Context, userID int64, token string, expiresAt time.Time) error {
	if _, ok := m.users[userID]; !ok {
		return fmt.Errorf("user %d not found", userID)
	}
	return nil
}

func (m *mockUserStore) ConsumeResetToken(_ context.Context, token string, newPasswordHash string) (*models.User, error) {
	for _, u := range m.users {
		if u.ResetToken != nil && *u.ResetToken == token {
			u.ResetToken = nil
			u.ResetExpiresAt = nil
			return u, nil
		}
	}
	return nil, nil
}

func (m *mockUserStore) SetEmailVerifyToken(_ context.Context, userID int64, token string, expiresAt time.Time) error {
	if _, ok := m.users[userID]; !ok {
		return fmt.Errorf("user %d not found", userID)
	}
	return nil
}

func (m *mockUserStore) ConsumeVerifyToken(_ context.Context, token string) (*models.User, error) {
	for _, u := range m.users {
		if u.EmailVerifyToken != nil && *u.EmailVerifyToken == token {
			u.EmailVerifyToken = nil
			u.EmailVerifyExpiresAt = nil
			u.EmailVerified = true
			return u, nil
		}
	}
	return nil, nil
}

func (m *mockUserStore) GetUserByResetToken(_ context.Context, token string) (*models.User, error) {
	for _, u := range m.users {
		if u.ResetToken != nil && *u.ResetToken == token {
			return u, nil
		}
	}
	return nil, nil
}

// ─── helpers ────────────────────────────────────────────────────────────────

func newAuthConfig() *config.Config {
	return &config.Config{
		Server: config.ServerConfig{
			Port:    8080,
			BaseURL: "http://localhost:8080",
		},
		Auth: config.AuthConfig{
			SessionSecret: "test-secret-that-is-at-least-32-bytes",
			OAuth: config.OAuthConfig{
				Enabled: true,
			},
			Providers: config.ProvidersConfig{
				Google: config.OAuthProviderConfig{
					ClientID:     "google-client-id",
					ClientSecret: "google-client-secret",
				},
				GitHub: config.OAuthProviderConfig{
					ClientID:     "github-client-id",
					ClientSecret: "github-client-secret",
				},
			},
		},
	}
}

func newAuthServer(t *testing.T, us *mockUserStore) *httptest.Server {
	t.Helper()
	cfg := newAuthConfig()
	ms := newMockJobStore()
	srv := web.NewServerWithConfig(ms, us, nil, cfg)
	return httptest.NewServer(srv.Handler())
}

// setSessionCookie creates a valid session cookie on the test server by
// directly using gorilla/sessions to encode a cookie with the user_id.
func setSessionCookie(t *testing.T, ts *httptest.Server, userID int64) *http.Cookie {
	t.Helper()
	cfg := newAuthConfig()
	store := sessions.NewCookieStore([]byte(cfg.Auth.SessionSecret))
	// Create a fake request/response to encode the session.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	sess, err := store.New(req, "jobhuntr_session")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	sess.Values["user_id"] = userID
	sess.Options.Path = "/"
	if err := sess.Save(req, w); err != nil {
		t.Fatalf("save session: %v", err)
	}
	cookies := w.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("no session cookie set")
	}
	return cookies[0]
}

// ─── tests ──────────────────────────────────────────────────────────────────

func TestRequireAuth_Unauthenticated(t *testing.T) {
	t.Run("unauthenticated request to protected route redirects to /login", func(t *testing.T) {
		us := newMockUserStore()
		ts := newAuthServer(t, us)
		defer ts.Close()

		client := ts.Client()
		client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		}

		// /settings is a requireAuth route; / is optionalAuth and intentionally
		// returns 200 to unauthenticated visitors.
		resp, err := client.Get(ts.URL + "/settings")
		if err != nil {
			t.Fatalf("GET /settings: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusSeeOther {
			t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusSeeOther)
		}
		loc := resp.Header.Get("Location")
		if loc != "/login" {
			t.Errorf("Location = %q, want /login", loc)
		}
	})
}

func TestRequireAuth_ValidSession(t *testing.T) {
	t.Run("authenticated request passes through", func(t *testing.T) {
		testUser := &models.User{
			ID:          42,
			Provider:    "google",
			ProviderID:  "g-123",
			Email:       "test@example.com",
			DisplayName: "Test User",
		}
		us := newMockUserStore(testUser)
		ts := newAuthServer(t, us)
		defer ts.Close()

		cookie := setSessionCookie(t, ts, 42)

		client := ts.Client()
		client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		}

		req, err := http.NewRequest(http.MethodGet, ts.URL+"/", nil)
		if err != nil {
			t.Fatalf("new request: %v", err)
		}
		req.AddCookie(cookie)

		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("GET /: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("status = %d, want 200", resp.StatusCode)
		}
	})
}

func TestHandleLogin_NotAuthenticated(t *testing.T) {
	t.Run("unauthenticated GET /login returns 200 HTML", func(t *testing.T) {
		us := newMockUserStore()
		ts := newAuthServer(t, us)
		defer ts.Close()

		resp, err := ts.Client().Get(ts.URL + "/login")
		if err != nil {
			t.Fatalf("GET /login: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("status = %d, want 200", resp.StatusCode)
		}
		ct := resp.Header.Get("Content-Type")
		if ct == "" || ct[:9] != "text/html" {
			t.Errorf("Content-Type = %q, want text/html", ct)
		}
	})
}

func TestHandleLogin_AlreadyAuthenticated(t *testing.T) {
	t.Run("authenticated GET /login redirects to /", func(t *testing.T) {
		testUser := &models.User{
			ID:          1,
			Provider:    "google",
			ProviderID:  "g-1",
			Email:       "user@example.com",
			DisplayName: "User",
		}
		us := newMockUserStore(testUser)
		ts := newAuthServer(t, us)
		defer ts.Close()

		cookie := setSessionCookie(t, ts, 1)

		client := ts.Client()
		client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		}

		req, err := http.NewRequest(http.MethodGet, ts.URL+"/login", nil)
		if err != nil {
			t.Fatalf("new request: %v", err)
		}
		req.AddCookie(cookie)

		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("GET /login: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusSeeOther {
			t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusSeeOther)
		}
		loc := resp.Header.Get("Location")
		if loc != "/" {
			t.Errorf("Location = %q, want /", loc)
		}
	})
}

func TestHandleLogout(t *testing.T) {
	t.Run("POST /logout clears session and redirects to /login", func(t *testing.T) {
		testUser := &models.User{
			ID:          1,
			Provider:    "github",
			ProviderID:  "gh-1",
			Email:       "dev@example.com",
			DisplayName: "Dev",
		}
		us := newMockUserStore(testUser)
		ts := newAuthServer(t, us)
		defer ts.Close()

		cookie := setSessionCookie(t, ts, 1)

		client := ts.Client()
		client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		}

		// GET / with the session cookie to get the dashboard + CSRF token.
		getReq, _ := http.NewRequest(http.MethodGet, ts.URL+"/", nil)
		getReq.AddCookie(cookie)
		getResp, err := client.Do(getReq)
		if err != nil {
			t.Fatalf("GET /: %v", err)
		}

		// Extract CSRF cookie from the response.
		var csrfCookie *http.Cookie
		for _, c := range getResp.Cookies() {
			if c.Name == "_gorilla_csrf" {
				csrfCookie = c
				break
			}
		}
		if csrfCookie == nil {
			t.Fatal("CSRF cookie not found in GET / response")
		}

		// Read entire response body to find CSRF token in meta tag.
		bodyBytes, err := io.ReadAll(getResp.Body)
		getResp.Body.Close()
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		body := string(bodyBytes)

		// Extract token from: <meta name="csrf-token" content="TOKEN">
		// html/template may HTML-encode characters like + as &#43; so we
		// unescape the extracted value to get the raw base64 token.
		csrfToken := ""
		marker := `name="csrf-token" content="`
		if idx := findIndex(body, marker); idx >= 0 {
			start := idx + len(marker)
			end := findIndex(body[start:], `"`)
			if end >= 0 {
				csrfToken = html.UnescapeString(body[start : start+end])
			}
		}
		if csrfToken == "" {
			t.Fatal("could not extract CSRF token from dashboard page")
		}

		// POST /logout with the session cookie, CSRF cookie, and CSRF token.
		req, _ := http.NewRequest(http.MethodPost, ts.URL+"/logout", nil)
		req.AddCookie(cookie)
		req.AddCookie(csrfCookie)
		req.Header.Set("X-CSRF-Token", csrfToken)

		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("POST /logout: %v", err)
		}

		if resp.StatusCode != http.StatusSeeOther {
			failBody, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			t.Logf("CSRF cookie value len=%d", len(csrfCookie.Value))
			t.Logf("CSRF token len=%d, value=%q", len(csrfToken), csrfToken)
			t.Logf("Response body: %s", string(failBody))
			t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusSeeOther)
		} else {
			resp.Body.Close()
		}
		loc := resp.Header.Get("Location")
		if loc != "/login" {
			t.Errorf("Location = %q, want /login", loc)
		}
	})
}

// findIndex returns the index of substr in s, or -1 if not found.
func findIndex(s, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func TestHandleOAuthStart_UnknownProvider(t *testing.T) {
	t.Run("GET /auth/unknown returns 400", func(t *testing.T) {
		us := newMockUserStore()
		ts := newAuthServer(t, us)
		defer ts.Close()

		client := ts.Client()
		client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		}

		resp, err := client.Get(ts.URL + "/auth/unknown")
		if err != nil {
			t.Fatalf("GET /auth/unknown: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", resp.StatusCode)
		}
	})
}

func TestHandleOAuthStart_KnownProvider(t *testing.T) {
	t.Run("GET /auth/google redirects to Google", func(t *testing.T) {
		us := newMockUserStore()
		ts := newAuthServer(t, us)
		defer ts.Close()

		client := ts.Client()
		client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		}

		resp, err := client.Get(ts.URL + "/auth/google")
		if err != nil {
			t.Fatalf("GET /auth/google: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusTemporaryRedirect {
			t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusTemporaryRedirect)
		}
		loc := resp.Header.Get("Location")
		if loc == "" {
			t.Error("Location header missing")
		}
		if !containsString(loc, "accounts.google.com") {
			t.Errorf("Location = %q, expected to contain accounts.google.com", loc)
		}
	})
}

func containsString(s, substr string) bool {
	return findIndex(s, substr) >= 0
}

func TestHandleOAuthCallback_StateMismatch(t *testing.T) {
	t.Run("callback with wrong state redirects to /login", func(t *testing.T) {
		us := newMockUserStore()
		ts := newAuthServer(t, us)
		defer ts.Close()

		client := ts.Client()
		client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		}

		resp, err := client.Get(ts.URL + "/auth/google/callback?code=test&state=wrong")
		if err != nil {
			t.Fatalf("GET callback: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusSeeOther {
			t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusSeeOther)
		}
		loc := resp.Header.Get("Location")
		if loc != "/login" {
			t.Errorf("Location = %q, want /login", loc)
		}
	})
}

func TestUserFromContext(t *testing.T) {
	t.Run("returns nil for empty context", func(t *testing.T) {
		u := web.UserFromContext(context.Background())
		if u != nil {
			t.Errorf("expected nil user, got %v", u)
		}
	})
}

func TestHandleOAuthCallback_UnknownProvider(t *testing.T) {
	t.Run("callback with unknown provider returns 400", func(t *testing.T) {
		us := newMockUserStore()
		ts := newAuthServer(t, us)
		defer ts.Close()

		client := ts.Client()
		client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		}

		resp, err := client.Get(ts.URL + "/auth/unknown/callback?code=test&state=test")
		if err != nil {
			t.Fatalf("GET /auth/unknown/callback: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", resp.StatusCode)
		}
	})
}

func TestHandleOAuthCallback_ProviderError(t *testing.T) {
	t.Run("callback with error param from provider redirects to /login", func(t *testing.T) {
		us := newMockUserStore()
		ts := newAuthServer(t, us)
		defer ts.Close()

		client := ts.Client()
		client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		}

		// Start the OAuth flow to get a valid session with state stored.
		startResp, err := client.Get(ts.URL + "/auth/google")
		if err != nil {
			t.Fatalf("GET /auth/google: %v", err)
		}
		startResp.Body.Close()

		// Extract the redirect URL to get the state parameter.
		loc := startResp.Header.Get("Location")
		if loc == "" {
			t.Fatal("no Location header from /auth/google")
		}
		redirectURL, err := url.Parse(loc)
		if err != nil {
			t.Fatalf("parse redirect URL: %v", err)
		}
		state := redirectURL.Query().Get("state")
		if state == "" {
			t.Fatal("no state parameter in redirect URL")
		}

		// Collect all cookies from the start response (session + CSRF).
		cookies := startResp.Cookies()

		// Simulate provider returning an error (user denied consent) with
		// the valid state so it passes state verification and reaches the
		// error-param check.
		callbackURL := fmt.Sprintf("%s/auth/google/callback?error=access_denied&error_description=user+denied&state=%s", ts.URL, state)
		req, _ := http.NewRequest(http.MethodGet, callbackURL, nil)
		for _, c := range cookies {
			req.AddCookie(c)
		}

		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("GET callback with error: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusSeeOther {
			t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusSeeOther)
		}
		respLoc := resp.Header.Get("Location")
		if respLoc != "/login" {
			t.Errorf("Location = %q, want /login", respLoc)
		}
	})
}

func TestRequireAuth_DeletedUser(t *testing.T) {
	t.Run("session referencing non-existent user redirects to /login on protected route", func(t *testing.T) {
		// Create mock store with NO users — the session will reference user 99
		// which does not exist. On a requireAuth route this must redirect to /login.
		// (On an optionalAuth route like / it returns 200 as an anonymous visitor.)
		us := newMockUserStore()
		ts := newAuthServer(t, us)
		defer ts.Close()

		cookie := setSessionCookie(t, ts, 99)

		client := ts.Client()
		client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		}

		req, _ := http.NewRequest(http.MethodGet, ts.URL+"/settings", nil)
		req.AddCookie(cookie)

		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("GET /settings: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusSeeOther {
			t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusSeeOther)
		}
		loc := resp.Header.Get("Location")
		if loc != "/login" {
			t.Errorf("Location = %q, want /login", loc)
		}
	})
}

func TestDoubleLogout(t *testing.T) {
	t.Run("POST /logout without session is blocked (CSRF or requireAuth)", func(t *testing.T) {
		us := newMockUserStore()
		ts := newAuthServer(t, us)
		defer ts.Close()

		client := ts.Client()
		client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		}

		// POST /logout without any cookies. CSRF middleware fires before
		// requireAuth, so we get 403 (Forbidden). Either way the user cannot
		// hit the logout handler without a valid session.
		req, _ := http.NewRequest(http.MethodPost, ts.URL+"/logout", nil)
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("POST /logout: %v", err)
		}
		defer resp.Body.Close()

		// Expect 403 (CSRF) since there's no CSRF token. This proves
		// unauthenticated users cannot trigger the logout handler.
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("status = %d, want %d (CSRF rejection)", resp.StatusCode, http.StatusForbidden)
		}
	})
}

func TestHandleLogout_HTMX(t *testing.T) {
	t.Run("HTMX POST /logout returns HX-Redirect header", func(t *testing.T) {
		testUser := &models.User{
			ID:          1,
			Provider:    "github",
			ProviderID:  "gh-1",
			Email:       "dev@example.com",
			DisplayName: "Dev",
		}
		us := newMockUserStore(testUser)
		ts := newAuthServer(t, us)
		defer ts.Close()

		cookie := setSessionCookie(t, ts, 1)

		client := ts.Client()
		client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		}

		// GET / to obtain CSRF cookie and token.
		getReq, _ := http.NewRequest(http.MethodGet, ts.URL+"/", nil)
		getReq.AddCookie(cookie)
		getResp, err := client.Do(getReq)
		if err != nil {
			t.Fatalf("GET /: %v", err)
		}

		var csrfCookie *http.Cookie
		for _, c := range getResp.Cookies() {
			if c.Name == "_gorilla_csrf" {
				csrfCookie = c
				break
			}
		}
		if csrfCookie == nil {
			t.Fatal("CSRF cookie not found")
		}

		bodyBytes, err := io.ReadAll(getResp.Body)
		getResp.Body.Close()
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		body := string(bodyBytes)

		csrfToken := ""
		marker := `name="csrf-token" content="`
		if idx := findIndex(body, marker); idx >= 0 {
			start := idx + len(marker)
			end := findIndex(body[start:], `"`)
			if end >= 0 {
				csrfToken = html.UnescapeString(body[start : start+end])
			}
		}
		if csrfToken == "" {
			t.Fatal("could not extract CSRF token")
		}

		// POST /logout with HX-Request header.
		req, _ := http.NewRequest(http.MethodPost, ts.URL+"/logout", nil)
		req.AddCookie(cookie)
		req.AddCookie(csrfCookie)
		req.Header.Set("X-CSRF-Token", csrfToken)
		req.Header.Set("HX-Request", "true")

		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("POST /logout: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("status = %d, want 200", resp.StatusCode)
		}
		hxRedirect := resp.Header.Get("HX-Redirect")
		if hxRedirect != "/login" {
			t.Errorf("HX-Redirect = %q, want /login", hxRedirect)
		}
	})
}

func TestProtectedRoutes_Unauthenticated(t *testing.T) {
	routes := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/jobs/1"},
		{http.MethodGet, "/settings"},
		{http.MethodGet, "/api/jobs"},
		{http.MethodGet, "/api/jobs/1"},
	}

	us := newMockUserStore()
	ts := newAuthServer(t, us)
	defer ts.Close()

	client := ts.Client()
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}

	for _, rt := range routes {
		t.Run(fmt.Sprintf("%s %s redirects to /login", rt.method, rt.path), func(t *testing.T) {
			req, _ := http.NewRequest(rt.method, ts.URL+rt.path, nil)
			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("%s %s: %v", rt.method, rt.path, err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusSeeOther {
				t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusSeeOther)
			}
			loc := resp.Header.Get("Location")
			if loc != "/login" {
				t.Errorf("Location = %q, want /login", loc)
			}
		})
	}
}

func TestOptionalAuthRoutes_Unauthenticated(t *testing.T) {
	routes := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/"},
		{http.MethodGet, "/partials/job-table"},
	}

	us := newMockUserStore()
	ts := newAuthServer(t, us)
	defer ts.Close()

	client := ts.Client()
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}

	for _, rt := range routes {
		t.Run(fmt.Sprintf("%s %s returns 200 when unauthenticated", rt.method, rt.path), func(t *testing.T) {
			req, _ := http.NewRequest(rt.method, ts.URL+rt.path, nil)
			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("%s %s: %v", rt.method, rt.path, err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Errorf("status = %d, want %d (optionalAuth route should allow unauthenticated access)", resp.StatusCode, http.StatusOK)
			}
		})
	}
}

func TestPublicRoutes_NoAuth(t *testing.T) {
	routes := []struct {
		path       string
		wantStatus int
	}{
		{"/login", http.StatusOK},
		{"/health", http.StatusOK},
	}

	us := newMockUserStore()
	ts := newAuthServer(t, us)
	defer ts.Close()

	for _, rt := range routes {
		t.Run(fmt.Sprintf("GET %s returns %d without auth", rt.path, rt.wantStatus), func(t *testing.T) {
			resp, err := ts.Client().Get(ts.URL + rt.path)
			if err != nil {
				t.Fatalf("GET %s: %v", rt.path, err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != rt.wantStatus {
				t.Errorf("status = %d, want %d", resp.StatusCode, rt.wantStatus)
			}
		})
	}
}

// ─── mock mailer ─────────────────────────────────────────────────────────────

type mockMailer struct {
	mu   sync.Mutex
	sent []mockMailMessage
}

type mockMailMessage struct {
	To      string
	Subject string
	Body    string
}

func (m *mockMailer) SendMail(_ context.Context, to, subject, body string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sent = append(m.sent, mockMailMessage{To: to, Subject: subject, Body: body})
	return nil
}

func (m *mockMailer) LastSent() *mockMailMessage {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.sent) == 0 {
		return nil
	}
	msg := m.sent[len(m.sent)-1]
	return &msg
}

func (m *mockMailer) Count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.sent)
}

// newAuthServerWithMailer creates a test server wired with a mock mailer.
func newAuthServerWithMailer(t *testing.T, us *mockUserStore, mailer *mockMailer) *httptest.Server {
	t.Helper()
	cfg := newAuthConfig()
	ms := newMockJobStore()
	srv := web.NewServerWithConfig(ms, us, nil, cfg).WithMailer(mailer)
	return httptest.NewServer(srv.Handler())
}

// csrfTokenAndCookie performs a GET to path and extracts the CSRF token from
// the response HTML and the gorilla CSRF cookie from the Set-Cookie header.
// Both are required for subsequent POST requests.
func csrfTokenAndCookie(t *testing.T, ts *httptest.Server, path string) (token string, cookie *http.Cookie) {
	t.Helper()
	client := ts.Client()
	resp, err := client.Get(ts.URL + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)

	// Extract token from hidden input value.
	const needle = `name="gorilla.csrf.Token" value="`
	idx := strings.Index(bodyStr, needle)
	if idx < 0 {
		t.Fatalf("CSRF token input not found in %s response", path)
	}
	rest := bodyStr[idx+len(needle):]
	end := strings.Index(rest, `"`)
	if end < 0 {
		t.Fatalf("CSRF token value not terminated in %s response", path)
	}
	token = html.UnescapeString(rest[:end])

	for _, c := range resp.Cookies() {
		if strings.HasPrefix(c.Name, "_gorilla_csrf") || c.Name == "gorilla.csrf" || strings.Contains(c.Name, "csrf") {
			cookie = c
			break
		}
	}
	if cookie == nil {
		// Fall back to first cookie if none matched by name.
		if cs := resp.Cookies(); len(cs) > 0 {
			cookie = cs[0]
		}
	}
	return token, cookie
}

// postFormWithCSRF performs a POST with CSRF token and cookie included.
func postFormWithCSRF(t *testing.T, ts *httptest.Server, path string, formPath string, formData url.Values) *http.Response {
	t.Helper()

	csrfToken, csrfCookie := csrfTokenAndCookie(t, ts, formPath)
	formData.Set("gorilla.csrf.Token", csrfToken)

	client := ts.Client()
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}

	req, err := http.NewRequest(http.MethodPost, ts.URL+path, strings.NewReader(formData.Encode()))
	if err != nil {
		t.Fatalf("new POST request to %s: %v", path, err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if csrfCookie != nil {
		req.AddCookie(csrfCookie)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	return resp
}

// ─── email handler tests ──────────────────────────────────────────────────────

func TestHandleRegisterGet(t *testing.T) {
	t.Run("GET /register returns 200 with form HTML", func(t *testing.T) {
		us := newMockUserStore()
		ts := newAuthServer(t, us)
		defer ts.Close()

		resp, err := ts.Client().Get(ts.URL + "/register")
		if err != nil {
			t.Fatalf("GET /register: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("status = %d, want 200", resp.StatusCode)
		}
		ct := resp.Header.Get("Content-Type")
		if !strings.HasPrefix(ct, "text/html") {
			t.Errorf("Content-Type = %q, want text/html", ct)
		}
		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), "gorilla.csrf.Token") {
			t.Error("response should contain CSRF token input")
		}
	})
}

func TestHandleRegisterPost(t *testing.T) {
	tests := []struct {
		name       string
		formData   url.Values
		existingUsers []*models.User
		wantStatus int
		wantFlash  string
		wantRedirect string
	}{
		{
			name: "empty display name returns 200 with error",
			formData: url.Values{
				"email": {"user@example.com"},
				"password": {"password123"},
				"confirm_password": {"password123"},
			},
			wantStatus: http.StatusOK,
			wantFlash:  "Display name is required.",
		},
		{
			name: "invalid email returns 200 with error",
			formData: url.Values{
				"display_name": {"Test User"},
				"email":        {"not-an-email"},
				"password":     {"password123"},
				"confirm_password": {"password123"},
			},
			wantStatus: http.StatusOK,
			wantFlash:  "valid email",
		},
		{
			name: "short password returns 200 with error",
			formData: url.Values{
				"display_name": {"Test User"},
				"email":        {"user@example.com"},
				"password":     {"short"},
				"confirm_password": {"short"},
			},
			wantStatus: http.StatusOK,
			wantFlash:  "8 characters",
		},
		{
			name: "mismatched passwords returns 200 with error",
			formData: url.Values{
				"display_name": {"Test User"},
				"email":        {"user@example.com"},
				"password":     {"password123"},
				"confirm_password": {"differentpwd"},
			},
			wantStatus: http.StatusOK,
			wantFlash:  "do not match",
		},
		{
			name: "duplicate email returns 200 with error",
			formData: url.Values{
				"display_name": {"Test User"},
				"email":        {"existing@example.com"},
				"password":     {"password123"},
				"confirm_password": {"password123"},
			},
			existingUsers: []*models.User{
				{ID: 1, Email: "existing@example.com", Provider: "email"},
			},
			wantStatus: http.StatusOK,
			wantFlash:  "already exists",
		},
		{
			name: "successful registration redirects to /onboarding",
			formData: url.Values{
				"display_name": {"New User"},
				"email":        {"newuser@example.com"},
				"password":     {"securepassword"},
				"confirm_password": {"securepassword"},
			},
			wantStatus:   http.StatusSeeOther,
			wantRedirect: "/onboarding",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			us := newMockUserStore(tc.existingUsers...)
			mailer := &mockMailer{}
			ts := newAuthServerWithMailer(t, us, mailer)
			defer ts.Close()

			resp := postFormWithCSRF(t, ts, "/register", "/register", tc.formData)
			defer resp.Body.Close()

			if resp.StatusCode != tc.wantStatus {
				t.Errorf("status = %d, want %d", resp.StatusCode, tc.wantStatus)
			}

			if tc.wantFlash != "" && tc.wantStatus == http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				if !strings.Contains(string(body), tc.wantFlash) {
					t.Errorf("response body should contain %q\nbody: %s", tc.wantFlash, string(body))
				}
			}

			if tc.wantRedirect != "" {
				loc := resp.Header.Get("Location")
				if loc != tc.wantRedirect {
					t.Errorf("Location = %q, want %q", loc, tc.wantRedirect)
				}
			}
		})
	}
}

func TestHandleLoginPost(t *testing.T) {
	// bcrypt hash of "testpassword" at cost 4 (low cost for fast tests).
	pwdHash := "$2a$04$aUq.FfQQ77I.5Fc1qso2Fe5EFfgCqc9sM5RZkSQNa3abAiraJfkwa"

	tests := []struct {
		name          string
		formData      url.Values
		existingUsers []*models.User
		wantStatus    int
		wantFlash     string
		wantRedirect  string
	}{
		{
			name: "unknown email redirects to /login with flash",
			formData: url.Values{
				"email":    {"unknown@example.com"},
				"password": {"testpassword"},
			},
			wantStatus:   http.StatusSeeOther,
			wantRedirect: "/login",
		},
		{
			name: "wrong password redirects to /login with flash",
			formData: url.Values{
				"email":    {"user@example.com"},
				"password": {"wrongpassword"},
			},
			existingUsers: []*models.User{
				{ID: 1, Email: "user@example.com", Provider: "email", PasswordHash: &pwdHash},
			},
			wantStatus:   http.StatusSeeOther,
			wantRedirect: "/login",
		},
		{
			name: "successful login redirects to /onboarding when onboarding incomplete",
			formData: url.Values{
				"email":    {"user@example.com"},
				"password": {"testpassword"},
			},
			existingUsers: []*models.User{
				{ID: 1, Email: "user@example.com", Provider: "email", PasswordHash: &pwdHash, OnboardingComplete: false},
			},
			wantStatus:   http.StatusSeeOther,
			wantRedirect: "/onboarding",
		},
		{
			name: "successful login redirects to / when onboarding complete",
			formData: url.Values{
				"email":    {"user@example.com"},
				"password": {"testpassword"},
			},
			existingUsers: []*models.User{
				{ID: 1, Email: "user@example.com", Provider: "email", PasswordHash: &pwdHash, OnboardingComplete: true},
			},
			wantStatus:   http.StatusSeeOther,
			wantRedirect: "/",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			us := newMockUserStore(tc.existingUsers...)
			ts := newAuthServer(t, us)
			defer ts.Close()

			resp := postFormWithCSRF(t, ts, "/login", "/login", tc.formData)
			defer resp.Body.Close()

			if resp.StatusCode != tc.wantStatus {
				t.Errorf("status = %d, want %d", resp.StatusCode, tc.wantStatus)
			}
			if tc.wantRedirect != "" {
				loc := resp.Header.Get("Location")
				if loc != tc.wantRedirect {
					t.Errorf("Location = %q, want %q", loc, tc.wantRedirect)
				}
			}
		})
	}
}

func TestHandleForgotPasswordGet(t *testing.T) {
	t.Run("GET /forgot-password returns 200 with form", func(t *testing.T) {
		us := newMockUserStore()
		ts := newAuthServer(t, us)
		defer ts.Close()

		resp, err := ts.Client().Get(ts.URL + "/forgot-password")
		if err != nil {
			t.Fatalf("GET /forgot-password: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("status = %d, want 200", resp.StatusCode)
		}
	})
}

func TestHandleForgotPasswordPost(t *testing.T) {
	pwdHash := "$2a$12$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"

	tests := []struct {
		name          string
		email         string
		existingUsers []*models.User
		wantRedirect  string
		wantEmailSent bool
	}{
		{
			name:         "unknown email still redirects to /login with success flash",
			email:        "nobody@example.com",
			wantRedirect: "/login",
			wantEmailSent: false,
		},
		{
			name:  "known email with password sends reset email and redirects to /login",
			email: "user@example.com",
			existingUsers: []*models.User{
				{ID: 1, Email: "user@example.com", Provider: "email", PasswordHash: &pwdHash},
			},
			wantRedirect:  "/login",
			wantEmailSent: true,
		},
		{
			name:  "OAuth-only user (no password) does not send email",
			email: "oauth@example.com",
			existingUsers: []*models.User{
				{ID: 1, Email: "oauth@example.com", Provider: "google", PasswordHash: nil},
			},
			wantRedirect:  "/login",
			wantEmailSent: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			us := newMockUserStore(tc.existingUsers...)
			mailer := &mockMailer{}
			ts := newAuthServerWithMailer(t, us, mailer)
			defer ts.Close()

			resp := postFormWithCSRF(t, ts, "/forgot-password", "/forgot-password", url.Values{
				"email": {tc.email},
			})
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusSeeOther {
				t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusSeeOther)
			}
			loc := resp.Header.Get("Location")
			if loc != tc.wantRedirect {
				t.Errorf("Location = %q, want %q", loc, tc.wantRedirect)
			}
			if tc.wantEmailSent && mailer.Count() == 0 {
				t.Error("expected reset email to be sent, but none was")
			}
			if !tc.wantEmailSent && mailer.Count() > 0 {
				t.Errorf("expected no email, but %d were sent", mailer.Count())
			}
		})
	}
}

func TestHandleResetPasswordPost(t *testing.T) {
	resetToken := "validresettoken1234567890abcdef12345678"
	future := time.Now().Add(1 * time.Hour)

	tests := []struct {
		name          string
		formData      url.Values
		existingUsers []*models.User
		wantStatus    int
		wantRedirect  string
		wantFlash     string
	}{
		{
			name: "mismatched passwords redirects back",
			formData: url.Values{
				"token":            {resetToken},
				"password":         {"newpassword1"},
				"confirm_password": {"newpassword2"},
			},
			existingUsers: []*models.User{
				{ID: 1, Email: "user@example.com", ResetToken: &resetToken, ResetExpiresAt: &future},
			},
			wantStatus:   http.StatusSeeOther,
			wantRedirect: "/reset-password?token=" + resetToken,
		},
		{
			name: "short password redirects back",
			formData: url.Values{
				"token":            {resetToken},
				"password":         {"short"},
				"confirm_password": {"short"},
			},
			existingUsers: []*models.User{
				{ID: 1, Email: "user@example.com", ResetToken: &resetToken, ResetExpiresAt: &future},
			},
			wantStatus:   http.StatusSeeOther,
			wantRedirect: "/reset-password?token=" + resetToken,
		},
		{
			name: "expired token redirects to /forgot-password",
			formData: url.Values{
				"token":            {"expiredtoken"},
				"password":         {"newpassword1"},
				"confirm_password": {"newpassword1"},
			},
			wantStatus:   http.StatusSeeOther,
			wantRedirect: "/forgot-password",
		},
		{
			name: "valid token redirects to /",
			formData: url.Values{
				"token":            {resetToken},
				"password":         {"newpassword1"},
				"confirm_password": {"newpassword1"},
			},
			existingUsers: []*models.User{
				{ID: 1, Email: "user@example.com", ResetToken: &resetToken, ResetExpiresAt: &future},
			},
			wantStatus:   http.StatusSeeOther,
			wantRedirect: "/",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			us := newMockUserStore(tc.existingUsers...)
			ts := newAuthServer(t, us)
			defer ts.Close()

			// For the reset password form, we need to get CSRF token from /forgot-password
			// since the reset form requires a valid token query param.
			resp := postFormWithCSRF(t, ts, "/reset-password", "/forgot-password", tc.formData)
			defer resp.Body.Close()

			if resp.StatusCode != tc.wantStatus {
				t.Errorf("status = %d, want %d", resp.StatusCode, tc.wantStatus)
			}
			loc := resp.Header.Get("Location")
			if loc != tc.wantRedirect {
				t.Errorf("Location = %q, want %q", loc, tc.wantRedirect)
			}
		})
	}
}

func TestHandleVerifyEmail(t *testing.T) {
	validToken := "validverifytoken1234567890abcdef12345678"
	future := time.Now().Add(24 * time.Hour)

	tests := []struct {
		name          string
		token         string
		existingUsers []*models.User
		wantRedirect  string
	}{
		{
			name:         "invalid token redirects to /login",
			token:        "invalidtoken",
			wantRedirect: "/login",
		},
		{
			name:  "valid token redirects to /",
			token: validToken,
			existingUsers: []*models.User{
				{ID: 1, Email: "user@example.com", EmailVerifyToken: &validToken, EmailVerifyExpiresAt: &future},
			},
			wantRedirect: "/",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			us := newMockUserStore(tc.existingUsers...)
			ts := newAuthServer(t, us)
			defer ts.Close()

			client := ts.Client()
			client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			}

			resp, err := client.Get(ts.URL + "/verify-email?token=" + tc.token)
			if err != nil {
				t.Fatalf("GET /verify-email: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusSeeOther {
				t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusSeeOther)
			}
			loc := resp.Header.Get("Location")
			if loc != tc.wantRedirect {
				t.Errorf("Location = %q, want %q", loc, tc.wantRedirect)
			}
		})
	}
}
