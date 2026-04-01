package web

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"

	"github.com/whinchman/jobhuntr/internal/config"
	"github.com/whinchman/jobhuntr/internal/models"
)

// contextKey is an unexported type for context keys in this package,
// preventing collisions with keys from other packages.
type contextKey string

// userContextKey is the context key for the authenticated *models.User.
const userContextKey contextKey = "user"

// UserFromContext extracts the authenticated user from the request context.
// Returns nil if no user is present (should not happen behind requireAuth).
func UserFromContext(ctx context.Context) *models.User {
	u, _ := ctx.Value(userContextKey).(*models.User)
	return u
}

const (
	sessionName      = "jobhuntr_session"
	sessionUserID    = "user_id"
	sessionMaxAge    = 30 * 24 * 60 * 60 // 30 days in seconds
	oauthStateName   = "oauth_state"
	sessionFlashKey  = "flash"
	sessionReturnToKey = "return_to"
)

// oauthProviders builds oauth2.Config for each enabled provider.
// baseURL is the server's externally reachable base URL (e.g. "http://localhost:8080").
func oauthProviders(authCfg config.AuthConfig, baseURL string) map[string]*oauth2.Config {
	providers := make(map[string]*oauth2.Config)

	if authCfg.Providers.Google.ClientID != "" {
		providers["google"] = &oauth2.Config{
			ClientID:     authCfg.Providers.Google.ClientID,
			ClientSecret: authCfg.Providers.Google.ClientSecret,
			RedirectURL:  strings.TrimRight(baseURL, "/") + "/auth/google/callback",
			Scopes:       []string{"openid", "email", "profile"},
			Endpoint: oauth2.Endpoint{
				AuthURL:  "https://accounts.google.com/o/oauth2/v2/auth",
				TokenURL: "https://oauth2.googleapis.com/token",
			},
		}
	}

	if authCfg.Providers.GitHub.ClientID != "" {
		providers["github"] = &oauth2.Config{
			ClientID:     authCfg.Providers.GitHub.ClientID,
			ClientSecret: authCfg.Providers.GitHub.ClientSecret,
			RedirectURL:  strings.TrimRight(baseURL, "/") + "/auth/github/callback",
			Scopes:       []string{"user:email"},
			Endpoint:     github.Endpoint,
		}
	}

	return providers
}

// getUserFromSession loads the user_id from the session cookie and fetches
// the full User from the store. Returns (nil, false) if no valid session or
// if session support is not configured.
func (s *Server) getUserFromSession(r *http.Request) (*models.User, bool) {
	if s.sessionStore == nil {
		return nil, false
	}
	sess, err := s.sessionStore.Get(r, sessionName)
	if err != nil {
		return nil, false
	}
	rawID, ok := sess.Values[sessionUserID]
	if !ok {
		return nil, false
	}
	userID, ok := rawID.(int64)
	if !ok {
		return nil, false
	}
	user, err := s.userStore.GetUser(r.Context(), userID)
	if err != nil {
		slog.Warn("session references missing user", "user_id", userID, "error", err)
		return nil, false
	}
	return user, true
}

// setSession creates or updates the session cookie with the given user's ID.
func (s *Server) setSession(w http.ResponseWriter, r *http.Request, user *models.User) error {
	sess, err := s.sessionStore.Get(r, sessionName)
	if err != nil {
		// Corrupted cookie — create a new session.
		sess, _ = s.sessionStore.New(r, sessionName)
	}
	sess.Values[sessionUserID] = user.ID
	sess.Options.MaxAge = sessionMaxAge
	sess.Options.HttpOnly = true
	sess.Options.SameSite = http.SameSiteLaxMode
	sess.Options.Secure = strings.HasPrefix(s.baseURL, "https")
	sess.Options.Path = "/"
	return sess.Save(r, w)
}

// clearSession removes the session cookie.
func (s *Server) clearSession(w http.ResponseWriter, r *http.Request) {
	sess, err := s.sessionStore.Get(r, sessionName)
	if err != nil {
		return
	}
	sess.Options.MaxAge = -1
	sess.Options.Path = "/"
	_ = sess.Save(r, w)
}

// setFlash stores a one-shot flash message in the session. The message is
// consumed and cleared on the next call to consumeFlash.
func (s *Server) setFlash(w http.ResponseWriter, r *http.Request, message string) {
	sess, err := s.sessionStore.Get(r, sessionName)
	if err != nil {
		sess, _ = s.sessionStore.New(r, sessionName)
	}
	sess.Values[sessionFlashKey] = message
	sess.Options.Path = "/"
	if err := sess.Save(r, w); err != nil {
		slog.Warn("setFlash: failed to save session", "error", err)
	}
}

// consumeFlash reads and clears the flash message from the session (consume-once).
// Returns "" if no flash is set.
func (s *Server) consumeFlash(w http.ResponseWriter, r *http.Request) string {
	if s.sessionStore == nil {
		return ""
	}
	sess, err := s.sessionStore.Get(r, sessionName)
	if err != nil {
		return ""
	}
	raw, ok := sess.Values[sessionFlashKey]
	if !ok {
		return ""
	}
	msg, _ := raw.(string)
	delete(sess.Values, sessionFlashKey)
	sess.Options.Path = "/"
	if err := sess.Save(r, w); err != nil {
		slog.Warn("consumeFlash: failed to save session", "error", err)
	}
	return msg
}

// consumeReturnTo reads and clears the "return_to" URL from the session.
// It validates that the stored value is a safe same-origin path: it must start
// with "/" and must not start with "//" or contain "://" (which would indicate
// an absolute URL and create an open-redirect vulnerability).
// Returns "/" if no value is set or if validation fails.
func (s *Server) consumeReturnTo(w http.ResponseWriter, r *http.Request) string {
	if s.sessionStore == nil {
		return "/"
	}
	sess, err := s.sessionStore.Get(r, sessionName)
	if err != nil {
		return "/"
	}
	raw, ok := sess.Values[sessionReturnToKey]
	if !ok {
		return "/"
	}
	dest, _ := raw.(string)
	delete(sess.Values, sessionReturnToKey)
	sess.Options.Path = "/"
	if err := sess.Save(r, w); err != nil {
		slog.Warn("consumeReturnTo: failed to save session", "error", err)
	}

	// Validate: must start with "/" and must not be a protocol-relative URL
	// (starts with "//") or contain a URL scheme ("://").
	if dest == "" || !strings.HasPrefix(dest, "/") || strings.HasPrefix(dest, "//") || strings.Contains(dest, "://") {
		return "/"
	}
	return dest
}

// generateState returns a random base64-encoded string for OAuth state parameter.
func generateState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate oauth state: %w", err)
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// loginData is the template data for the login page.
type loginData struct {
	// Providers is the list of OAuth provider names that are configured and
	// available for login (e.g. "google", "github").
	Providers []string
	// Flash is a one-shot error or info message to display. Empty means no alert.
	Flash string
}

// handleLogin renders the login page with OAuth provider buttons.
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	// If already logged in, redirect to the stored return_to destination (or /).
	if _, ok := s.getUserFromSession(r); ok {
		http.Redirect(w, r, s.consumeReturnTo(w, r), http.StatusSeeOther)
		return
	}

	// Collect the names of configured OAuth providers so the template can
	// render only the buttons that will actually work.
	var providers []string
	for name := range s.oauthProviders {
		providers = append(providers, name)
	}

	// Consume any one-shot flash message (e.g. from a failed OAuth callback).
	flash := s.consumeFlash(w, r)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.loginTmpl.ExecuteTemplate(w, "login.html", loginData{Providers: providers, Flash: flash}); err != nil {
		slog.Error("login template render error", "error", err)
	}
}

// handleOAuthStart initiates the OAuth flow by redirecting to the provider's
// authorization URL. The provider name comes from the URL path parameter.
func (s *Server) handleOAuthStart(w http.ResponseWriter, r *http.Request) {
	providerName := chi.URLParam(r, "provider")
	oauthCfg, ok := s.oauthProviders[providerName]
	if !ok {
		http.Error(w, "unknown provider", http.StatusBadRequest)
		return
	}

	state, err := generateState()
	if err != nil {
		slog.Error("failed to generate oauth state", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Store state in session for CSRF verification in the callback.
	sess, _ := s.sessionStore.Get(r, sessionName)
	sess.Values[oauthStateName] = state
	sess.Options.Path = "/"
	if err := sess.Save(r, w); err != nil {
		slog.Error("failed to save session state", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	url := oauthCfg.AuthCodeURL(state)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

// handleOAuthCallback handles the OAuth provider redirect. It exchanges the
// authorization code for a token, fetches user info from the provider,
// upserts the user in the database, creates a session, and redirects to /.
func (s *Server) handleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	providerName := chi.URLParam(r, "provider")
	oauthCfg, ok := s.oauthProviders[providerName]
	if !ok {
		http.Error(w, "unknown provider", http.StatusBadRequest)
		return
	}

	// Verify state matches what we stored in the session.
	sess, err := s.sessionStore.Get(r, sessionName)
	if err != nil {
		slog.Warn("callback: invalid session", "error", err)
		s.setFlash(w, r, "Sign-in session expired. Please try again.")
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	expectedState, _ := sess.Values[oauthStateName].(string)
	if expectedState == "" || r.URL.Query().Get("state") != expectedState {
		slog.Warn("callback: state mismatch")
		s.setFlash(w, r, "Sign-in request was invalid or expired. Please try again.")
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	delete(sess.Values, oauthStateName)

	// Check for error from provider (e.g. user denied consent).
	if errMsg := r.URL.Query().Get("error"); errMsg != "" {
		slog.Warn("oauth error from provider", "provider", providerName, "error", errMsg)
		flashMsg := "Sign-in was cancelled or denied. Please try again."
		if errDesc := r.URL.Query().Get("error_description"); errDesc != "" {
			flashMsg = errDesc
		}
		s.setFlash(w, r, flashMsg)
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Exchange authorization code for token.
	code := r.URL.Query().Get("code")
	token, err := oauthCfg.Exchange(r.Context(), code)
	if err != nil {
		slog.Error("oauth code exchange failed", "provider", providerName, "error", err)
		s.setFlash(w, r, "Sign-in failed: could not complete authentication. Please try again.")
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Fetch user info from provider.
	user, err := s.fetchProviderUser(r.Context(), providerName, oauthCfg, token)
	if err != nil {
		slog.Error("failed to fetch provider user info", "provider", providerName, "error", err)
		s.setFlash(w, r, "Sign-in failed: could not retrieve your profile. Please try again.")
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Upsert user in database.
	dbUser, err := s.userStore.UpsertUser(r.Context(), user)
	if err != nil {
		slog.Error("failed to upsert user", "provider", providerName, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Create session.
	if err := s.setSession(w, r, dbUser); err != nil {
		slog.Error("failed to set session", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// For new users, redirect to /onboarding to collect their display name
	// and optional resume. The return_to session value is preserved so the
	// onboarding POST handler can redirect to the original destination.
	// For returning users, consume return_to and redirect normally.
	if !dbUser.OnboardingComplete {
		http.Redirect(w, r, "/onboarding", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, s.consumeReturnTo(w, r), http.StatusSeeOther)
}

// fetchProviderUser calls the provider's user-info API and returns a
// partially-populated models.User (without ID or timestamps).
func (s *Server) fetchProviderUser(ctx context.Context, provider string, cfg *oauth2.Config, token *oauth2.Token) (*models.User, error) {
	switch provider {
	case "google":
		return fetchGoogleUser(ctx, cfg, token)
	case "github":
		return fetchGitHubUser(ctx, cfg, token)
	default:
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}
}

// fetchGoogleUser retrieves the authenticated user's profile from Google.
func fetchGoogleUser(ctx context.Context, cfg *oauth2.Config, token *oauth2.Token) (*models.User, error) {
	client := cfg.Client(ctx, token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		return nil, fmt.Errorf("google userinfo request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("google userinfo: status %d", resp.StatusCode)
	}

	var info struct {
		ID      string `json:"id"`
		Email   string `json:"email"`
		Name    string `json:"name"`
		Picture string `json:"picture"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("google userinfo decode: %w", err)
	}

	return &models.User{
		Provider:    "google",
		ProviderID:  info.ID,
		Email:       info.Email,
		DisplayName: info.Name,
		AvatarURL:   info.Picture,
	}, nil
}

// fetchGitHubUser retrieves the authenticated user's profile from GitHub.
func fetchGitHubUser(ctx context.Context, cfg *oauth2.Config, token *oauth2.Token) (*models.User, error) {
	client := cfg.Client(ctx, token)

	// Get user profile.
	resp, err := client.Get("https://api.github.com/user")
	if err != nil {
		return nil, fmt.Errorf("github user request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github user: status %d", resp.StatusCode)
	}

	var profile struct {
		ID        int64  `json:"id"`
		Login     string `json:"login"`
		Name      string `json:"name"`
		Email     string `json:"email"`
		AvatarURL string `json:"avatar_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&profile); err != nil {
		return nil, fmt.Errorf("github user decode: %w", err)
	}

	displayName := profile.Name
	if displayName == "" {
		displayName = profile.Login
	}

	email := profile.Email
	// If email is private, fetch from /user/emails endpoint.
	if email == "" {
		email, _ = fetchGitHubPrimaryEmail(ctx, client)
	}

	return &models.User{
		Provider:    "github",
		ProviderID:  fmt.Sprintf("%d", profile.ID),
		Email:       email,
		DisplayName: displayName,
		AvatarURL:   profile.AvatarURL,
	}, nil
}

// fetchGitHubPrimaryEmail retrieves the user's primary verified email from
// the GitHub /user/emails API. Returns ("", nil) if none found.
func fetchGitHubPrimaryEmail(ctx context.Context, client *http.Client) (string, error) {
	resp, err := client.Get("https://api.github.com/user/emails")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("github emails: unexpected status %d", resp.StatusCode)
	}

	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&emails); err != nil {
		return "", err
	}

	for _, e := range emails {
		if e.Primary && e.Verified {
			return e.Email, nil
		}
	}
	return "", nil
}

// handleLogout clears the session and redirects to the login page.
// For HTMX requests it uses the HX-Redirect header so the browser performs
// a full page navigation instead of swapping the response into the DOM.
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	s.clearSession(w, r)
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", "/login")
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// optionalAuth is a Chi middleware that injects the authenticated user into
// the request context if a valid session exists, but does not redirect or
// return an error if the session is absent or invalid. This allows handlers
// to serve different content for logged-in vs. logged-out visitors.
func (s *Server) optionalAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if user, ok := s.getUserFromSession(r); ok {
			ctx := context.WithValue(r.Context(), userContextKey, user)
			r = r.WithContext(ctx)
		}
		next.ServeHTTP(w, r)
	})
}

// requireAuth is a Chi middleware that checks for a valid session. If the user
// is not authenticated, it saves the current request URL in the session as
// "return_to" (for GET requests only) and redirects to /login. If
// authenticated, it injects the *models.User into the request context.
func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := s.getUserFromSession(r)
		if !ok {
			// Only preserve the destination for GET requests — there is no
			// sensible way to replay a POST after login.
			if r.Method == http.MethodGet {
				path := r.URL.Path
				// Don't store the login or logout paths to avoid redirect loops.
				if path != "/login" && path != "/logout" {
					dest := r.URL.RequestURI()
					sess, _ := s.sessionStore.Get(r, sessionName)
					sess.Values[sessionReturnToKey] = dest
					sess.Options.Path = "/"
					if err := sess.Save(r, w); err != nil {
						slog.Warn("requireAuth: failed to save return_to", "error", err)
					}
				}
			}
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		ctx := context.WithValue(r.Context(), userContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
