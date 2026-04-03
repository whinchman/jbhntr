// Package web provides the HTTP API server for jobhuntr.
package web

import (
	"context"
	"embed"
	"encoding/json"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/gorilla/csrf"
	"github.com/gorilla/sessions"
	"golang.org/x/oauth2"

	"github.com/whinchman/jobhuntr/internal/config"
	"github.com/whinchman/jobhuntr/internal/exporter"
	"github.com/whinchman/jobhuntr/internal/models"
	"github.com/whinchman/jobhuntr/internal/store"
)

// jobDetailData is the template data for the job detail page.
type jobDetailData struct {
	Job        *models.Job
	ResumeHTML template.HTML
	CoverHTML  template.HTML
	CSRFToken  string
	User       *models.User
}

//go:embed templates
var templateFS embed.FS

// slogRequestLogger is a chi middleware that logs each request with slog.
func slogRequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := chimw.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		slog.Info("http request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", ww.Status(),
			"duration", time.Since(start),
		)
	})
}

// JobStore is the subset of store.Store used by the web server.
type JobStore interface {
	GetJob(ctx context.Context, userID int64, id int64) (*models.Job, error)
	ListJobs(ctx context.Context, userID int64, f store.ListJobsFilter) ([]models.Job, error)
	UpdateJobStatus(ctx context.Context, userID int64, id int64, status models.JobStatus) error
}

// UserStore is the subset of store.Store used by the auth system.
type UserStore interface {
	GetUser(ctx context.Context, id int64) (*models.User, error)
	UpsertUser(ctx context.Context, user *models.User) (*models.User, error)
	UpdateUserOnboarding(ctx context.Context, userID int64, displayName string, resume string) error
	UpdateUserDisplayName(ctx context.Context, userID int64, displayName string) error
	CreateUserWithPassword(ctx context.Context, email, displayName, passwordHash, verifyToken string, verifyExpiresAt time.Time) (*models.User, error)
	GetUserByEmail(ctx context.Context, email string) (*models.User, error)
	SetResetToken(ctx context.Context, userID int64, token string, expiresAt time.Time) error
	ConsumeResetToken(ctx context.Context, token string, newPasswordHash string) (*models.User, error)
	SetEmailVerifyToken(ctx context.Context, userID int64, token string, expiresAt time.Time) error
	ConsumeVerifyToken(ctx context.Context, token string) (*models.User, error)
}

// FilterStore is the subset of store.Store used by the settings handlers.
type FilterStore interface {
	CreateUserFilter(ctx context.Context, userID int64, filter *models.UserSearchFilter) error
	ListUserFilters(ctx context.Context, userID int64) ([]models.UserSearchFilter, error)
	DeleteUserFilter(ctx context.Context, userID int64, filterID int64) error
	UpdateUserResume(ctx context.Context, userID int64, markdown string) error
	UpdateUserNtfyTopic(ctx context.Context, userID int64, topic string) error
}

// allStatuses lists job statuses shown as tabs in the dashboard.
var allStatuses = []models.JobStatus{
	models.StatusDiscovered, models.StatusNotified, models.StatusApproved,
	models.StatusGenerating, models.StatusComplete, models.StatusFailed, models.StatusRejected,
}

// Server holds the HTTP dependencies.
type Server struct {
	store          JobStore
	userStore      UserStore
	filterStore    FilterStore
	sessionStore   sessions.Store
	oauthProviders map[string]*oauth2.Config
	baseURL        string

	templates      *template.Template
	detailTmpl     *template.Template
	settingsTmpl   *template.Template
	profileTmpl    *template.Template
	onboardingTmpl *template.Template
	loginTmpl      *template.Template

	startTime    time.Time
	lastScrapeFn func() time.Time // optional; returns last scrape time

	cfg *config.Config
}

// NewServer constructs a Server and parses embedded templates.
// All optional dependencies (UserStore, FilterStore, Config) are set to nil.
func NewServer(st JobStore) *Server {
	return NewServerWithConfig(st, nil, nil, nil)
}

// NewServerWithConfig constructs a Server with config, auth, and filter store.
// Pass nil cfg or nil userStore/filterStore to disable settings/auth.
func NewServerWithConfig(st JobStore, us UserStore, fs FilterStore, cfg *config.Config) *Server {
	tmpl := template.Must(template.ParseFS(templateFS,
		"templates/layout.html",
		"templates/dashboard.html",
		"templates/partials/job_rows.html",
	))
	detail := template.Must(template.ParseFS(templateFS,
		"templates/layout.html",
		"templates/job_detail.html",
	))
	settings := template.Must(template.ParseFS(templateFS,
		"templates/layout.html",
		"templates/settings.html",
	))
	profileTmpl := template.Must(template.ParseFS(templateFS,
		"templates/layout.html",
		"templates/profile.html",
	))
	onboardingTmpl := template.Must(template.ParseFS(templateFS,
		"templates/layout.html",
		"templates/onboarding.html",
	))
	loginTmpl := template.Must(template.ParseFS(templateFS, "templates/login.html"))

	srv := &Server{
		store:          st,
		userStore:      us,
		filterStore:    fs,
		templates:      tmpl,
		detailTmpl:     detail,
		settingsTmpl:   settings,
		profileTmpl:    profileTmpl,
		onboardingTmpl: onboardingTmpl,
		loginTmpl:      loginTmpl,
		startTime:      time.Now(),
		cfg:            cfg,
	}

	// Set up auth if session secret is configured and a user store is available.
	if cfg != nil && cfg.Auth.SessionSecret != "" && us != nil {
		sessStore := sessions.NewCookieStore([]byte(cfg.Auth.SessionSecret))
		sessStore.Options = &sessions.Options{
			Path:     "/",
			MaxAge:   sessionMaxAge,
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			Secure:   strings.HasPrefix(cfg.Server.BaseURL, "https"),
		}
		srv.sessionStore = sessStore
		srv.oauthProviders = oauthProviders(cfg.Auth, cfg.Server.BaseURL)
		srv.baseURL = cfg.Server.BaseURL
	} else if cfg != nil {
		srv.baseURL = cfg.Server.BaseURL
	}

	return srv
}

// WithLastScrapeFn sets a function the server calls to obtain the last scrape
// time for the /health endpoint. Call this after NewServerWithConfig.
func (s *Server) WithLastScrapeFn(fn func() time.Time) *Server {
	s.lastScrapeFn = fn
	return s
}

// WithTestOAuthProvider replaces an OAuth provider's configuration.
// Intended for integration tests that need to point OAuth endpoints at a
// mock server.
func (s *Server) WithTestOAuthProvider(name string, cfg *oauth2.Config) {
	if s.oauthProviders == nil {
		s.oauthProviders = make(map[string]*oauth2.Config)
	}
	s.oauthProviders[name] = cfg
}

// Handler builds and returns the chi router.
func (s *Server) Handler() http.Handler {
	r := chi.NewRouter()
	r.Use(slogRequestLogger)
	r.Use(chimw.Recoverer)

	// CSRF protection — only apply if auth is configured (allows tests
	// without auth to skip it).
	if s.sessionStore != nil {
		csrfSecure := strings.HasPrefix(s.baseURL, "https")
		csrfMiddleware := csrf.Protect(
			[]byte(s.cfg.Auth.SessionSecret),
			csrf.Secure(csrfSecure),
			csrf.Path("/"),
			csrf.SameSite(csrf.SameSiteLaxMode),
		)
		r.Use(csrfMiddleware)
	}

	// Static file serving — serve templates/static/ at /static/*
	staticFS, err := fs.Sub(templateFS, "templates/static")
	if err != nil {
		panic(err)
	}
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	// Public routes — no auth required.
	r.Get("/health", s.handleHealth)
	r.Get("/healthz", s.handleHealthz)
	r.Get("/login", s.handleLogin)

	// Auth routes are only registered when auth is configured.
	// Registering them unconditionally would allow callers to hit handlers
	// that dereference s.sessionStore/s.userStore/s.oauthProviders while nil.
	if s.sessionStore != nil {
		r.Get("/auth/{provider}", s.handleOAuthStart)
		r.Get("/auth/{provider}/callback", s.handleOAuthCallback)
	}

	// Optional-auth routes — serve different content for logged-in vs. logged-out users.
	r.Group(func(r chi.Router) {
		if s.sessionStore != nil {
			r.Use(s.optionalAuth)
		}

		r.Get("/", s.handleDashboard)
		r.Get("/partials/job-table", s.handleJobTablePartial)
	})

	// Protected routes — require authenticated session.
	r.Group(func(r chi.Router) {
		if s.sessionStore != nil {
			r.Use(s.requireAuth)
		}

		r.Get("/jobs/{id}", s.handleJobDetail)
		r.Get("/output/{id}/resume.pdf", s.handleDownloadResume)
		r.Get("/output/{id}/cover_letter.pdf", s.handleDownloadCover)
		r.Get("/output/{id}/resume.md", s.handleDownloadResumeMarkdown)
		r.Get("/output/{id}/cover_letter.md", s.handleDownloadCoverMarkdown)
		r.Get("/output/{id}/resume.docx", s.handleDownloadResumeDocx)
		r.Get("/output/{id}/cover_letter.docx", s.handleDownloadCoverDocx)

		r.Get("/settings", s.handleSettings)
		r.Post("/settings/resume", s.handleSaveResume)
		r.Post("/settings/ntfy", s.handleSaveNtfyTopic)
		r.Post("/settings/filters", s.handleAddFilter)
		r.Post("/settings/filters/remove", s.handleRemoveFilter)

		r.Get("/onboarding", s.handleOnboardingGet)
		r.Post("/onboarding", s.handleOnboardingPost)

		r.Get("/profile", s.handleProfileGet)
		r.Post("/profile", s.handleProfileSave)

		r.Post("/logout", s.handleLogout)

		r.Route("/api/jobs", func(r chi.Router) {
			r.Get("/", s.handleListJobs)
			r.Get("/{id}", s.handleGetJob)
			r.Post("/{id}/approve", s.handleApproveJob)
			r.Post("/{id}/reject", s.handleRejectJob)
		})
	})

	return r
}

// columnDef drives a single sortable column header in the dashboard template.
type columnDef struct {
	Key       string // DB column key (e.g. "title")
	Label     string // Display name
	Arrow     string // "▲", "▼", or "" if not the active sort column
	NextOrder string // The order value to use when this header is clicked
}

// sortableColumns are the columns the dashboard can be sorted by.
var sortableColumns = []struct{ key, label string }{
	{"title", "Title"},
	{"company", "Company"},
	{"location", "Location"},
	{"salary", "Salary"},
	{"status", "Status"},
	{"discovered_at", "Date"},
}

// allowedSortColumns prevents SQL injection via the sort parameter.
var allowedSortColumns = map[string]bool{
	"title": true, "company": true, "location": true,
	"salary": true, "status": true, "discovered_at": true,
}

func buildColumns(activeSort, activeOrder string) []columnDef {
	cols := make([]columnDef, len(sortableColumns))
	for i, sc := range sortableColumns {
		col := columnDef{Key: sc.key, Label: sc.label}
		if sc.key == activeSort {
			if activeOrder == "asc" {
				col.Arrow = "\u25B2"
				col.NextOrder = "desc"
			} else {
				col.Arrow = "\u25BC"
				col.NextOrder = "asc"
			}
		} else {
			col.NextOrder = "asc"
		}
		cols[i] = col
	}
	return cols
}

type dashboardData struct {
	Jobs         []models.Job
	Statuses     []models.JobStatus
	ActiveStatus string
	Search       string
	Sort         string
	Order        string
	Columns      []columnDef
	CSRFToken    string
	User         *models.User
}

func parseSortParams(q url.Values) (string, string) {
	sort := q.Get("sort")
	order := q.Get("order")
	if !allowedSortColumns[sort] {
		sort = "discovered_at"
	}
	if order != "asc" {
		order = "desc"
	}
	return sort, order
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	sort, order := parseSortParams(q)
	f := store.ListJobsFilter{
		Status: models.JobStatus(q.Get("status")),
		Search: q.Get("q"),
		Sort:   sort,
		Order:  order,
	}
	user := UserFromContext(r.Context())
	var userID int64
	if user != nil {
		userID = user.ID
	}
	jobs, err := s.store.ListJobs(r.Context(), userID, f)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list jobs")
		return
	}
	if jobs == nil {
		jobs = []models.Job{}
	}
	data := dashboardData{
		Jobs:         jobs,
		Statuses:     allStatuses,
		ActiveStatus: string(f.Status),
		Search:       f.Search,
		Sort:         sort,
		Order:        order,
		Columns:      buildColumns(sort, order),
		CSRFToken:    csrf.Token(r),
		User:         user,
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.templates.ExecuteTemplate(w, "layout.html", data); err != nil {
		slog.Error("template render error", "error", err)
	}
}

func (s *Server) handleJobTablePartial(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		// Unauthenticated: return an empty fragment so HTMX polling does not
		// receive an error or trigger a redirect loop.
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		return
	}
	q := r.URL.Query()
	sort, order := parseSortParams(q)
	f := store.ListJobsFilter{
		Status: models.JobStatus(q.Get("status")),
		Search: q.Get("q"),
		Sort:   sort,
		Order:  order,
	}
	jobs, err := s.store.ListJobs(r.Context(), user.ID, f)
	if err != nil {
		http.Error(w, "failed to list jobs", http.StatusInternalServerError)
		return
	}
	if jobs == nil {
		jobs = []models.Job{}
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.templates.ExecuteTemplate(w, "job_rows", jobs); err != nil {
		slog.Error("template render error", "error", err)
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	resp := map[string]interface{}{
		"status": "ok",
		"uptime": time.Since(s.startTime).Round(time.Second).String(),
	}
	if s.lastScrapeFn != nil {
		if t := s.lastScrapeFn(); !t.IsZero() {
			resp["last_scrape"] = t.UTC().Format(time.RFC3339)
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

// handleHealthz is a minimal liveness probe used by Render's health check.
// It returns 200 OK with {"status":"ok"} — no auth required, no DB query.
func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleListJobs(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	f := store.ListJobsFilter{
		Search: q.Get("q"),
	}
	if raw := q.Get("status"); raw != "" {
		f.Status = models.JobStatus(raw)
	}
	if raw := q.Get("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			f.Limit = n
		}
	}
	if raw := q.Get("page"); raw != "" {
		if p, err := strconv.Atoi(raw); err == nil && p > 1 && f.Limit > 0 {
			f.Offset = (p - 1) * f.Limit
		}
	}

	user := UserFromContext(r.Context())
	var userID int64
	if user != nil {
		userID = user.ID
	}
	jobs, err := s.store.ListJobs(r.Context(), userID, f)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list jobs")
		return
	}
	if jobs == nil {
		jobs = []models.Job{}
	}
	writeJSON(w, http.StatusOK, jobs)
}

func (s *Server) handleGetJob(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	user := UserFromContext(r.Context())
	var userID int64
	if user != nil {
		userID = user.ID
	}
	job, err := s.store.GetJob(r.Context(), userID, id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, "job not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get job")
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (s *Server) handleApproveJob(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}

	user := UserFromContext(r.Context())
	var userID int64
	if user != nil {
		userID = user.ID
	}
	job, err := s.store.GetJob(r.Context(), userID, id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, "job not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get job")
		return
	}

	if job.Status != models.StatusDiscovered && job.Status != models.StatusNotified {
		writeError(w, http.StatusConflict, "job cannot be approved from status "+string(job.Status))
		return
	}

	if err := s.store.UpdateJobStatus(r.Context(), userID, id, models.StatusApproved); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update job status")
		return
	}
	job.Status = models.StatusApproved
	s.respondJobAction(w, r, job)
}

func (s *Server) handleRejectJob(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}

	user := UserFromContext(r.Context())
	var userID int64
	if user != nil {
		userID = user.ID
	}
	job, err := s.store.GetJob(r.Context(), userID, id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, "job not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get job")
		return
	}

	if err := s.store.UpdateJobStatus(r.Context(), userID, id, models.StatusRejected); err != nil {
		if strings.Contains(err.Error(), "invalid transition") {
			writeError(w, http.StatusConflict, "job cannot be rejected from status "+string(job.Status))
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to update job status")
		return
	}
	job.Status = models.StatusRejected
	s.respondJobAction(w, r, job)
}

// respondJobAction returns HTML for HTMX requests and JSON for API clients.
func (s *Server) respondJobAction(w http.ResponseWriter, r *http.Request, job *models.Job) {
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := s.templates.ExecuteTemplate(w, "job_rows", []models.Job{*job}); err != nil {
			slog.Error("template render error", "error", err)
		}
		return
	}
	writeJSON(w, http.StatusOK, job)
}

// ─── settings handlers ────────────────────────────────────────────────────────

type settingsData struct {
	Filters   []models.UserSearchFilter
	Resume    string
	NtfyTopic string
	Saved     bool
	CSRFToken string
	User      *models.User
}

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	if s.filterStore == nil {
		http.Error(w, "settings not configured", http.StatusServiceUnavailable)
		return
	}
	user := UserFromContext(r.Context())
	var userID int64
	if user != nil {
		userID = user.ID
	}

	filters, err := s.filterStore.ListUserFilters(r.Context(), userID)
	if err != nil {
		slog.Error("failed to list user filters", "error", err)
		http.Error(w, "failed to load settings", http.StatusInternalServerError)
		return
	}
	if filters == nil {
		filters = []models.UserSearchFilter{}
	}

	resume := ""
	ntfyTopic := ""
	if user != nil {
		resume = user.ResumeMarkdown
		ntfyTopic = user.NtfyTopic
	}

	data := settingsData{
		Filters:   filters,
		Resume:    resume,
		NtfyTopic: ntfyTopic,
		Saved:     r.URL.Query().Get("saved") == "1",
		CSRFToken: csrf.Token(r),
		User:      user,
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.settingsTmpl.ExecuteTemplate(w, "layout.html", data); err != nil {
		slog.Error("settings template render error", "error", err)
	}
}

func (s *Server) handleSaveResume(w http.ResponseWriter, r *http.Request) {
	if s.filterStore == nil {
		http.Error(w, "settings not configured", http.StatusServiceUnavailable)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	content := r.FormValue("resume")
	user := UserFromContext(r.Context())
	var userID int64
	if user != nil {
		userID = user.ID
	}

	if err := s.filterStore.UpdateUserResume(r.Context(), userID, content); err != nil {
		slog.Error("failed to save resume", "error", err, "user_id", userID)
		http.Error(w, "failed to save resume", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/settings?saved=1", http.StatusSeeOther)
}

func (s *Server) handleSaveNtfyTopic(w http.ResponseWriter, r *http.Request) {
	if s.filterStore == nil {
		http.Error(w, "settings not configured", http.StatusServiceUnavailable)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	topic := strings.TrimSpace(r.FormValue("ntfy_topic"))
	user := UserFromContext(r.Context())
	var userID int64
	if user != nil {
		userID = user.ID
	}

	if err := s.filterStore.UpdateUserNtfyTopic(r.Context(), userID, topic); err != nil {
		slog.Error("failed to save ntfy topic", "error", err, "user_id", userID)
		http.Error(w, "failed to save notification settings", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/settings?saved=1", http.StatusSeeOther)
}

func (s *Server) handleAddFilter(w http.ResponseWriter, r *http.Request) {
	if s.filterStore == nil {
		http.Error(w, "settings not configured", http.StatusServiceUnavailable)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	user := UserFromContext(r.Context())
	var userID int64
	if user != nil {
		userID = user.ID
	}

	minSalary := 0
	if raw := r.FormValue("min_salary"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			minSalary = n
		}
	}
	filter := &models.UserSearchFilter{
		Keywords:  r.FormValue("keywords"),
		Location:  r.FormValue("location"),
		MinSalary: minSalary,
	}

	if err := s.filterStore.CreateUserFilter(r.Context(), userID, filter); err != nil {
		slog.Error("failed to create filter", "error", err, "user_id", userID)
		http.Error(w, "failed to save filter", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/settings?saved=1", http.StatusSeeOther)
}

func (s *Server) handleRemoveFilter(w http.ResponseWriter, r *http.Request) {
	if s.filterStore == nil {
		http.Error(w, "settings not configured", http.StatusServiceUnavailable)
		return
	}
	filterID, err := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid filter id", http.StatusBadRequest)
		return
	}
	user := UserFromContext(r.Context())
	var userID int64
	if user != nil {
		userID = user.ID
	}

	if err := s.filterStore.DeleteUserFilter(r.Context(), userID, filterID); err != nil {
		slog.Error("failed to delete filter", "error", err, "user_id", userID, "filter_id", filterID)
		http.Error(w, "failed to remove filter", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/settings?saved=1", http.StatusSeeOther)
}

// ─── onboarding handlers ─────────────────────────────────────────────────────

// onboardingData is the template data for the onboarding page.
type onboardingData struct {
	User        *models.User
	CSRFToken   string
	DisplayName string
	Resume      string
	Error       string
}

// handleOnboardingGet renders the onboarding form. If the user has already
// completed onboarding, it redirects them to the home page instead.
func (s *Server) handleOnboardingGet(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user != nil && user.OnboardingComplete {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	displayName := ""
	resume := ""
	if user != nil {
		displayName = user.DisplayName
		resume = user.ResumeMarkdown
	}

	data := onboardingData{
		User:        user,
		CSRFToken:   csrf.Token(r),
		DisplayName: displayName,
		Resume:      resume,
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.onboardingTmpl.ExecuteTemplate(w, "layout.html", data); err != nil {
		slog.Error("onboarding template render error", "error", err)
	}
}

// handleOnboardingPost processes the onboarding form submission. It validates
// the display name, calls UpdateUserOnboarding to save the data and set
// onboarding_complete=true, then redirects to the original destination (if any)
// or to /.
func (s *Server) handleOnboardingPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	user := UserFromContext(r.Context())

	displayName := strings.TrimSpace(r.FormValue("display_name"))
	resume := r.FormValue("resume")

	if displayName == "" || len(displayName) > 100 {
		errMsg := "Display name is required and must be 100 characters or fewer."
		data := onboardingData{
			User:        user,
			CSRFToken:   csrf.Token(r),
			DisplayName: displayName,
			Resume:      resume,
			Error:       errMsg,
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusUnprocessableEntity)
		if err := s.onboardingTmpl.ExecuteTemplate(w, "layout.html", data); err != nil {
			slog.Error("onboarding template render error", "error", err)
		}
		return
	}

	var userID int64
	if user != nil {
		userID = user.ID
	}
	if err := s.userStore.UpdateUserOnboarding(r.Context(), userID, displayName, resume); err != nil {
		slog.Error("failed to save onboarding data", "error", err, "user_id", userID)
		data := onboardingData{
			User:        user,
			CSRFToken:   csrf.Token(r),
			DisplayName: displayName,
			Resume:      resume,
			Error:       "Failed to save your profile. Please try again.",
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
		if err := s.onboardingTmpl.ExecuteTemplate(w, "layout.html", data); err != nil {
			slog.Error("onboarding template render error", "error", err)
		}
		return
	}

	http.Redirect(w, r, s.consumeReturnTo(w, r), http.StatusSeeOther)
}

// ─── profile handlers ─────────────────────────────────────────────────────────

type profileData struct {
	User      *models.User
	CSRFToken string
	Saved     bool
	Error     string
}

func (s *Server) handleProfileGet(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	data := profileData{
		User:      user,
		CSRFToken: csrf.Token(r),
		Saved:     r.URL.Query().Get("saved") == "1",
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.profileTmpl.ExecuteTemplate(w, "layout.html", data); err != nil {
		slog.Error("profile template render error", "error", err)
	}
}

func (s *Server) handleProfileSave(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	user := UserFromContext(r.Context())
	displayName := strings.TrimSpace(r.FormValue("display_name"))

	if displayName == "" || len(displayName) > 100 {
		errMsg := "Display name must be between 1 and 100 characters."
		data := profileData{
			User:      user,
			CSRFToken: csrf.Token(r),
			Error:     errMsg,
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusUnprocessableEntity)
		if err := s.profileTmpl.ExecuteTemplate(w, "layout.html", data); err != nil {
			slog.Error("profile template render error", "error", err)
		}
		return
	}

	var userID int64
	if user != nil {
		userID = user.ID
	}
	if err := s.userStore.UpdateUserDisplayName(r.Context(), userID, displayName); err != nil {
		slog.Error("failed to update display name", "error", err, "user_id", userID)
		data := profileData{
			User:      user,
			CSRFToken: csrf.Token(r),
			Error:     "Failed to save display name. Please try again.",
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
		if err := s.profileTmpl.ExecuteTemplate(w, "layout.html", data); err != nil {
			slog.Error("profile template render error", "error", err)
		}
		return
	}

	http.Redirect(w, r, "/profile?saved=1", http.StatusSeeOther)
}

// ─── detail / download handlers ───────────────────────────────────────────────

func (s *Server) handleJobDetail(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	user := UserFromContext(r.Context())
	var userID int64
	if user != nil {
		userID = user.ID
	}
	job, err := s.store.GetJob(r.Context(), userID, id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "failed to get job", http.StatusInternalServerError)
		return
	}
	data := jobDetailData{
		Job:        job,
		ResumeHTML: template.HTML(job.ResumeHTML), //nolint:gosec // HTML is generated by Claude, not user input
		CoverHTML:  template.HTML(job.CoverHTML),  //nolint:gosec // HTML is generated by Claude, not user input
		CSRFToken:  csrf.Token(r),
		User:       user,
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.detailTmpl.ExecuteTemplate(w, "layout.html", data); err != nil {
		slog.Error("template render error", "error", err)
	}
}

func (s *Server) handleDownloadResume(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	user := UserFromContext(r.Context())
	var userID int64
	if user != nil {
		userID = user.ID
	}
	job, err := s.store.GetJob(r.Context(), userID, id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "failed to get job", http.StatusInternalServerError)
		return
	}
	if job.ResumePDF == "" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Disposition", "attachment; filename=resume.pdf")
	http.ServeFile(w, r, job.ResumePDF)
}

func (s *Server) handleDownloadCover(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	user := UserFromContext(r.Context())
	var userID int64
	if user != nil {
		userID = user.ID
	}
	job, err := s.store.GetJob(r.Context(), userID, id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "failed to get job", http.StatusInternalServerError)
		return
	}
	if job.CoverPDF == "" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Disposition", "attachment; filename=cover_letter.pdf")
	http.ServeFile(w, r, job.CoverPDF)
}

func (s *Server) handleDownloadResumeMarkdown(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	user := UserFromContext(r.Context())
	var userID int64
	if user != nil {
		userID = user.ID
	}
	job, err := s.store.GetJob(r.Context(), userID, id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "failed to get job", http.StatusInternalServerError)
		return
	}
	if job.ResumeMarkdown == "" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=resume.md")
	_, _ = w.Write([]byte(job.ResumeMarkdown))
}

func (s *Server) handleDownloadCoverMarkdown(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	user := UserFromContext(r.Context())
	var userID int64
	if user != nil {
		userID = user.ID
	}
	job, err := s.store.GetJob(r.Context(), userID, id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "failed to get job", http.StatusInternalServerError)
		return
	}
	if job.CoverMarkdown == "" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=cover_letter.md")
	_, _ = w.Write([]byte(job.CoverMarkdown))
}

func (s *Server) handleDownloadResumeDocx(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	user := UserFromContext(r.Context())
	var userID int64
	if user != nil {
		userID = user.ID
	}
	job, err := s.store.GetJob(r.Context(), userID, id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "failed to get job", http.StatusInternalServerError)
		return
	}
	if job.ResumeMarkdown == "" {
		http.NotFound(w, r)
		return
	}
	docxBytes, err := exporter.ToDocx(job.ResumeMarkdown)
	if err != nil {
		slog.Error("failed to generate resume docx", "error", err, "job_id", id)
		http.Error(w, "failed to generate document", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.wordprocessingml.document")
	w.Header().Set("Content-Disposition", "attachment; filename=resume.docx")
	_, _ = w.Write(docxBytes)
}

func (s *Server) handleDownloadCoverDocx(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	user := UserFromContext(r.Context())
	var userID int64
	if user != nil {
		userID = user.ID
	}
	job, err := s.store.GetJob(r.Context(), userID, id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "failed to get job", http.StatusInternalServerError)
		return
	}
	if job.CoverMarkdown == "" {
		http.NotFound(w, r)
		return
	}
	docxBytes, err := exporter.ToDocx(job.CoverMarkdown)
	if err != nil {
		slog.Error("failed to generate cover letter docx", "error", err, "job_id", id)
		http.Error(w, "failed to generate document", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.wordprocessingml.document")
	w.Header().Set("Content-Disposition", "attachment; filename=cover_letter.docx")
	_, _ = w.Write(docxBytes)
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func parseID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	raw := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid job id")
		return 0, false
	}
	return id, true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
