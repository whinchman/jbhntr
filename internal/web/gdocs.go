package web

import (
	"bytes"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/oauth2"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"

	"github.com/whinchman/jobhuntr/internal/config"
	"github.com/whinchman/jobhuntr/internal/exporter"
)

// driveOAuthStateName is the session key used to store the CSRF state token
// for the Drive-specific OAuth flow.
const driveOAuthStateName = "drive_oauth_state"

// buildDriveOAuthConfig constructs an oauth2.Config for the Google Drive
// export flow. It requests only the drive.file scope, which is non-sensitive
// and only grants access to files the app creates.
func buildDriveOAuthConfig(cfg config.GoogleDriveConfig, baseURL string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  strings.TrimRight(baseURL, "/") + "/auth/google-drive/callback",
		Scopes:       []string{"https://www.googleapis.com/auth/drive.file"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://accounts.google.com/o/oauth2/v2/auth",
			TokenURL: "https://oauth2.googleapis.com/token",
		},
	}
}

// handleDriveOAuthStart redirects the user to Google's consent page to
// grant the drive.file scope. It saves the "return_to" query parameter in
// the session so the callback can redirect back to the job page.
func (s *Server) handleDriveOAuthStart(w http.ResponseWriter, r *http.Request) {
	if s.driveOAuthCfg == nil {
		http.Error(w, "Google Drive not configured", http.StatusServiceUnavailable)
		return
	}

	state, err := generateState()
	if err != nil {
		slog.Error("handleDriveOAuthStart: failed to generate state", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Persist both the CSRF state token and the return_to destination in a
	// single session write to avoid the second Get overwriting the first.
	sess, _ := s.sessionStore.Get(r, sessionName)
	sess.Values[driveOAuthStateName] = state
	returnTo := r.URL.Query().Get("return_to")
	if returnTo != "" && strings.HasPrefix(returnTo, "/") && !strings.HasPrefix(returnTo, "//") && !strings.Contains(returnTo, "://") {
		sess.Values[sessionReturnToKey] = returnTo
	}
	sess.Options.Path = "/"
	if err := sess.Save(r, w); err != nil {
		slog.Error("handleDriveOAuthStart: failed to save session", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// access_type=offline requests a refresh token so we can refresh without
	// prompting the user again.
	authURL := s.driveOAuthCfg.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce)
	http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
}

// handleDriveOAuthCallback receives the authorization code from Google,
// exchanges it for a token, encrypts and stores it in the DB, then redirects
// to the original destination stored in the session.
func (s *Server) handleDriveOAuthCallback(w http.ResponseWriter, r *http.Request) {
	if s.driveOAuthCfg == nil || s.driveTokenStore == nil {
		http.Error(w, "Google Drive not configured", http.StatusServiceUnavailable)
		return
	}

	// Verify CSRF state.
	sess, err := s.sessionStore.Get(r, sessionName)
	if err != nil {
		slog.Warn("handleDriveOAuthCallback: invalid session", "error", err)
		http.Error(w, "session error", http.StatusBadRequest)
		return
	}
	expectedState, _ := sess.Values[driveOAuthStateName].(string)
	if expectedState == "" || r.URL.Query().Get("state") != expectedState {
		slog.Warn("handleDriveOAuthCallback: state mismatch")
		http.Error(w, "invalid state", http.StatusBadRequest)
		return
	}
	delete(sess.Values, driveOAuthStateName)
	if err := sess.Save(r, w); err != nil {
		slog.Warn("handleDriveOAuthCallback: failed to clear state from session", "error", err)
	}

	// Handle user-denied consent.
	if errMsg := r.URL.Query().Get("error"); errMsg != "" {
		slog.Warn("handleDriveOAuthCallback: provider returned error", "error", errMsg)
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	// Exchange code for token.
	code := r.URL.Query().Get("code")
	tok, err := s.driveOAuthCfg.Exchange(r.Context(), code)
	if err != nil {
		slog.Error("handleDriveOAuthCallback: code exchange failed", "error", err)
		http.Error(w, "authentication failed", http.StatusInternalServerError)
		return
	}

	// Load the authenticated user from context.
	user := UserFromContext(r.Context())
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Encrypt and persist the token.
	encrypted, err := encryptToken(deriveKey(s.cfg.Auth.SessionSecret), tok)
	if err != nil {
		slog.Error("handleDriveOAuthCallback: failed to encrypt token", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if err := s.driveTokenStore.UpsertGoogleDriveToken(r.Context(), user.ID, encrypted); err != nil {
		slog.Error("handleDriveOAuthCallback: failed to store token", "error", err, "user_id", user.ID)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, s.consumeReturnTo(w, r), http.StatusSeeOther)
}

// handleSendResumeToGoogleDocs exports job.ResumeMarkdown as a Google Doc.
func (s *Server) handleSendResumeToGoogleDocs(w http.ResponseWriter, r *http.Request) {
	s.handleSendToGoogleDocs(w, r, false)
}

// handleSendCoverToGoogleDocs exports job.CoverMarkdown as a Google Doc.
func (s *Server) handleSendCoverToGoogleDocs(w http.ResponseWriter, r *http.Request) {
	s.handleSendToGoogleDocs(w, r, true)
}

// handleSendToGoogleDocs is the shared implementation for both send-to-Drive
// handlers. When isCover is false the resume Markdown is used; when true the
// cover letter Markdown is used.
func (s *Server) handleSendToGoogleDocs(w http.ResponseWriter, r *http.Request, isCover bool) {
	// Step 1: Parse job ID.
	id, ok := parseID(w, r)
	if !ok {
		return
	}

	// Step 2: Load authenticated user.
	user := UserFromContext(r.Context())
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Step 3: Fetch job and verify ownership.
	job, err := s.store.GetJob(r.Context(), user.ID, id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, "job not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get job")
		return
	}
	if job.UserID != user.ID {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}

	// Step 4: Check that the relevant Markdown field is non-empty.
	var markdown string
	var docTitle string
	var returnPath string
	if isCover {
		markdown = job.CoverMarkdown
		docTitle = fmt.Sprintf("Cover Letter \u2014 %s at %s", job.Title, job.Company)
		returnPath = fmt.Sprintf("/output/%d/cover_letter.gdoc", id)
	} else {
		markdown = job.ResumeMarkdown
		docTitle = fmt.Sprintf("Resume \u2014 %s at %s", job.Title, job.Company)
		returnPath = fmt.Sprintf("/output/%d/resume.gdoc", id)
	}
	if markdown == "" {
		writeError(w, http.StatusBadRequest, "document content is empty")
		return
	}

	// Step 5: Load encrypted token; redirect to OAuth if not found.
	if s.driveOAuthCfg == nil || s.driveTokenStore == nil {
		writeError(w, http.StatusServiceUnavailable, "Google Drive not configured")
		return
	}
	encryptedJSON, err := s.driveTokenStore.GetGoogleDriveToken(r.Context(), user.ID)
	if err != nil {
		// Token not found — redirect to Drive OAuth consent.
		redirectURL := fmt.Sprintf("/auth/google-drive?return_to=%s", url.QueryEscape(returnPath))
		http.Redirect(w, r, redirectURL, http.StatusFound)
		return
	}

	// Step 6: Decrypt the token.
	tok, err := decryptToken(deriveKey(s.cfg.Auth.SessionSecret), encryptedJSON)
	if err != nil {
		slog.Error("handleSendToGoogleDocs: failed to decrypt token", "error", err, "user_id", user.ID)
		// Treat as missing token — force re-auth.
		redirectURL := fmt.Sprintf("/auth/google-drive?return_to=%s", url.QueryEscape(returnPath))
		http.Redirect(w, r, redirectURL, http.StatusFound)
		return
	}

	// Step 7: Build token source (handles automatic refresh).
	tokenSource := s.driveOAuthCfg.TokenSource(r.Context(), tok)

	// Step 8: Construct Drive service.
	driveService, err := drive.NewService(r.Context(), option.WithTokenSource(tokenSource))
	if err != nil {
		slog.Error("handleSendToGoogleDocs: failed to create drive service", "error", err, "user_id", user.ID)
		writeError(w, http.StatusInternalServerError, "failed to connect to Google Drive")
		return
	}

	// Step 9: Convert Markdown to DOCX.
	docxBytes, err := exporter.ToDocx(markdown)
	if err != nil {
		slog.Error("handleSendToGoogleDocs: failed to generate docx", "error", err, "job_id", id)
		writeError(w, http.StatusInternalServerError, "failed to generate document")
		return
	}

	// Steps 10-11: Upload to Drive with DOCX source, Google Doc target MIME type.
	file := &drive.File{
		Name:     docTitle,
		MimeType: "application/vnd.google-apps.document",
	}
	result, err := driveService.Files.Create(file).
		Media(
			bytes.NewReader(docxBytes),
			googleapi.ContentType("application/vnd.openxmlformats-officedocument.wordprocessingml.document"),
		).
		Fields("id, webViewLink").
		Do()
	if err != nil {
		slog.Error("handleSendToGoogleDocs: drive upload failed", "error", err, "user_id", user.ID, "job_id", id)
		writeError(w, http.StatusInternalServerError, "failed to upload to Google Drive")
		return
	}

	// Step 12: Write possibly-refreshed token back to DB (best-effort).
	refreshedTok, refreshErr := tokenSource.Token()
	if refreshErr == nil {
		if encrypted, encErr := encryptToken(deriveKey(s.cfg.Auth.SessionSecret), refreshedTok); encErr == nil {
			if upsertErr := s.driveTokenStore.UpsertGoogleDriveToken(r.Context(), user.ID, encrypted); upsertErr != nil {
				slog.Warn("handleSendToGoogleDocs: failed to write back refreshed token", "error", upsertErr, "user_id", user.ID)
			}
		}
	}

	// Return JSON with the Google Doc URL.
	writeJSON(w, http.StatusOK, map[string]string{"url": result.WebViewLink})
}
