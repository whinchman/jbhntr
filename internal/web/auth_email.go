package web

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"html/template"
	"log/slog"
	"net"
	"net/http"
	"net/mail"
	"time"

	"github.com/gorilla/csrf"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/time/rate"

	"github.com/whinchman/jobhuntr/internal/store"
)

// generateToken returns a cryptographically random 32-byte token encoded as a
// 64-character lowercase hex string.
func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// getLimiter returns (or creates) a per-IP rate limiter that allows 5
// requests per minute with a burst of 5.
func (s *Server) getLimiter(ip string) *rate.Limiter {
	v, _ := s.rateLimiters.LoadOrStore(ip, rate.NewLimiter(rate.Every(time.Minute/5), 5))
	return v.(*rate.Limiter)
}

// rateLimit checks the per-IP limiter.  If the limit is exceeded it sets a
// flash message, redirects back to the current path, and returns false.
func (s *Server) rateLimit(w http.ResponseWriter, r *http.Request) bool {
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	if !s.getLimiter(ip).Allow() {
		s.setFlash(w, r, "Too many requests. Please wait a minute and try again.")
		http.Redirect(w, r, r.URL.Path, http.StatusSeeOther)
		return false
	}
	return true
}

// renderEmailBody executes a named email template into a string.
func (s *Server) renderEmailBody(name string, data any) (string, error) {
	var buf bytes.Buffer
	if err := emailTemplates.ExecuteTemplate(&buf, name, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// ─── template data types for email bodies ────────────────────────────────────

type verifyEmailTemplateData struct {
	DisplayName string
	VerifyURL   string
	Year        int
}

type resetEmailTemplateData struct {
	DisplayName string
	ResetURL    string
	Year        int
}

// ─── handlers ────────────────────────────────────────────────────────────────

// handleRegisterGet renders the registration form.
func (s *Server) handleRegisterGet(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.registerTmpl.ExecuteTemplate(w, "register.html", registerData{
		CSRFToken: csrf.Token(r),
	}); err != nil {
		slog.Error("register template render error", "error", err)
	}
}

// handleRegisterPost processes the registration form.
func (s *Server) handleRegisterPost(w http.ResponseWriter, r *http.Request) {
	if !s.rateLimit(w, r) {
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	displayName := r.FormValue("display_name")
	email := r.FormValue("email")
	password := r.FormValue("password")
	confirmPassword := r.FormValue("confirm_password")

	renderErr := func(flash string) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		data := registerData{
			Flash:     flash,
			CSRFToken: csrf.Token(r),
		}
		data.Form.DisplayName = displayName
		data.Form.Email = email
		if err := s.registerTmpl.ExecuteTemplate(w, "register.html", data); err != nil {
			slog.Error("register template render error", "error", err)
		}
	}

	// Validate input.
	if displayName == "" {
		renderErr("Display name is required.")
		return
	}
	if _, err := mail.ParseAddress(email); err != nil {
		renderErr("Please enter a valid email address.")
		return
	}
	if len(password) < 8 {
		renderErr("Password must be at least 8 characters.")
		return
	}
	if password != confirmPassword {
		renderErr("Passwords do not match.")
		return
	}

	// Hash password.
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		slog.Error("bcrypt error during registration", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Generate verification token.
	verifyToken, err := generateToken()
	if err != nil {
		slog.Error("generateToken error during registration", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	verifyExpiresAt := time.Now().UTC().Add(24 * time.Hour)

	// Create user in DB.
	user, err := s.userStore.CreateUserWithPassword(r.Context(), email, displayName, string(hash), verifyToken, verifyExpiresAt)
	if err != nil {
		if err == store.ErrEmailTaken {
			renderErr("An account with that email already exists. Try signing in.")
			return
		}
		slog.Error("CreateUserWithPassword error", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Send verification email (best-effort).
	baseURL := s.baseURL
	verifyURL := baseURL + "/verify-email?token=" + verifyToken
	emailBody, err := s.renderEmailBody("email/verify_email.html", verifyEmailTemplateData{
		DisplayName: user.DisplayName,
		VerifyURL:   verifyURL,
		Year:        time.Now().Year(),
	})
	if err != nil {
		slog.Error("failed to render verification email", "error", err)
	} else if s.mailer != nil {
		if err := s.mailer.SendMail(r.Context(), user.Email, "Verify your email address", emailBody); err != nil {
			slog.Error("failed to send verification email", "error", err, "user_id", user.ID)
		}
	}

	// Log user in immediately.
	if err := s.setSession(w, r, user); err != nil {
		slog.Error("setSession error after registration", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/onboarding", http.StatusSeeOther)
}

// handleLoginPost processes the email/password login form.
func (s *Server) handleLoginPost(w http.ResponseWriter, r *http.Request) {
	if !s.rateLimit(w, r) {
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	email := r.FormValue("email")
	password := r.FormValue("password")

	user, err := s.userStore.GetUserByEmail(r.Context(), email)
	if err != nil || user == nil {
		// Constant-time delay to prevent user enumeration via timing.
		time.Sleep(200 * time.Millisecond)
		s.setFlash(w, r, "Invalid email or password.")
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if user.PasswordHash == nil {
		// OAuth-only account — no password set.
		time.Sleep(200 * time.Millisecond)
		s.setFlash(w, r, "Invalid email or password.")
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(*user.PasswordHash), []byte(password)); err != nil {
		s.setFlash(w, r, "Invalid email or password.")
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if user.BannedAt != nil {
		s.setFlash(w, r, "Your account has been suspended.")
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if err := s.setSession(w, r, user); err != nil {
		slog.Error("setSession error during login", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if !user.OnboardingComplete {
		http.Redirect(w, r, "/onboarding", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, s.consumeReturnTo(w, r), http.StatusSeeOther)
}

// handleForgotPasswordGet renders the forgot-password form.
func (s *Server) handleForgotPasswordGet(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.forgotPasswordTmpl.ExecuteTemplate(w, "forgot_password.html", forgotPasswordData{
		CSRFToken: csrf.Token(r),
	}); err != nil {
		slog.Error("forgot_password template render error", "error", err)
	}
}

// handleForgotPasswordPost processes the forgot-password form submission.
// It always shows the same success message to prevent email enumeration.
func (s *Server) handleForgotPasswordPost(w http.ResponseWriter, r *http.Request) {
	if !s.rateLimit(w, r) {
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	email := r.FormValue("email")

	// Look up user — ignore error (nil user is handled below).
	user, _ := s.userStore.GetUserByEmail(r.Context(), email)

	if user != nil && user.PasswordHash != nil {
		token, err := generateToken()
		if err != nil {
			slog.Error("generateToken error for password reset", "error", err)
		} else {
			expiresAt := time.Now().UTC().Add(1 * time.Hour)
			if err := s.userStore.SetResetToken(r.Context(), user.ID, token, expiresAt); err != nil {
				slog.Error("SetResetToken error", "error", err, "user_id", user.ID)
			} else {
				resetURL := s.baseURL + "/reset-password?token=" + token
				emailBody, err := s.renderEmailBody("email/reset_password.html", resetEmailTemplateData{
					DisplayName: user.DisplayName,
					ResetURL:    resetURL,
					Year:        time.Now().Year(),
				})
				if err != nil {
					slog.Error("failed to render reset email", "error", err)
				} else if s.mailer != nil {
					if err := s.mailer.SendMail(r.Context(), user.Email, "Reset your password", emailBody); err != nil {
						slog.Error("failed to send reset email", "error", err, "user_id", user.ID)
					}
				}
			}
		}
	}

	// Always respond with the same message.
	s.setFlash(w, r, "If that email is registered, a reset link has been sent.")
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// handleResetPasswordGet validates the reset token and renders the
// reset-password form (or an error state if the token is invalid/expired).
func (s *Server) handleResetPasswordGet(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if token == "" {
		if err := s.resetPasswordTmpl.ExecuteTemplate(w, "reset_password.html", resetPasswordData{
			TokenValid: false,
			Flash:      "Reset link is invalid or has expired.",
		}); err != nil {
			slog.Error("reset_password template render error", "error", err)
		}
		return
	}

	user, err := s.userStore.GetUserByResetToken(r.Context(), token)
	if err != nil {
		slog.Error("GetUserByResetToken error", "error", err)
		user = nil
	}

	if user == nil {
		if err := s.resetPasswordTmpl.ExecuteTemplate(w, "reset_password.html", resetPasswordData{
			TokenValid: false,
			Flash:      "Reset link is invalid or has expired.",
		}); err != nil {
			slog.Error("reset_password template render error", "error", err)
		}
		return
	}

	if err := s.resetPasswordTmpl.ExecuteTemplate(w, "reset_password.html", resetPasswordData{
		TokenValid: true,
		Token:      token,
		CSRFToken:  csrf.Token(r),
	}); err != nil {
		slog.Error("reset_password template render error", "error", err)
	}
}

// handleResetPasswordPost processes the reset-password form submission.
func (s *Server) handleResetPasswordPost(w http.ResponseWriter, r *http.Request) {
	if !s.rateLimit(w, r) {
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	token := r.FormValue("token")
	password := r.FormValue("password")
	confirmPassword := r.FormValue("confirm_password")

	if password != confirmPassword {
		s.setFlash(w, r, "Passwords do not match.")
		http.Redirect(w, r, "/reset-password?token="+token, http.StatusSeeOther)
		return
	}
	if len(password) < 8 {
		s.setFlash(w, r, "Password must be at least 8 characters.")
		http.Redirect(w, r, "/reset-password?token="+token, http.StatusSeeOther)
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		slog.Error("bcrypt error during password reset", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	user, err := s.userStore.ConsumeResetToken(r.Context(), token, string(hash))
	if err != nil {
		slog.Error("ConsumeResetToken error", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if user == nil {
		s.setFlash(w, r, "Reset link has expired.")
		http.Redirect(w, r, "/forgot-password", http.StatusSeeOther)
		return
	}

	if err := s.setSession(w, r, user); err != nil {
		slog.Error("setSession error after password reset", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	s.setFlash(w, r, "Your password has been updated.")
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// handleVerifyEmail handles the email verification link.
func (s *Server) handleVerifyEmail(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")

	user, err := s.userStore.ConsumeVerifyToken(r.Context(), token)
	if err != nil {
		slog.Error("ConsumeVerifyToken error", "error", err)
	}

	if user == nil {
		s.setFlash(w, r, "Verification link is invalid or has expired.")
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	s.setFlash(w, r, "Your email has been verified.")
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// ─── email template loading ───────────────────────────────────────────────────

// emailTemplates is parsed separately from page templates because email
// bodies do not extend layout.html — they are standalone plain-text or
// simple HTML fragments.
var emailTemplates = template.Must(template.ParseFS(templateFS,
	"templates/email/verify_email.html",
	"templates/email/reset_password.html",
))
