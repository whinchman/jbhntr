package web_test

// ban_enforcement_test.go — tests for banned-user enforcement in
// handleLoginPost, handleOAuthCallback, and requireAuth middleware.
//
// Covers:
//   - handleLoginPost: banned user with correct credentials → flash error, no session
//   - handleLoginPost: non-banned user with correct credentials → logs in normally
//   - requireAuth: user banned while session is active → session destroyed, redirect to /login
//   - handleOAuthCallback: banned user in DB after upsert → redirect to /login with flash

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/whinchman/jobhuntr/internal/models"
	"golang.org/x/crypto/bcrypt"
)

// bannedAt returns a pointer to a non-nil time.Time for use in BannedAt fields.
func bannedAt() *time.Time {
	t := time.Now().UTC()
	return &t
}

// newUserWithPassword creates a User with a bcrypt-hashed password for testing.
func newUserWithPassword(id int64, email, password string, banned bool) *models.User {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		panic("bcrypt error in test helper: " + err.Error())
	}
	hashStr := string(hash)
	u := &models.User{
		ID:                 id,
		Email:              email,
		Provider:           "email",
		PasswordHash:       &hashStr,
		OnboardingComplete: true,
	}
	if banned {
		u.BannedAt = bannedAt()
	}
	return u
}

// ─── handleLoginPost: banned user cannot log in ───────────────────────────────

// TestHandleLoginPost_BannedUser verifies that a banned user who submits
// correct credentials receives a "suspended" flash error and is NOT logged in.
func TestHandleLoginPost_BannedUser(t *testing.T) {
	bannedUser := newUserWithPassword(10, "banned@example.com", "correctpassword", true)
	us := newMockUserStore(bannedUser)
	ts := newAuthServer(t, us)
	defer ts.Close()

	resp := postFormWithCSRF(t, ts, "/login", "/login", url.Values{
		"email":    {"banned@example.com"},
		"password": {"correctpassword"},
	})
	defer resp.Body.Close()

	// Should redirect to /login (not to / or /onboarding).
	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("status = %d, want %d (SeeOther)", resp.StatusCode, http.StatusSeeOther)
	}
	if loc := resp.Header.Get("Location"); loc != "/login" {
		t.Errorf("Location = %q, want /login", loc)
	}

	// The redirect response itself should set a flash cookie; verify the next
	// GET /login shows the suspension message.
	client := ts.Client()
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}

	followReq, _ := http.NewRequest(http.MethodGet, ts.URL+"/login", nil)
	// Forward all cookies from the redirect response so the flash is consumed.
	for _, c := range resp.Cookies() {
		followReq.AddCookie(c)
	}
	followResp, err := client.Do(followReq)
	if err != nil {
		t.Fatalf("GET /login after redirect: %v", err)
	}
	defer followResp.Body.Close()

	body, _ := io.ReadAll(followResp.Body)
	if !strings.Contains(string(body), "suspended") {
		t.Errorf("expected 'suspended' flash in /login page after banned-user login attempt; body:\n%s", string(body))
	}
}

// TestHandleLoginPost_BannedUser_NoSessionCreated verifies the response does
// NOT contain a user_id session (i.e. no authenticated session cookie is issued).
func TestHandleLoginPost_BannedUser_NoSessionCreated(t *testing.T) {
	bannedUser := newUserWithPassword(11, "bannedsession@example.com", "password123", true)
	us := newMockUserStore(bannedUser)
	ts := newAuthServer(t, us)
	defer ts.Close()

	resp := postFormWithCSRF(t, ts, "/login", "/login", url.Values{
		"email":    {"bannedsession@example.com"},
		"password": {"password123"},
	})
	defer resp.Body.Close()

	// After the redirect, attempt to access a protected route using any
	// cookies returned by the login response. A protected route should still
	// redirect to /login, proving no valid session was established.
	client := ts.Client()
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/settings", nil)
	for _, c := range resp.Cookies() {
		req.AddCookie(c)
	}
	protectedResp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET /settings: %v", err)
	}
	defer protectedResp.Body.Close()

	if protectedResp.StatusCode != http.StatusSeeOther {
		t.Errorf("GET /settings status = %d, want 303 (should redirect, no session)", protectedResp.StatusCode)
	}
	if loc := protectedResp.Header.Get("Location"); loc != "/login" {
		t.Errorf("Location = %q, want /login (session must NOT be created for banned user)", loc)
	}
}

// ─── handleLoginPost: non-banned user logs in normally ────────────────────────

// TestHandleLoginPost_NonBannedUser_LogsIn verifies that a normal (non-banned)
// user with correct credentials is successfully logged in.
func TestHandleLoginPost_NonBannedUser_LogsIn(t *testing.T) {
	activeUser := newUserWithPassword(12, "active@example.com", "goodpassword", false)
	activeUser.OnboardingComplete = true
	us := newMockUserStore(activeUser)
	ts := newAuthServer(t, us)
	defer ts.Close()

	resp := postFormWithCSRF(t, ts, "/login", "/login", url.Values{
		"email":    {"active@example.com"},
		"password": {"goodpassword"},
	})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusSeeOther)
	}
	loc := resp.Header.Get("Location")
	// Should go to / (or /onboarding), not back to /login.
	if loc == "/login" {
		t.Errorf("Location = /login, expected successful login redirect (/ or /onboarding)")
	}
}

// ─── requireAuth: banned active-session user is evicted ───────────────────────

// TestRequireAuth_BannedActiveSessionUser verifies that when a user who has an
// active session is banned, their next request to a protected route results in
// their session being invalidated and a redirect to /login.
func TestRequireAuth_BannedActiveSessionUser(t *testing.T) {
	// Create user — initially not banned.
	user := &models.User{
		ID:                 20,
		Email:              "willbeban@example.com",
		Provider:           "google",
		ProviderID:         "g-ban-20",
		OnboardingComplete: true,
	}
	us := newMockUserStore(user)
	ts := newAuthServer(t, us)
	defer ts.Close()

	// Create a valid session cookie for this user.
	cookie := setSessionCookie(t, ts, 20)

	// Verify they can access a protected route while not banned.
	client := ts.Client()
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}

	req1, _ := http.NewRequest(http.MethodGet, ts.URL+"/settings", nil)
	req1.AddCookie(cookie)
	resp1, err := client.Do(req1)
	if err != nil {
		t.Fatalf("GET /settings (before ban): %v", err)
	}
	resp1.Body.Close()
	if resp1.StatusCode == http.StatusSeeOther {
		t.Logf("Note: /settings redirected before ban (may require onboarding) — skipping pre-ban check")
	}

	// Now ban the user by setting BannedAt on the in-memory store.
	bannedTime := time.Now().UTC()
	user.BannedAt = &bannedTime

	// Attempt to access the same protected route with the still-valid session cookie.
	req2, _ := http.NewRequest(http.MethodGet, ts.URL+"/settings", nil)
	req2.AddCookie(cookie)
	resp2, err := client.Do(req2)
	if err != nil {
		t.Fatalf("GET /settings (after ban): %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusSeeOther {
		t.Errorf("status after ban = %d, want %d (should redirect)", resp2.StatusCode, http.StatusSeeOther)
	}
	if loc := resp2.Header.Get("Location"); loc != "/login" {
		t.Errorf("Location = %q, want /login (banned user must be evicted)", loc)
	}

	// Verify that the session is invalidated: using the original cookie plus
	// any new cookies from the eviction response should still fail.
	req3, _ := http.NewRequest(http.MethodGet, ts.URL+"/settings", nil)
	req3.AddCookie(cookie)
	for _, c := range resp2.Cookies() {
		req3.AddCookie(c)
	}
	resp3, err := client.Do(req3)
	if err != nil {
		t.Fatalf("GET /settings (re-attempt after eviction): %v", err)
	}
	defer resp3.Body.Close()

	if resp3.StatusCode != http.StatusSeeOther {
		t.Errorf("re-attempt status = %d, want 303 (session must be destroyed)", resp3.StatusCode)
	}
	if loc := resp3.Header.Get("Location"); loc != "/login" {
		t.Errorf("re-attempt Location = %q, want /login", loc)
	}
}

// ─── requireAuth: non-banned user session still works ────────────────────────

// TestRequireAuth_ActiveUser_NotEvicted verifies that requireAuth does NOT evict
// non-banned users (regression guard).
func TestRequireAuth_ActiveUser_NotEvicted(t *testing.T) {
	user := &models.User{
		ID:                 30,
		Email:              "active2@example.com",
		Provider:           "github",
		ProviderID:         "gh-30",
		OnboardingComplete: true,
	}
	us := newMockUserStore(user)
	ts := newAuthServer(t, us)
	defer ts.Close()

	cookie := setSessionCookie(t, ts, 30)

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

	// The home page (optionalAuth) should return 200 for an active user.
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200 for non-banned active user", resp.StatusCode)
	}
}

// ─── handleOAuthCallback: banned user blocked ─────────────────────────────────

// TestBanEnforcementOAuth verifies that a banned user who completes the OAuth
// flow is redirected to /login with a "suspended" flash and does NOT receive
// an authenticated session. This test uses a real in-memory SQLite store and
// a mock OAuth provider to simulate the full callback round-trip.
func TestBanEnforcementOAuth(t *testing.T) {
	// Profile used for both the "first login" (creates user) and the "banned login".
	profile := providerProfile{
		ID:          "oauth-banned-001",
		Email:       "banned-oauth@example.com",
		DisplayName: "Banned OAuth User",
		AvatarURL:   "",
	}

	ts, db := newIntegrationServer(t, "google", profile)

	jar, _ := cookiejar.New(nil)
	client := &http.Client{
		Jar: jar,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// ── Step 1: first login — user is created in DB and initially active. ──
	state1 := extractOAuthState(t, client, ts.URL, "google")
	callbackURL1 := fmt.Sprintf("%s/auth/google/callback?code=test-code&state=%s", ts.URL, url.QueryEscape(state1))
	resp1, err := client.Get(callbackURL1)
	if err != nil {
		t.Fatalf("first callback: %v", err)
	}
	resp1.Body.Close()
	if resp1.StatusCode != http.StatusSeeOther {
		t.Fatalf("first callback: got %d, want 303", resp1.StatusCode)
	}

	// ── Step 2: ban the user via the admin store method. ──
	user, err := db.GetUserByProvider(context.Background(), "google", "oauth-banned-001")
	if err != nil {
		t.Fatalf("GetUserByProvider: %v", err)
	}
	if err := db.BanUser(context.Background(), user.ID); err != nil {
		t.Fatalf("BanUser: %v", err)
	}

	// ── Step 3: clear cookies so we start a fresh login attempt as banned user. ──
	jar2, _ := cookiejar.New(nil)
	client2 := &http.Client{
		Jar: jar2,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// ── Step 4: go through OAuth flow again — same provider profile (banned user). ──
	state2 := extractOAuthState(t, client2, ts.URL, "google")
	callbackURL2 := fmt.Sprintf("%s/auth/google/callback?code=test-code&state=%s", ts.URL, url.QueryEscape(state2))
	resp2, err := client2.Get(callbackURL2)
	if err != nil {
		t.Fatalf("banned callback: %v", err)
	}
	defer resp2.Body.Close()

	// Should redirect to /login.
	if resp2.StatusCode != http.StatusSeeOther {
		body, _ := io.ReadAll(resp2.Body)
		t.Fatalf("banned callback: got %d, want 303; body=%s", resp2.StatusCode, body)
	}
	if loc := resp2.Header.Get("Location"); loc != "/login" {
		t.Errorf("banned callback Location = %q, want /login", loc)
	}

	// ── Step 5: follow to /login and verify the suspension flash is present. ──
	loginResp, err := client2.Get(ts.URL + "/login")
	if err != nil {
		t.Fatalf("GET /login after banned OAuth callback: %v", err)
	}
	defer loginResp.Body.Close()
	body, _ := io.ReadAll(loginResp.Body)
	if !strings.Contains(string(body), "suspended") {
		t.Errorf("expected 'suspended' flash on /login after banned OAuth user; body snippet: %q", truncate(string(body), 500))
	}

	// ── Step 6: verify no authenticated session was created for the banned user. ──
	// Access a requireAuth route with client2's cookies; should still redirect to /login.
	protectedResp, err := client2.Get(ts.URL + "/settings")
	if err != nil {
		t.Fatalf("GET /settings after banned OAuth callback: %v", err)
	}
	defer protectedResp.Body.Close()
	if protectedResp.StatusCode != http.StatusSeeOther {
		t.Errorf("GET /settings: got %d, want 303 (no session for banned user)", protectedResp.StatusCode)
	}
	if loc := protectedResp.Header.Get("Location"); loc != "/login" {
		t.Errorf("GET /settings Location = %q, want /login", loc)
	}
}

// extractOAuthState starts the OAuth flow and returns the state parameter
// extracted from the provider redirect URL.
func extractOAuthState(t *testing.T, client *http.Client, baseURL, provider string) string {
	t.Helper()
	resp, err := client.Get(baseURL + "/auth/" + provider)
	if err != nil {
		t.Fatalf("GET /auth/%s: %v", provider, err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusTemporaryRedirect {
		t.Fatalf("/auth/%s: got %d, want 307", provider, resp.StatusCode)
	}
	loc := resp.Header.Get("Location")
	parsed, err := url.Parse(loc)
	if err != nil {
		t.Fatalf("parse Location %q: %v", loc, err)
	}
	state := parsed.Query().Get("state")
	if state == "" {
		t.Fatalf("/auth/%s: no state parameter in redirect URL %q", provider, loc)
	}
	return state
}

// truncate returns the first n bytes of s (for diagnostic output).
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
