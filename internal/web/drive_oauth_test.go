package web_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"golang.org/x/oauth2"

	"github.com/whinchman/jobhuntr/internal/config"
	"github.com/whinchman/jobhuntr/internal/models"
	"github.com/whinchman/jobhuntr/internal/web"
)

// ─── helpers ─────────────────────────────────────────────────────────────────

// newDriveAuthConfig returns a config with both regular auth and Drive OAuth
// credentials populated.
func newDriveAuthConfig() *config.Config {
	cfg := newAuthConfig()
	cfg.GoogleDrive = config.GoogleDriveConfig{
		ClientID:     "drive-client-id",
		ClientSecret: "drive-client-secret",
	}
	return cfg
}

// newDriveServer builds a test HTTP server wired with the Drive token store
// and a fake Drive OAuth provider pointing at a mock Google endpoint.
func newDriveServer(t *testing.T, mockGoogle *httptest.Server) (*httptest.Server, *mockDriveTokenStore, *mockUserStore) {
	t.Helper()
	cfg := newDriveAuthConfig()
	us := newMockUserStore(&models.User{ID: 1, Email: "user@example.com", OnboardingComplete: true})
	ms := newMockJobStore()
	dts := newMockDriveTokenStore()

	srv := web.NewServerWithConfig(ms, us, nil, cfg).WithDriveTokenStore(dts)

	// Override the Drive OAuth config to point at the mock Google server.
	driveOAuthCfg := &oauth2.Config{
		ClientID:     "drive-client-id",
		ClientSecret: "drive-client-secret",
		RedirectURL:  "http://localhost/auth/google-drive/callback",
		Scopes:       []string{"https://www.googleapis.com/auth/drive.file"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  mockGoogle.URL + "/auth",
			TokenURL: mockGoogle.URL + "/token",
		},
	}
	srv.WithTestDriveOAuthConfig(driveOAuthCfg)

	return httptest.NewServer(srv.Handler()), dts, us
}

// ─── route registration ───────────────────────────────────────────────────────

func TestDriveRoutes_NotRegisteredWithoutConfig(t *testing.T) {
	t.Run("no Drive routes when ClientID is empty", func(t *testing.T) {
		cfg := newAuthConfig() // no GoogleDrive section
		us := newMockUserStore(&models.User{ID: 1, Email: "user@example.com", OnboardingComplete: true})
		ms := newMockJobStore()

		srv := web.NewServerWithConfig(ms, us, nil, cfg)
		ts := httptest.NewServer(srv.Handler())
		defer ts.Close()

		client := ts.Client()
		client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		}

		cookie := setSessionCookie(t, ts, 1)

		req, _ := http.NewRequest(http.MethodGet, ts.URL+"/auth/google-drive", nil)
		req.AddCookie(cookie)
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("request error: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("expected 404 for unregistered Drive route, got %d", resp.StatusCode)
		}
	})
}

func TestDriveRoutes_NotRegisteredWithoutTokenStore(t *testing.T) {
	t.Run("Drive routes not registered when token store is nil", func(t *testing.T) {
		cfg := newDriveAuthConfig()
		us := newMockUserStore(&models.User{ID: 1, Email: "user@example.com", OnboardingComplete: true})
		ms := newMockJobStore()

		// No WithDriveTokenStore call — driveTokenStore is nil.
		srv := web.NewServerWithConfig(ms, us, nil, cfg)
		ts := httptest.NewServer(srv.Handler())
		defer ts.Close()

		client := ts.Client()
		client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		}

		cookie := setSessionCookie(t, ts, 1)

		req, _ := http.NewRequest(http.MethodGet, ts.URL+"/auth/google-drive", nil)
		req.AddCookie(cookie)
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("request error: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("expected 404 when token store is absent, got %d", resp.StatusCode)
		}
	})
}

// ─── handleDriveOAuthStart ────────────────────────────────────────────────────

func TestHandleDriveOAuthStart(t *testing.T) {
	// Minimal mock — the start handler only redirects; it never contacts Google.
	mockGoogle := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer mockGoogle.Close()

	ts, _, _ := newDriveServer(t, mockGoogle)
	defer ts.Close()

	client := ts.Client()
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}

	t.Run("redirects to Google auth URL", func(t *testing.T) {
		cookie := setSessionCookie(t, ts, 1)
		req, _ := http.NewRequest(http.MethodGet, ts.URL+"/auth/google-drive", nil)
		req.AddCookie(cookie)

		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusTemporaryRedirect {
			t.Errorf("status = %d, want 307", resp.StatusCode)
		}
		loc := resp.Header.Get("Location")
		if !strings.Contains(loc, mockGoogle.URL+"/auth") {
			t.Errorf("redirect location %q does not point to mock Google auth", loc)
		}
	})

	t.Run("redirect URL contains state parameter", func(t *testing.T) {
		cookie := setSessionCookie(t, ts, 1)
		req, _ := http.NewRequest(http.MethodGet, ts.URL+"/auth/google-drive", nil)
		req.AddCookie(cookie)

		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("request: %v", err)
		}
		defer resp.Body.Close()

		loc := resp.Header.Get("Location")
		parsed, err := url.Parse(loc)
		if err != nil {
			t.Fatalf("parse location: %v", err)
		}
		state := parsed.Query().Get("state")
		if state == "" {
			t.Error("expected non-empty state in redirect URL")
		}
	})

	t.Run("unauthenticated request is redirected to login", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, ts.URL+"/auth/google-drive", nil)
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusSeeOther {
			t.Errorf("expected redirect to /login, got %d", resp.StatusCode)
		}
		if !strings.Contains(resp.Header.Get("Location"), "/login") {
			t.Errorf("expected redirect to /login, got %q", resp.Header.Get("Location"))
		}
	})
}

// ─── handleDriveOAuthCallback ─────────────────────────────────────────────────

func TestHandleDriveOAuthCallback_StateMismatch(t *testing.T) {
	mockGoogle := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer mockGoogle.Close()

	ts, _, _ := newDriveServer(t, mockGoogle)
	defer ts.Close()

	client := ts.Client()
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}

	t.Run("returns 400 on state mismatch", func(t *testing.T) {
		cookie := setSessionCookie(t, ts, 1)
		// Call the callback without a prior start — session has no state stored.
		req, _ := http.NewRequest(http.MethodGet, ts.URL+"/auth/google-drive/callback?code=abc&state=wrong-state", nil)
		req.AddCookie(cookie)

		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("expected 400 on state mismatch, got %d", resp.StatusCode)
		}
	})
}

func TestHandleDriveOAuthCallback_ProviderError(t *testing.T) {
	mockGoogle := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer mockGoogle.Close()

	ts, _, _ := newDriveServer(t, mockGoogle)
	defer ts.Close()

	client := ts.Client()
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}

	t.Run("redirects to / on provider error after valid state", func(t *testing.T) {
		// Initiate the flow to get a valid state in the session.
		cookie := setSessionCookie(t, ts, 1)
		startReq, _ := http.NewRequest(http.MethodGet, ts.URL+"/auth/google-drive", nil)
		startReq.AddCookie(cookie)
		startResp, err := client.Do(startReq)
		if err != nil {
			t.Fatalf("start request: %v", err)
		}
		defer startResp.Body.Close()

		loc := startResp.Header.Get("Location")
		parsed, _ := url.Parse(loc)
		state := parsed.Query().Get("state")

		// Collect cookies from the start response (session now contains state).
		sessionCookies := startResp.Cookies()
		if len(sessionCookies) == 0 {
			sessionCookies = []*http.Cookie{cookie}
		}

		// Simulate Google returning an error.
		callbackURL := ts.URL + "/auth/google-drive/callback?error=access_denied&state=" + state
		callbackReq, _ := http.NewRequest(http.MethodGet, callbackURL, nil)
		for _, c := range sessionCookies {
			callbackReq.AddCookie(c)
		}

		resp, err := client.Do(callbackReq)
		if err != nil {
			t.Fatalf("callback request: %v", err)
		}
		defer resp.Body.Close()

		// On provider error with valid state, the handler redirects to returnTo ("/").
		if resp.StatusCode != http.StatusSeeOther {
			t.Errorf("expected 303 redirect, got %d", resp.StatusCode)
		}
	})
}

func TestHandleDriveOAuthCallback_HappyPath(t *testing.T) {
	// Mock Google token endpoint that returns a minimal access token.
	mockGoogle := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"access_token":"test-access-token","token_type":"Bearer","expires_in":3600}`))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer mockGoogle.Close()

	ts, dts, _ := newDriveServer(t, mockGoogle)
	defer ts.Close()

	client := ts.Client()
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}

	t.Run("stores encrypted token and redirects", func(t *testing.T) {
		// Step 1: initiate the flow.
		cookie := setSessionCookie(t, ts, 1)
		startReq, _ := http.NewRequest(http.MethodGet, ts.URL+"/auth/google-drive", nil)
		startReq.AddCookie(cookie)
		startResp, err := client.Do(startReq)
		if err != nil {
			t.Fatalf("start request: %v", err)
		}
		defer startResp.Body.Close()

		loc := startResp.Header.Get("Location")
		parsed, _ := url.Parse(loc)
		state := parsed.Query().Get("state")

		// Collect updated session cookies from start response.
		sessionCookies := startResp.Cookies()
		if len(sessionCookies) == 0 {
			sessionCookies = []*http.Cookie{cookie}
		}

		// Step 2: call the callback with matching state and a fake code.
		callbackURL := ts.URL + "/auth/google-drive/callback?code=fake-code&state=" + state
		callbackReq, _ := http.NewRequest(http.MethodGet, callbackURL, nil)
		for _, c := range sessionCookies {
			callbackReq.AddCookie(c)
		}

		resp, err := client.Do(callbackReq)
		if err != nil {
			t.Fatalf("callback request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusSeeOther {
			t.Errorf("expected 303 redirect after successful callback, got %d", resp.StatusCode)
		}

		// The token should now be stored in the mock store.
		stored, err := dts.GetGoogleDriveToken(context.Background(), 1)
		if err != nil {
			t.Fatalf("GetGoogleDriveToken: %v", err)
		}
		if stored == "" {
			t.Error("expected a non-empty encrypted token to be stored")
		}
	})
}
