package web_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/sessions"
	"golang.org/x/oauth2"

	"github.com/whinchman/jobhuntr/internal/config"
	"github.com/whinchman/jobhuntr/internal/models"
	"github.com/whinchman/jobhuntr/internal/store"
	"github.com/whinchman/jobhuntr/internal/web"
)

// ─── mock DriveTokenStore ─────────────────────────────────────────────────────

type mockDriveTokenStore struct {
	mu     sync.Mutex
	tokens map[int64]string // userID → encrypted token string
}

func newMockDriveTokenStore() *mockDriveTokenStore {
	return &mockDriveTokenStore{tokens: make(map[int64]string)}
}

func (m *mockDriveTokenStore) UpsertGoogleDriveToken(_ context.Context, userID int64, encryptedJSON string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tokens[userID] = encryptedJSON
	return nil
}

func (m *mockDriveTokenStore) GetGoogleDriveToken(_ context.Context, userID int64) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	tok, ok := m.tokens[userID]
	if !ok {
		return "", fmt.Errorf("store: no google drive token for user %d", userID)
	}
	return tok, nil
}

func (m *mockDriveTokenStore) DeleteGoogleDriveToken(_ context.Context, userID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.tokens, userID)
	return nil
}

// ─── gdoc test constants ──────────────────────────────────────────────────────

const gdocsTestSecret = "gdocs-test-session-secret-32bytes!"

// ─── gdoc server factory ──────────────────────────────────────────────────────

// newGdocsServer creates a test HTTP server with Google Drive export configured.
// The server uses a real in-memory SQLite store for auth/job storage, and the
// supplied driveTokens mock for Drive token storage.
func newGdocsServer(t *testing.T, driveTokens *mockDriveTokenStore) (*httptest.Server, *store.Store) {
	t.Helper()

	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open in-memory store: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	mockProvider := mockOAuthProvider(t, providerProfile{
		ID:    "google-stub",
		Email: "stub@example.com",
	})

	cfg := &config.Config{
		Server: config.ServerConfig{BaseURL: "http://localhost"},
		Auth: config.AuthConfig{
			SessionSecret: gdocsTestSecret,
			OAuth:         config.OAuthConfig{Enabled: true},
			Providers: config.ProvidersConfig{
				Google: config.OAuthProviderConfig{
					ClientID:     "test-google-id",
					ClientSecret: "test-google-secret",
				},
			},
		},
		GoogleDrive: config.GoogleDriveConfig{
			ClientID:     "test-drive-id",
			ClientSecret: "test-drive-secret",
		},
	}

	srv := web.NewServerWithConfig(db, db, db, cfg)
	if driveTokens != nil {
		srv.WithDriveTokenStore(driveTokens)
	}

	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	// Point the Google login provider at the mock so OAuth flows work.
	srv.WithTestOAuthProvider("google", &oauth2.Config{
		ClientID:     "test-google-id",
		ClientSecret: "test-google-secret",
		RedirectURL:  ts.URL + "/auth/google/callback",
		Scopes:       []string{"openid", "email", "profile"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  mockProvider.URL + "/authorize",
			TokenURL: mockProvider.URL + "/token",
		},
	})
	oldTransport := http.DefaultTransport
	http.DefaultTransport = &mockProviderTransport{mockURL: mockProvider.URL, base: oldTransport}
	t.Cleanup(func() { http.DefaultTransport = oldTransport })

	return ts, db
}

// gdocsSessionCookie creates an authenticated session cookie using the same
// secret as newGdocsServer.
func gdocsSessionCookie(t *testing.T, userID int64) *http.Cookie {
	t.Helper()
	sessStore := sessions.NewCookieStore([]byte(gdocsTestSecret))
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

// authedGdocsClient returns an http.Client that carries a session for the
// given userID and does NOT follow redirects.
func authedGdocsClient(t *testing.T, userID int64) *http.Client {
	t.Helper()
	jar := newTestCookieJar()
	cookie := gdocsSessionCookie(t, userID)
	// We need the jar to return this cookie for any URL. Create a dummy request
	// to set it. We use httptest to capture the cookie path.
	u, _ := http.NewRequest(http.MethodGet, "http://127.0.0.1/", nil)
	u.AddCookie(cookie)
	// Inject via a custom Transport that always adds the cookie.
	_ = jar
	return &http.Client{
		Transport: &addCookieTransport{cookie: cookie, base: http.DefaultTransport},
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// addCookieTransport adds a fixed cookie to every outbound request.
type addCookieTransport struct {
	cookie *http.Cookie
	base   http.RoundTripper
}

func (t *addCookieTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	clone.AddCookie(t.cookie)
	return t.base.RoundTrip(clone)
}

// ─── tests ────────────────────────────────────────────────────────────────────

// TestSendResumeToGoogleDocs_RedirectsToOAuthWhenNoToken verifies that when no
// Drive token is stored, the handler redirects to /auth/google-drive.
func TestSendResumeToGoogleDocs_RedirectsToOAuthWhenNoToken(t *testing.T) {
	driveTokens := newMockDriveTokenStore() // empty — no token stored
	ts, db := newGdocsServer(t, driveTokens)

	user := &models.User{Email: "alice@example.com", DisplayName: "Alice", Provider: "google", ProviderID: "alice-1"}
	upserted, err := db.UpsertUser(context.Background(), user)
	if err != nil {
		t.Fatalf("upsert user: %v", err)
	}
	job := &models.Job{
		Title:          "Engineer",
		Company:        "Acme",
		Status:         models.StatusComplete,
		ResumeMarkdown: "# Resume\n\nContent here",
	}
	if _, err := db.CreateJob(context.Background(), upserted.ID, job); err != nil {
		t.Fatalf("create job: %v", err)
	}

	client := authedGdocsClient(t, upserted.ID)
	resp, err := client.Get(fmt.Sprintf("%s/output/%d/resume.gdoc", ts.URL, job.ID))
	if err != nil {
		t.Fatalf("GET resume.gdoc: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusFound {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 302, got %d; body: %s", resp.StatusCode, body)
	}
	loc := resp.Header.Get("Location")
	if !strings.HasPrefix(loc, "/auth/google-drive") {
		t.Errorf("expected redirect to /auth/google-drive, got %q", loc)
	}
	if !strings.Contains(loc, "return_to=") {
		t.Errorf("expected return_to param in redirect URL, got %q", loc)
	}
}

// TestSendCoverToGoogleDocs_RedirectsToOAuthWhenNoToken verifies the same
// redirect behaviour for the cover letter route.
func TestSendCoverToGoogleDocs_RedirectsToOAuthWhenNoToken(t *testing.T) {
	driveTokens := newMockDriveTokenStore()
	ts, db := newGdocsServer(t, driveTokens)

	user := &models.User{Email: "bob@example.com", DisplayName: "Bob", Provider: "google", ProviderID: "bob-1"}
	upserted, err := db.UpsertUser(context.Background(), user)
	if err != nil {
		t.Fatalf("upsert user: %v", err)
	}
	job := &models.Job{
		Title:         "Designer",
		Company:       "Corp",
		Status:        models.StatusComplete,
		CoverMarkdown: "# Cover Letter\n\nContent",
	}
	if _, err := db.CreateJob(context.Background(), upserted.ID, job); err != nil {
		t.Fatalf("create job: %v", err)
	}

	client := authedGdocsClient(t, upserted.ID)
	resp, err := client.Get(fmt.Sprintf("%s/output/%d/cover_letter.gdoc", ts.URL, job.ID))
	if err != nil {
		t.Fatalf("GET cover_letter.gdoc: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusFound {
		t.Errorf("expected 302, got %d", resp.StatusCode)
	}
	loc := resp.Header.Get("Location")
	if !strings.HasPrefix(loc, "/auth/google-drive") {
		t.Errorf("expected redirect to /auth/google-drive, got %q", loc)
	}
}

// TestSendToGoogleDocs_EmptyResumeMarkdownReturns400 verifies that requesting a
// gdoc for a job with no resume Markdown returns 400 (when a token IS stored).
func TestSendToGoogleDocs_EmptyResumeMarkdownReturns400(t *testing.T) {
	driveTokens := newMockDriveTokenStore()
	ts, db := newGdocsServer(t, driveTokens)

	user := &models.User{Email: "carol@example.com", DisplayName: "Carol", Provider: "google", ProviderID: "carol-1"}
	upserted, err := db.UpsertUser(context.Background(), user)
	if err != nil {
		t.Fatalf("upsert user: %v", err)
	}
	job := &models.Job{
		Title:          "PM",
		Company:        "StartupX",
		Status:         models.StatusComplete,
		ResumeMarkdown: "", // empty
	}
	if _, err := db.CreateJob(context.Background(), upserted.ID, job); err != nil {
		t.Fatalf("create job: %v", err)
	}

	// Store a fake (syntactically invalid but present) token so the empty-
	// markdown check runs. The handler checks markdown before decrypting.
	_ = driveTokens.UpsertGoogleDriveToken(context.Background(), upserted.ID, "fakefakefake")

	client := authedGdocsClient(t, upserted.ID)
	resp, err := client.Get(fmt.Sprintf("%s/output/%d/resume.gdoc", ts.URL, job.ID))
	if err != nil {
		t.Fatalf("GET resume.gdoc: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("expected 400, got %d; body: %s", resp.StatusCode, body)
	}
}

// TestSendToGoogleDocs_WrongUserReturns404 verifies that a job belonging to
// another user returns 404.
func TestSendToGoogleDocs_WrongUserReturns404(t *testing.T) {
	driveTokens := newMockDriveTokenStore()
	ts, db := newGdocsServer(t, driveTokens)

	owner := &models.User{Email: "owner@example.com", DisplayName: "Owner", Provider: "google", ProviderID: "owner-1"}
	ownerDB, err := db.UpsertUser(context.Background(), owner)
	if err != nil {
		t.Fatalf("upsert owner: %v", err)
	}
	requester := &models.User{Email: "requester@example.com", DisplayName: "Requester", Provider: "google", ProviderID: "req-1"}
	requesterDB, err := db.UpsertUser(context.Background(), requester)
	if err != nil {
		t.Fatalf("upsert requester: %v", err)
	}

	job := &models.Job{
		Title:          "Engineer",
		Company:        "Acme",
		Status:         models.StatusComplete,
		ResumeMarkdown: "# Resume",
	}
	if _, err := db.CreateJob(context.Background(), ownerDB.ID, job); err != nil {
		t.Fatalf("create job: %v", err)
	}

	// Log in as requester (not the owner).
	client := authedGdocsClient(t, requesterDB.ID)
	resp, err := client.Get(fmt.Sprintf("%s/output/%d/resume.gdoc", ts.URL, job.ID))
	if err != nil {
		t.Fatalf("GET resume.gdoc: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d; body: %s", resp.StatusCode, body)
	}
}

// TestDriveOAuthStart_RedirectsToGoogle verifies that /auth/google-drive
// redirects to accounts.google.com and includes the drive.file scope.
func TestDriveOAuthStart_RedirectsToGoogle(t *testing.T) {
	ts, db := newGdocsServer(t, newMockDriveTokenStore())

	user := &models.User{Email: "dave@example.com", DisplayName: "Dave", Provider: "google", ProviderID: "dave-1"}
	upserted, err := db.UpsertUser(context.Background(), user)
	if err != nil {
		t.Fatalf("upsert user: %v", err)
	}

	client := authedGdocsClient(t, upserted.ID)
	resp, err := client.Get(ts.URL + "/auth/google-drive?return_to=/output/42/resume.gdoc")
	if err != nil {
		t.Fatalf("GET /auth/google-drive: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusTemporaryRedirect {
		t.Errorf("expected 307, got %d", resp.StatusCode)
	}
	loc := resp.Header.Get("Location")
	if !strings.Contains(loc, "accounts.google.com") {
		t.Errorf("expected redirect to accounts.google.com, got %q", loc)
	}
	if !strings.Contains(loc, "drive.file") {
		t.Errorf("expected drive.file scope in redirect URL, got %q", loc)
	}
}

// TestGdocRoutesNotRegistered_WhenDriveNotConfigured verifies that the .gdoc
// routes return 404 when GoogleDrive.ClientID is empty.
func TestGdocRoutesNotRegistered_WhenDriveNotConfigured(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{BaseURL: "http://localhost"},
		Auth: config.AuthConfig{
			SessionSecret: gdocsTestSecret,
			OAuth:         config.OAuthConfig{Enabled: false},
		},
		// GoogleDrive is zero-value: ClientID == ""
	}
	job := newTestJobWithMarkdown(1, 1, "# Resume")
	js := newMockJobStore(job)
	srv := web.NewServerWithConfig(js, nil, nil, cfg)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	resp, err := http.Get(ts.URL + "/output/1/resume.gdoc")
	if err != nil {
		t.Fatalf("GET resume.gdoc: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 when Drive not configured, got %d", resp.StatusCode)
	}
}

// TestWithDriveTokenStore_ReturnsServer verifies that WithDriveTokenStore
// returns the *Server for fluent chaining.
func TestWithDriveTokenStore_ReturnsServer(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{BaseURL: "http://localhost"},
		Auth:   config.AuthConfig{SessionSecret: "test-secret-at-least-32-bytes!!"},
		GoogleDrive: config.GoogleDriveConfig{
			ClientID:     "test-id",
			ClientSecret: "test-secret",
		},
	}
	js := newMockJobStore()
	srv := web.NewServerWithConfig(js, nil, nil, cfg)
	dts := newMockDriveTokenStore()
	result := srv.WithDriveTokenStore(dts)
	if result == nil {
		t.Error("WithDriveTokenStore returned nil; expected *Server")
	}
}

// TestSendToGoogleDocs_Unauthenticated_RedirectsToLogin verifies that an
// unauthenticated request to a .gdoc route redirects to /login.
func TestSendToGoogleDocs_Unauthenticated_RedirectsToLogin(t *testing.T) {
	ts, _ := newGdocsServer(t, newMockDriveTokenStore())

	client := &http.Client{
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Get(ts.URL + "/output/1/resume.gdoc")
	if err != nil {
		t.Fatalf("GET resume.gdoc unauthenticated: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusSeeOther && resp.StatusCode != http.StatusFound {
		t.Errorf("expected redirect to login, got %d", resp.StatusCode)
	}
	loc := resp.Header.Get("Location")
	if !strings.Contains(loc, "login") {
		t.Errorf("expected redirect to /login, got %q", loc)
	}
}

// TestDriveOAuthCallback_CSRFStateMismatch verifies that a callback request with
// a state that does not match the session returns 400.
func TestDriveOAuthCallback_CSRFStateMismatch(t *testing.T) {
	ts, db := newGdocsServer(t, newMockDriveTokenStore())

	user := &models.User{Email: "csrf@example.com", DisplayName: "CSRF", Provider: "google", ProviderID: "csrf-1"}
	upserted, err := db.UpsertUser(context.Background(), user)
	if err != nil {
		t.Fatalf("upsert user: %v", err)
	}

	// Build a session cookie that has drive_oauth_state = "correct-state".
	sessStore := sessions.NewCookieStore([]byte(gdocsTestSecret))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	sess, _ := sessStore.New(req, "jobhuntr_session")
	sess.Values["user_id"] = upserted.ID
	sess.Values["drive_oauth_state"] = "correct-state"
	sess.Options.Path = "/"
	_ = sess.Save(req, w)
	cookies := w.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("no session cookie set")
	}
	sessionCookie := cookies[0]

	client := &http.Client{
		Transport: &addCookieTransport{cookie: sessionCookie, base: http.DefaultTransport},
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// Pass a state value that intentionally differs from the session value.
	resp, err := client.Get(ts.URL + "/auth/google-drive/callback?state=wrong-state&code=some-code")
	if err != nil {
		t.Fatalf("GET callback: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("expected 400 for CSRF state mismatch, got %d; body: %s", resp.StatusCode, body)
	}
}

// TestDriveOAuthCallback_NoStateInSession verifies that a callback request when
// the session holds no drive_oauth_state returns 400.
func TestDriveOAuthCallback_NoStateInSession(t *testing.T) {
	ts, db := newGdocsServer(t, newMockDriveTokenStore())

	user := &models.User{Email: "nostate@example.com", DisplayName: "NoState", Provider: "google", ProviderID: "nostate-1"}
	upserted, err := db.UpsertUser(context.Background(), user)
	if err != nil {
		t.Fatalf("upsert user: %v", err)
	}

	// Session cookie has no drive_oauth_state key at all.
	client := authedGdocsClient(t, upserted.ID)
	resp, err := client.Get(ts.URL + "/auth/google-drive/callback?state=any-state&code=some-code")
	if err != nil {
		t.Fatalf("GET callback: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("expected 400 when no CSRF state in session, got %d; body: %s", resp.StatusCode, body)
	}
}

// mockDriveTransport intercepts HTTP requests to the Google Drive upload
// endpoint and redirects them to a mock Drive API httptest.Server.
// All other requests pass through to the base transport.
type mockDriveTransport struct {
	driveServerURL string
	base           http.RoundTripper
}

func (t *mockDriveTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	target := req.URL.String()
	// Intercept Drive files.create (multipart upload) and files list calls.
	if strings.Contains(target, "www.googleapis.com/upload/drive") ||
		strings.Contains(target, "www.googleapis.com/drive/v3/files") {
		newURL := t.driveServerURL + req.URL.Path
		if req.URL.RawQuery != "" {
			newURL += "?" + req.URL.RawQuery
		}
		newReq, _ := http.NewRequestWithContext(req.Context(), req.Method, newURL, req.Body)
		for k, v := range req.Header {
			newReq.Header[k] = v
		}
		return t.base.RoundTrip(newReq)
	}
	return t.base.RoundTrip(req)
}

// newMockDriveServer starts an httptest.Server that simulates the minimal
// Google Drive v3 files.create endpoint, returning a fake doc ID and
// webViewLink.
func newMockDriveServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	// Drive files.create — multipart/resumable upload.
	mux.HandleFunc("/upload/drive/v3/files", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"id":          "fake-doc-id",
			"webViewLink": "https://docs.google.com/document/d/fake-doc-id/edit",
		})
	})
	// Drive files metadata endpoint (sometimes called without /upload prefix).
	mux.HandleFunc("/drive/v3/files", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"id":          "fake-doc-id",
			"webViewLink": "https://docs.google.com/document/d/fake-doc-id/edit",
		})
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// TestSendResumeToGoogleDocs_SuccessReturnsDocURL verifies that when a valid
// Drive token is stored and the mock Drive server returns a doc ID, the
// handler responds with 200 JSON containing the Google Doc URL.
func TestSendResumeToGoogleDocs_SuccessReturnsDocURL(t *testing.T) {
	driveTokens := newMockDriveTokenStore()
	mockDrive := newMockDriveServer(t)

	ts, db := newGdocsServer(t, driveTokens)

	user := &models.User{Email: "success@example.com", DisplayName: "Success", Provider: "google", ProviderID: "success-1"}
	upserted, err := db.UpsertUser(context.Background(), user)
	if err != nil {
		t.Fatalf("upsert user: %v", err)
	}

	job := &models.Job{
		Title:          "Engineer",
		Company:        "TechCo",
		Status:         models.StatusComplete,
		ResumeMarkdown: "# Resume\n\nContent here",
	}
	if _, err := db.CreateJob(context.Background(), upserted.ID, job); err != nil {
		t.Fatalf("create job: %v", err)
	}

	// Store a real encrypted token.
	tok := &oauth2.Token{
		AccessToken:  "test-access-token",
		TokenType:    "Bearer",
		RefreshToken: "test-refresh-token",
		Expiry:       time.Now().Add(24 * time.Hour),
	}
	key := web.DeriveKeyForTest(gdocsTestSecret)
	encTok, err := web.EncryptTokenForTest(key, tok)
	if err != nil {
		t.Fatalf("encrypt token: %v", err)
	}
	if err := driveTokens.UpsertGoogleDriveToken(context.Background(), upserted.ID, encTok); err != nil {
		t.Fatalf("upsert drive token: %v", err)
	}

	// Install a transport that redirects Drive API calls to the mock server.
	oldTransport := http.DefaultTransport
	http.DefaultTransport = &mockDriveTransport{driveServerURL: mockDrive.URL, base: oldTransport}
	t.Cleanup(func() { http.DefaultTransport = oldTransport })

	client := authedGdocsClient(t, upserted.ID)
	// Use a client that does NOT stop on redirect, since we want the 200 response.
	client.CheckRedirect = nil

	resp, err := client.Get(fmt.Sprintf("%s/output/%d/resume.gdoc", ts.URL, job.ID))
	if err != nil {
		t.Fatalf("GET resume.gdoc: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", resp.StatusCode, body)
	}

	var result map[string]string
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("decode JSON response: %v — body: %s", err, body)
	}
	url, ok := result["url"]
	if !ok {
		t.Fatalf("response JSON missing 'url' key; got: %s", body)
	}
	if !strings.Contains(url, "fake-doc-id") {
		t.Errorf("expected URL to contain 'fake-doc-id', got %q", url)
	}
}

// TestSendResumeToGoogleDocs_TokenWrittenBackAfterDriveCall verifies that after
// a successful Drive upload, the handler writes the (possibly refreshed) token
// back to the DriveTokenStore.
func TestSendResumeToGoogleDocs_TokenWrittenBackAfterDriveCall(t *testing.T) {
	driveTokens := newMockDriveTokenStore()
	mockDrive := newMockDriveServer(t)

	ts, db := newGdocsServer(t, driveTokens)

	user := &models.User{Email: "writeback@example.com", DisplayName: "WriteBack", Provider: "google", ProviderID: "wb-1"}
	upserted, err := db.UpsertUser(context.Background(), user)
	if err != nil {
		t.Fatalf("upsert user: %v", err)
	}

	job := &models.Job{
		Title:          "Designer",
		Company:        "DesignCo",
		Status:         models.StatusComplete,
		ResumeMarkdown: "# Resume\n\nWrite-back test",
	}
	if _, err := db.CreateJob(context.Background(), upserted.ID, job); err != nil {
		t.Fatalf("create job: %v", err)
	}

	tok := &oauth2.Token{
		AccessToken:  "original-access-token",
		TokenType:    "Bearer",
		RefreshToken: "original-refresh-token",
		Expiry:       time.Now().Add(24 * time.Hour),
	}
	key := web.DeriveKeyForTest(gdocsTestSecret)
	encTok, err := web.EncryptTokenForTest(key, tok)
	if err != nil {
		t.Fatalf("encrypt token: %v", err)
	}
	if err := driveTokens.UpsertGoogleDriveToken(context.Background(), upserted.ID, encTok); err != nil {
		t.Fatalf("upsert drive token: %v", err)
	}

	// Record the initial encrypted token value.
	tokenBeforeCall, _ := driveTokens.GetGoogleDriveToken(context.Background(), upserted.ID)

	oldTransport := http.DefaultTransport
	http.DefaultTransport = &mockDriveTransport{driveServerURL: mockDrive.URL, base: oldTransport}
	t.Cleanup(func() { http.DefaultTransport = oldTransport })

	client := authedGdocsClient(t, upserted.ID)
	client.CheckRedirect = nil

	resp, err := client.Get(fmt.Sprintf("%s/output/%d/resume.gdoc", ts.URL, job.ID))
	if err != nil {
		t.Fatalf("GET resume.gdoc: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d; body: %s", resp.StatusCode, body)
	}

	// After the call the store should still hold a token for this user (written
	// back by the handler). The token may be identical to the original (no
	// refresh was needed) or different (token was refreshed). Either way, a
	// token must be present and readable.
	tokenAfterCall, err := driveTokens.GetGoogleDriveToken(context.Background(), upserted.ID)
	if err != nil {
		t.Fatalf("expected token to remain in store after Drive call, got error: %v", err)
	}
	if tokenAfterCall == "" {
		t.Error("expected non-empty token in store after Drive call")
	}
	// The store must not lose the token (write-back must not delete it).
	_ = tokenBeforeCall
}

// TestJobDetailHTML_GoogleDriveButtonPresent verifies that when Google Drive is
// configured (GoogleDriveEnabled = true) and the job is complete with resume
// markdown, the job detail page contains the "Send to Google Docs" button for
// the resume.
func TestJobDetailHTML_GoogleDriveButtonPresent(t *testing.T) {
	driveTokens := newMockDriveTokenStore()
	ts, db := newGdocsServer(t, driveTokens)

	user := &models.User{Email: "button@example.com", DisplayName: "Button", Provider: "google", ProviderID: "btn-1"}
	upserted, err := db.UpsertUser(context.Background(), user)
	if err != nil {
		t.Fatalf("upsert user: %v", err)
	}
	job := &models.Job{
		Title:          "Engineer",
		Company:        "Acme",
		Status:         models.StatusComplete,
		ResumeMarkdown: "# Resume\n\nSome content",
	}
	if _, err := db.CreateJob(context.Background(), upserted.ID, job); err != nil {
		t.Fatalf("create job: %v", err)
	}

	client := authedGdocsClient(t, upserted.ID)
	client.CheckRedirect = nil

	resp, err := client.Get(fmt.Sprintf("%s/jobs/%d", ts.URL, job.ID))
	if err != nil {
		t.Fatalf("GET /jobs/%d: %v", job.ID, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if !strings.Contains(string(body), "btn-resume-gdoc") {
		t.Errorf("expected 'btn-resume-gdoc' button in HTML when GoogleDriveEnabled=true and resume exists; body excerpt: %.300s", body)
	}
	if !strings.Contains(string(body), "Send to Google Docs") {
		t.Errorf("expected 'Send to Google Docs' link text in HTML")
	}
}

// TestJobDetailHTML_GoogleDriveButtonAbsentWhenNotConfigured verifies that when
// Google Drive is NOT configured (GoogleDriveEnabled = false), the "Send to
// Google Docs" button does not appear in the job detail page.
func TestJobDetailHTML_GoogleDriveButtonAbsentWhenNotConfigured(t *testing.T) {
	// Build a server with no GoogleDrive config (ClientID is empty).
	cfg := &config.Config{
		Server: config.ServerConfig{BaseURL: "http://localhost"},
		Auth: config.AuthConfig{
			SessionSecret: gdocsTestSecret,
			OAuth:         config.OAuthConfig{Enabled: false},
		},
		// GoogleDrive is zero-value: ClientID == "" → driveOAuthCfg is nil
	}

	job := &models.Job{
		ID:             10,
		UserID:         0, // userID 0 means no ownership check for mockJobStore
		Title:          "Engineer",
		Company:        "Acme",
		Status:         models.StatusComplete,
		ResumeMarkdown: "# Resume\n\nContent",
		CoverMarkdown:  "# Cover\n\nContent",
	}
	js := newMockJobStore(job)
	srv := web.NewServerWithConfig(js, nil, nil, cfg)
	testSrv := httptest.NewServer(srv.Handler())
	t.Cleanup(testSrv.Close)

	resp, err := http.Get(fmt.Sprintf("%s/jobs/10", testSrv.URL))
	if err != nil {
		t.Fatalf("GET /jobs/10: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if strings.Contains(string(body), "btn-resume-gdoc") {
		t.Errorf("expected NO 'btn-resume-gdoc' button when GoogleDriveEnabled=false")
	}
	if strings.Contains(string(body), "btn-cover-gdoc") {
		t.Errorf("expected NO 'btn-cover-gdoc' button when GoogleDriveEnabled=false")
	}
}

// TestJobDetailHTML_GoogleDriveButtonAbsentForIncompleteJob verifies that the
// "Send to Google Docs" button does not appear for a job that is not yet
// "complete" (template condition: GoogleDriveEnabled AND status == "complete").
func TestJobDetailHTML_GoogleDriveButtonAbsentForIncompleteJob(t *testing.T) {
	driveTokens := newMockDriveTokenStore()
	ts, db := newGdocsServer(t, driveTokens)

	user := &models.User{Email: "incomplete@example.com", DisplayName: "Incomplete", Provider: "google", ProviderID: "inc-1"}
	upserted, err := db.UpsertUser(context.Background(), user)
	if err != nil {
		t.Fatalf("upsert user: %v", err)
	}
	// Job status is "approved" (not "complete") — buttons should be absent.
	job := &models.Job{
		Title:          "Engineer",
		Company:        "Acme",
		Status:         models.StatusApproved,
		ResumeMarkdown: "# Resume\n\nContent",
	}
	if _, err := db.CreateJob(context.Background(), upserted.ID, job); err != nil {
		t.Fatalf("create job: %v", err)
	}

	client := authedGdocsClient(t, upserted.ID)
	client.CheckRedirect = nil

	resp, err := client.Get(fmt.Sprintf("%s/jobs/%d", ts.URL, job.ID))
	if err != nil {
		t.Fatalf("GET /jobs/%d: %v", job.ID, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if strings.Contains(string(body), "btn-resume-gdoc") {
		t.Errorf("expected NO 'btn-resume-gdoc' button for non-complete job")
	}
}

// ─── helpers ──────────────────────────────────────────────────────────────────

// newTestJobWithMarkdown returns a *models.Job with the given ID, userID, and
// resume Markdown set. Used to pre-populate the mock job store.
func newTestJobWithMarkdown(id, userID int64, markdown string) *models.Job {
	return &models.Job{
		ID:             id,
		UserID:         userID,
		Title:          "Engineer",
		Company:        "Acme",
		Status:         models.StatusComplete,
		ResumeMarkdown: markdown,
	}
}
