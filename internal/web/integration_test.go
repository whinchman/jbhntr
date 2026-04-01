package web_test

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gorilla/sessions"
	"golang.org/x/oauth2"

	"github.com/whinchman/jobhuntr/internal/config"
	"github.com/whinchman/jobhuntr/internal/models"
	"github.com/whinchman/jobhuntr/internal/store"
	"github.com/whinchman/jobhuntr/internal/web"
)

// ─── mock OAuth provider ───────────────────────────────────────────────────

// providerProfile holds fields returned by the mock OAuth user-info endpoints.
type providerProfile struct {
	ID          string
	Email       string
	DisplayName string
	AvatarURL   string
}

// mockOAuthProvider starts an httptest.Server that serves:
//
//	POST /token     — returns a valid access_token JSON response
//	GET  /userinfo  — returns a JSON user profile (Google format)
//	GET  /user      — returns a JSON user profile (GitHub format)
//	GET  /user/emails — returns a JSON array of GitHub emails
func mockOAuthProvider(t *testing.T, profile providerProfile) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	mux.HandleFunc("POST /token", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"access_token": "test-token",
			"token_type":   "Bearer",
		})
	})

	mux.HandleFunc("GET /userinfo", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"id":      profile.ID,
			"email":   profile.Email,
			"name":    profile.DisplayName,
			"picture": profile.AvatarURL,
		})
	})

	mux.HandleFunc("GET /user", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":         42,
			"login":      profile.DisplayName,
			"name":       profile.DisplayName,
			"email":      profile.Email,
			"avatar_url": profile.AvatarURL,
		})
	})

	mux.HandleFunc("GET /user/emails", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"email": profile.Email, "primary": true, "verified": true},
		})
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// mockProviderTransport intercepts HTTP requests to Google/GitHub user-info
// APIs and redirects them to the mock OAuth provider. All other requests are
// sent through the underlying base transport.
type mockProviderTransport struct {
	mockURL string
	base    http.RoundTripper
}

func (t *mockProviderTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	target := req.URL.String()

	// Redirect Google userinfo endpoint.
	if strings.Contains(target, "googleapis.com/oauth2/v2/userinfo") {
		newURL := t.mockURL + "/userinfo"
		newReq, _ := http.NewRequestWithContext(req.Context(), req.Method, newURL, req.Body)
		for k, v := range req.Header {
			newReq.Header[k] = v
		}
		return t.base.RoundTrip(newReq)
	}

	// Redirect GitHub user endpoint — check /user/emails before /user.
	if strings.Contains(target, "api.github.com/user/emails") {
		newURL := t.mockURL + "/user/emails"
		newReq, _ := http.NewRequestWithContext(req.Context(), req.Method, newURL, req.Body)
		for k, v := range req.Header {
			newReq.Header[k] = v
		}
		return t.base.RoundTrip(newReq)
	}
	if strings.Contains(target, "api.github.com/user") {
		newURL := t.mockURL + "/user"
		newReq, _ := http.NewRequestWithContext(req.Context(), req.Method, newURL, req.Body)
		for k, v := range req.Header {
			newReq.Header[k] = v
		}
		return t.base.RoundTrip(newReq)
	}

	return t.base.RoundTrip(req)
}

// ─── cookie jar ────────────────────────────────────────────────────────────

// newTestCookieJar creates a cookie jar that accepts all cookies regardless
// of domain, which is needed for httptest servers that use 127.0.0.1.
func newTestCookieJar() http.CookieJar {
	jar, _ := cookiejar.New(nil)
	return jar
}

// ─── integration helpers ───────────────────────────────────────────────────

// newIntegrationServer creates an httptest.Server wired to a real in-memory
// SQLite store and a mock OAuth provider. It returns the test server and the
// underlying store (for assertions).
func newIntegrationServer(t *testing.T, providerName string, profile providerProfile) (*httptest.Server, *store.Store) {
	t.Helper()

	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open in-memory store: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	mock := mockOAuthProvider(t, profile)

	cfg := &config.Config{
		Server: config.ServerConfig{
			Port:    0,
			BaseURL: "http://localhost",
		},
		Auth: config.AuthConfig{
			SessionSecret: "integration-test-secret-at-least-32-bytes!",
			Providers: config.ProvidersConfig{
				Google: config.OAuthProviderConfig{
					ClientID:     "test-client-id",
					ClientSecret: "test-client-secret",
				},
				GitHub: config.OAuthProviderConfig{
					ClientID:     "test-github-id",
					ClientSecret: "test-github-secret",
				},
			},
		},
	}

	srv := web.NewServerWithConfig(db, db, db, cfg)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	// Override the OAuth provider to point at the mock.
	srv.WithTestOAuthProvider(providerName, &oauth2.Config{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		RedirectURL:  ts.URL + "/auth/" + providerName + "/callback",
		Scopes:       []string{"openid", "email", "profile"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  mock.URL + "/authorize",
			TokenURL: mock.URL + "/token",
		},
	})

	// Redirect Google/GitHub API calls to the mock provider.
	oldTransport := http.DefaultTransport
	http.DefaultTransport = &mockProviderTransport{mockURL: mock.URL, base: oldTransport}
	t.Cleanup(func() { http.DefaultTransport = oldTransport })

	return ts, db
}

// noRedirectClient returns an HTTP client that does not follow redirects.
func noRedirectClient() *http.Client {
	return &http.Client{
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// setIntegrationSessionCookie creates a session cookie for a given user ID
// using the same session secret as the integration server.
func setIntegrationSessionCookie(t *testing.T, userID int64) *http.Cookie {
	t.Helper()
	sessStore := sessions.NewCookieStore([]byte("integration-test-secret-at-least-32-bytes!"))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	sess, err := sessStore.New(req, "jobhuntr_session")
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

// ─── integration tests ─────────────────────────────────────────────────────

func TestIntegration_OAuthLoginFlow(t *testing.T) {
	profile := providerProfile{
		ID:          "google-123",
		Email:       "alice@example.com",
		DisplayName: "Alice",
		AvatarURL:   "https://example.com/alice.png",
	}
	ts, db := newIntegrationServer(t, "google", profile)

	// Use a cookie jar to automatically manage cookies across requests,
	// just like a real browser.
	jar := newTestCookieJar()
	client := &http.Client{
		Jar: jar,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// Step 1: GET / — unauthenticated → redirect to /login.
	t.Run("unauthenticated root redirects to login", func(t *testing.T) {
		resp, err := client.Get(ts.URL + "/")
		if err != nil {
			t.Fatalf("GET /: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusSeeOther {
			t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusSeeOther)
		}
		if loc := resp.Header.Get("Location"); loc != "/login" {
			t.Fatalf("Location = %q, want /login", loc)
		}
	})

	// Step 2: GET /auth/google — redirect to mock provider's authorize URL.
	var state string
	t.Run("oauth start redirects to provider", func(t *testing.T) {
		resp, err := client.Get(ts.URL + "/auth/google")
		if err != nil {
			t.Fatalf("GET /auth/google: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusTemporaryRedirect {
			t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusTemporaryRedirect)
		}

		loc := resp.Header.Get("Location")
		parsed, err := url.Parse(loc)
		if err != nil {
			t.Fatalf("parse Location: %v", err)
		}
		state = parsed.Query().Get("state")
		if state == "" {
			t.Fatal("no state parameter in redirect URL")
		}
	})

	// Step 3: Simulate provider callback.
	t.Run("oauth callback creates session and redirects to root", func(t *testing.T) {
		callbackURL := fmt.Sprintf("%s/auth/google/callback?code=test-code&state=%s", ts.URL, url.QueryEscape(state))
		resp, err := client.Get(callbackURL)
		if err != nil {
			t.Fatalf("GET callback: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusSeeOther {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("status = %d, want %d; body = %s", resp.StatusCode, http.StatusSeeOther, body)
		}
		if loc := resp.Header.Get("Location"); loc != "/" {
			t.Fatalf("Location = %q, want /", loc)
		}
	})

	// Step 4: Follow redirect to GET / — now authenticated.
	t.Run("authenticated root returns 200", func(t *testing.T) {
		resp, err := client.Get(ts.URL + "/")
		if err != nil {
			t.Fatalf("GET /: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("status = %d, want 200; body = %s", resp.StatusCode, body)
		}
		ct := resp.Header.Get("Content-Type")
		if !strings.HasPrefix(ct, "text/html") {
			t.Errorf("Content-Type = %q, want text/html", ct)
		}
	})

	// Step 5: Verify user in DB.
	t.Run("user created in database", func(t *testing.T) {
		user, err := db.GetUserByProvider(context.Background(), "google", "google-123")
		if err != nil {
			t.Fatalf("GetUserByProvider: %v", err)
		}
		if user.Email != "alice@example.com" {
			t.Errorf("email = %q, want alice@example.com", user.Email)
		}
		if user.DisplayName != "Alice" {
			t.Errorf("display_name = %q, want Alice", user.DisplayName)
		}
		if user.Provider != "google" {
			t.Errorf("provider = %q, want google", user.Provider)
		}
	})
}

func TestIntegration_UserIsolation_Jobs(t *testing.T) {
	// Setup: in-memory store with two users and their jobs.
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	ctx := context.Background()

	userA, err := db.UpsertUser(ctx, &models.User{
		Provider: "google", ProviderID: "ua", Email: "a@test.com", DisplayName: "User A",
	})
	if err != nil {
		t.Fatalf("upsert user A: %v", err)
	}

	userB, err := db.UpsertUser(ctx, &models.User{
		Provider: "google", ProviderID: "ub", Email: "b@test.com", DisplayName: "User B",
	})
	if err != nil {
		t.Fatalf("upsert user B: %v", err)
	}

	jobA := &models.Job{ExternalID: "ext-a", Source: "test", Title: "User A's Job", Company: "Co A", Status: models.StatusDiscovered}
	jobB := &models.Job{ExternalID: "ext-b", Source: "test", Title: "User B's Job", Company: "Co B", Status: models.StatusDiscovered}

	if _, err := db.CreateJob(ctx, userA.ID, jobA); err != nil {
		t.Fatalf("create job A: %v", err)
	}
	if _, err := db.CreateJob(ctx, userB.ID, jobB); err != nil {
		t.Fatalf("create job B: %v", err)
	}

	cfg := &config.Config{
		Server: config.ServerConfig{BaseURL: "http://localhost"},
		Auth: config.AuthConfig{
			SessionSecret: "integration-test-secret-at-least-32-bytes!",
			Providers: config.ProvidersConfig{
				Google: config.OAuthProviderConfig{ClientID: "test", ClientSecret: "test"},
			},
		},
	}
	srv := web.NewServerWithConfig(db, db, db, cfg)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	cookieA := setIntegrationSessionCookie(t, userA.ID)
	cookieB := setIntegrationSessionCookie(t, userB.ID)
	client := noRedirectClient()

	cases := []struct {
		name       string
		cookie     *http.Cookie
		path       string
		wantStatus int
		wantTitle  string // for 200 list responses
		wantCount  int    // for 200 list responses
	}{
		{
			name:       "user A sees only own jobs",
			cookie:     cookieA,
			path:       "/api/jobs",
			wantStatus: http.StatusOK,
			wantTitle:  "User A's Job",
			wantCount:  1,
		},
		{
			name:       "user A cannot access user B's job",
			cookie:     cookieA,
			path:       fmt.Sprintf("/api/jobs/%d", jobB.ID),
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "user B sees only own jobs",
			cookie:     cookieB,
			path:       "/api/jobs",
			wantStatus: http.StatusOK,
			wantTitle:  "User B's Job",
			wantCount:  1,
		},
		{
			name:       "user B cannot access user A's job",
			cookie:     cookieB,
			path:       fmt.Sprintf("/api/jobs/%d", jobA.ID),
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req, _ := http.NewRequest(http.MethodGet, ts.URL+tc.path, nil)
			req.AddCookie(tc.cookie)

			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("GET %s: %v", tc.path, err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tc.wantStatus {
				body, _ := io.ReadAll(resp.Body)
				t.Fatalf("status = %d, want %d; body = %s", resp.StatusCode, tc.wantStatus, body)
			}

			if tc.wantCount > 0 {
				var jobs []models.Job
				if err := json.NewDecoder(resp.Body).Decode(&jobs); err != nil {
					t.Fatalf("decode jobs: %v", err)
				}
				if len(jobs) != tc.wantCount {
					t.Errorf("len(jobs) = %d, want %d", len(jobs), tc.wantCount)
				}
				if len(jobs) > 0 && jobs[0].Title != tc.wantTitle {
					t.Errorf("jobs[0].Title = %q, want %q", jobs[0].Title, tc.wantTitle)
				}
			}
		})
	}
}

func TestIntegration_PerUserSettings(t *testing.T) {
	// Setup: in-memory store with two users.
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	ctx := context.Background()

	userA, err := db.UpsertUser(ctx, &models.User{
		Provider: "google", ProviderID: "ua", Email: "a@test.com", DisplayName: "User A",
	})
	if err != nil {
		t.Fatalf("upsert user A: %v", err)
	}

	userB, err := db.UpsertUser(ctx, &models.User{
		Provider: "google", ProviderID: "ub", Email: "b@test.com", DisplayName: "User B",
	})
	if err != nil {
		t.Fatalf("upsert user B: %v", err)
	}

	cfg := &config.Config{
		Server: config.ServerConfig{BaseURL: "http://localhost"},
		Auth: config.AuthConfig{
			SessionSecret: "integration-test-secret-at-least-32-bytes!",
			Providers: config.ProvidersConfig{
				Google: config.OAuthProviderConfig{ClientID: "test", ClientSecret: "test"},
			},
		},
	}
	srv := web.NewServerWithConfig(db, db, db, cfg)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	cookieA := setIntegrationSessionCookie(t, userA.ID)
	cookieB := setIntegrationSessionCookie(t, userB.ID)
	client := noRedirectClient()

	// getCSRFWithCookie fetches the settings page and returns the CSRF token
	// and CSRF cookie from the response. Uses manual cookie management.
	getCSRFWithCookie := func(t *testing.T, sessCookie *http.Cookie) (string, *http.Cookie) {
		t.Helper()
		req, _ := http.NewRequest(http.MethodGet, ts.URL+"/settings", nil)
		req.AddCookie(sessCookie)

		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("GET /settings: %v", err)
		}
		bodyBytes, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET /settings status = %d; body = %s", resp.StatusCode, bodyBytes)
		}

		var csrfCookie *http.Cookie
		for _, c := range resp.Cookies() {
			if c.Name == "_gorilla_csrf" {
				csrfCookie = c
				break
			}
		}
		if csrfCookie == nil {
			t.Fatal("CSRF cookie not found")
		}

		body := string(bodyBytes)
		csrfToken := ""
		marker := `name="csrf-token" content="`
		if idx := strings.Index(body, marker); idx >= 0 {
			start := idx + len(marker)
			end := strings.Index(body[start:], `"`)
			if end >= 0 {
				csrfToken = html.UnescapeString(body[start : start+end])
			}
		}
		if csrfToken == "" {
			t.Fatal("could not extract CSRF token")
		}
		return csrfToken, csrfCookie
	}

	// Step a: User A saves resume.
	t.Run("user A saves resume", func(t *testing.T) {
		csrfToken, csrfCookie := getCSRFWithCookie(t, cookieA)

		form := url.Values{"resume": {"# Alice Resume"}}
		req, _ := http.NewRequest(http.MethodPost, ts.URL+"/settings/resume", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("X-CSRF-Token", csrfToken)
		req.AddCookie(cookieA)
		req.AddCookie(csrfCookie)

		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("POST /settings/resume: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusSeeOther {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("status = %d, want 303; body = %s", resp.StatusCode, body)
		}
		loc := resp.Header.Get("Location")
		if !strings.Contains(loc, "/settings") {
			t.Errorf("Location = %q, want /settings?saved=1", loc)
		}
	})

	// Step b: User A adds a filter (fresh CSRF token).
	t.Run("user A adds filter", func(t *testing.T) {
		csrfToken, csrfCookie := getCSRFWithCookie(t, cookieA)

		form := url.Values{
			"keywords":   {"golang"},
			"location":   {"Remote"},
			"min_salary": {"150000"},
		}
		req, _ := http.NewRequest(http.MethodPost, ts.URL+"/settings/filters", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("X-CSRF-Token", csrfToken)
		req.AddCookie(cookieA)
		req.AddCookie(csrfCookie)

		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("POST /settings/filters: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusSeeOther {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("status = %d, want 303; body = %s", resp.StatusCode, body)
		}
	})

	// Step c: User B's settings are isolated from User A.
	t.Run("user B has no trace of user A settings", func(t *testing.T) {
		// Verify via HTTP: User B's settings page does not contain A's data.
		req, _ := http.NewRequest(http.MethodGet, ts.URL+"/settings", nil)
		req.AddCookie(cookieB)

		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("GET /settings as B: %v", err)
		}
		bodyBytes, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET /settings as B status = %d", resp.StatusCode)
		}
		body := string(bodyBytes)
		if strings.Contains(body, "Alice Resume") {
			t.Error("user B's settings page contains 'Alice Resume'")
		}
		// Check that user A's filter data does not appear in the table.
		// "golang" appears in the placeholder text, so check for the
		// table cell pattern: <td>golang</td>
		if strings.Contains(body, "<td>golang</td>") {
			t.Error("user B's settings page contains user A's golang filter")
		}
		if strings.Contains(body, "<td>Remote</td>") {
			t.Error("user B's settings page contains user A's Remote location filter")
		}

		// Verify via DB: no filters or resume for User B.
		filtersB, err := db.ListUserFilters(ctx, userB.ID)
		if err != nil {
			t.Fatalf("ListUserFilters(B): %v", err)
		}
		if len(filtersB) != 0 {
			t.Errorf("user B filters = %d, want 0", len(filtersB))
		}

		uB, err := db.GetUser(ctx, userB.ID)
		if err != nil {
			t.Fatalf("GetUser(B): %v", err)
		}
		if uB.ResumeMarkdown != "" {
			t.Errorf("user B resume = %q, want empty", uB.ResumeMarkdown)
		}
	})

	// Step d: User A's settings persisted correctly.
	t.Run("user A settings persisted", func(t *testing.T) {
		filtersA, err := db.ListUserFilters(ctx, userA.ID)
		if err != nil {
			t.Fatalf("ListUserFilters(A): %v", err)
		}
		if len(filtersA) != 1 {
			t.Fatalf("user A filters = %d, want 1", len(filtersA))
		}
		if filtersA[0].Keywords != "golang" {
			t.Errorf("filter keywords = %q, want golang", filtersA[0].Keywords)
		}
		if filtersA[0].Location != "Remote" {
			t.Errorf("filter location = %q, want Remote", filtersA[0].Location)
		}
		if filtersA[0].MinSalary != 150000 {
			t.Errorf("filter min_salary = %d, want 150000", filtersA[0].MinSalary)
		}

		uA, err := db.GetUser(ctx, userA.ID)
		if err != nil {
			t.Fatalf("GetUser(A): %v", err)
		}
		if uA.ResumeMarkdown != "# Alice Resume" {
			t.Errorf("user A resume = %q, want '# Alice Resume'", uA.ResumeMarkdown)
		}
	})
}
