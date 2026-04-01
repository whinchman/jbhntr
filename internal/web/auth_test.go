package web_test

import (
	"context"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gorilla/sessions"

	"github.com/whinchman/jobhuntr/internal/config"
	"github.com/whinchman/jobhuntr/internal/models"
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

// ─── helpers ────────────────────────────────────────────────────────────────

func newAuthConfig() *config.Config {
	return &config.Config{
		Server: config.ServerConfig{
			Port:    8080,
			BaseURL: "http://localhost:8080",
		},
		Auth: config.AuthConfig{
			SessionSecret: "test-secret-that-is-at-least-32-bytes",
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
	srv := web.NewServerWithConfig(ms, us, cfg, "", "")
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
	t.Run("unauthenticated request redirects to /login", func(t *testing.T) {
		us := newMockUserStore()
		ts := newAuthServer(t, us)
		defer ts.Close()

		client := ts.Client()
		client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		}

		resp, err := client.Get(ts.URL + "/")
		if err != nil {
			t.Fatalf("GET /: %v", err)
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
	t.Run("session referencing non-existent user redirects to /login", func(t *testing.T) {
		// Create mock store with NO users — the session will reference user 99
		// which does not exist.
		us := newMockUserStore()
		ts := newAuthServer(t, us)
		defer ts.Close()

		cookie := setSessionCookie(t, ts, 99)

		client := ts.Client()
		client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		}

		req, _ := http.NewRequest(http.MethodGet, ts.URL+"/", nil)
		req.AddCookie(cookie)

		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("GET /: %v", err)
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
		{http.MethodGet, "/"},
		{http.MethodGet, "/jobs/1"},
		{http.MethodGet, "/settings"},
		{http.MethodGet, "/api/jobs"},
		{http.MethodGet, "/api/jobs/1"},
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
