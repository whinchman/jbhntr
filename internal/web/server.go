// Package web provides the HTTP API server for jobhuntr.
package web

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
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
	"github.com/whinchman/jobhuntr/internal/web/admin"
)

// jobDetailData is the template data for the job detail page.
type jobDetailData struct {
	Job                *models.Job
	ResumeHTML         template.HTML
	CoverHTML          template.HTML
	CSRFToken          string
	User               *models.User
	GoogleDriveEnabled bool
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
	UpdateApplicationStatus(ctx context.Context, userID int64, jobID int64, status models.ApplicationStatus) error
	RetryJob(ctx context.Context, userID int64, jobID int64) error
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
	GetUserByResetToken(ctx context.Context, token string) (*models.User, error)
}

// FilterStore is the subset of store.Store used by the settings handlers.
type FilterStore interface {
	CreateUserFilter(ctx context.Context, userID int64, filter *models.UserSearchFilter) error
	ListUserFilters(ctx context.Context, userID int64) ([]models.UserSearchFilter, error)
	DeleteUserFilter(ctx context.Context, userID int64, filterID int64) error
	UpdateUserResume(ctx context.Context, userID int64, markdown string) error
	UpdateUserNtfyTopic(ctx context.Context, userID int64, topic string) error
	ListUserBannedTerms(ctx context.Context, userID int64) ([]models.UserBannedTerm, error)
	CreateUserBannedTerm(ctx context.Context, userID int64, term string) (*models.UserBannedTerm, error)
	DeleteUserBannedTerm(ctx context.Context, userID int64, termID int64) error
}

// StatsStore is the subset of store.Store used by the stats handlers.
type StatsStore interface {
	GetUserJobStats(ctx context.Context, userID int64) (store.UserJobStats, error)
	GetJobsPerWeek(ctx context.Context, userID int64, weeks int) ([]store.WeeklyJobCount, error)
}

// DriveTokenStore manages per-user encrypted Google Drive OAuth tokens.
type DriveTokenStore interface {
	UpsertGoogleDriveToken(ctx context.Context, userID int64, encryptedJSON string) error
	GetGoogleDriveToken(ctx context.Context, userID int64) (encryptedJSON string, err error)
	DeleteGoogleDriveToken(ctx context.Context, userID int64) error
}

// EmailSender is the interface consumed by auth handlers to send email.
// It is satisfied by *mailer.SMTPMailer and *mailer.NoopMailer.
type EmailSender interface {
	SendMail(ctx context.Context, to, subject, body string) error
}

// dashboardStatuses lists job statuses shown as tabs in the dashboard (triage view).
var dashboardStatuses = []models.JobStatus{
	models.StatusDiscovered, models.StatusNotified,
}

// approvedPageStatuses lists job statuses shown on the Approved Jobs page.
var approvedPageStatuses = []models.JobStatus{
	models.StatusApproved, models.StatusGenerating, models.StatusComplete, models.StatusFailed,
}

// Server holds the HTTP dependencies.
type Server struct {
	store           JobStore
	userStore       UserStore
	filterStore     FilterStore
	adminStore      admin.AdminStore
	statsStore      StatsStore
	driveTokenStore DriveTokenStore // nil if google_drive not configured
	sessionStore    sessions.Store
	oauthProviders  map[string]*oauth2.Config
	driveOAuthCfg   *oauth2.Config // nil if google_drive not configured
	baseURL         string
	mailer          EmailSender
	rateLimiters    sync.Map // map[string]*rate.Limiter — keyed by IP

	templates          *template.Template
	detailTmpl         *template.Template
	settingsTmpl       *template.Template
	profileTmpl        *template.Template
	onboardingTmpl     *template.Template
	loginTmpl          *template.Template
	registerTmpl       *template.Template
	verifyEmailTmpl    *template.Template
	forgotPasswordTmpl *template.Template
	resetPasswordTmpl  *template.Template
	approvedJobsTmpl   *template.Template
	rejectedJobsTmpl   *template.Template
	statsTmpl          *template.Template

	startTime      time.Time
	lastScrapeFn   func() time.Time // optional; returns last scrape time
	scrapeInterval time.Duration

	cfg *config.Config
}

// NewServer constructs a Server and parses embedded templates.
// All optional dependencies (UserStore, FilterStore, Config) are set to nil.
func NewServer(st JobStore) *Server {
	return NewServerWithConfig(st, nil, nil, nil)
}

// commaDollars formats an integer as a USD currency string, e.g. 150000 → "$150,000".
func commaDollars(n int) string {
	if n == 0 {
		return "—"
	}
	s := strconv.Itoa(n)
	out := make([]byte, 0, len(s)+(len(s)-1)/3)
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, byte(c))
	}
	return "$" + string(out)
}

// applicationStatusDate returns a human-readable string describing when the
// job reached its current application status.
func applicationStatusDate(job models.Job) string {
	switch models.ApplicationStatus(job.ApplicationStatus) {
	case models.AppStatusWon:
		if job.WonAt != nil {
			return "Won " + job.WonAt.Format("Jan 2, 2006")
		}
	case models.AppStatusLost:
		if job.LostAt != nil {
			return "Lost " + job.LostAt.Format("Jan 2, 2006")
		}
	case models.AppStatusInterviewing:
		if job.InterviewingAt != nil {
			return "Interviewing since " + job.InterviewingAt.Format("Jan 2, 2006")
		}
	case models.AppStatusApplied:
		if job.AppliedAt != nil {
			return "Applied " + job.AppliedAt.Format("Jan 2, 2006")
		}
	}
	return "—"
}

var tmplFuncs = template.FuncMap{
	"commaDollars":          commaDollars,
	"applicationStatusDate": applicationStatusDate,
	"toJSON": func(v any) (template.JS, error) {
		b, err := json.Marshal(v)
		if err != nil {
			return "", err
		}
		return template.JS(b), nil
	},
}

// NewServerWithConfig constructs a Server with config, auth, and filter store.
// Pass nil cfg or nil userStore/filterStore to disable settings/auth.
func NewServerWithConfig(st JobStore, us UserStore, fs FilterStore, cfg *config.Config) *Server {
	tmpl := template.Must(template.New("layout.html").Funcs(tmplFuncs).ParseFS(templateFS,
		"templates/layout.html",
		"templates/dashboard.html",
		"templates/partials/job_rows.html",
		"templates/partials/job_cards.html",
	))
	detail := template.Must(template.New("layout.html").Funcs(tmplFuncs).ParseFS(templateFS,
		"templates/layout.html",
		"templates/job_detail.html",
	))
	settings := template.Must(template.New("layout.html").Funcs(tmplFuncs).ParseFS(templateFS,
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
	registerTmpl := template.Must(template.ParseFS(templateFS, "templates/register.html"))
	verifyEmailTmpl := template.Must(template.ParseFS(templateFS, "templates/verify_email.html"))
	forgotPasswordTmpl := template.Must(template.ParseFS(templateFS, "templates/forgot_password.html"))
	resetPasswordTmpl := template.Must(template.ParseFS(templateFS, "templates/reset_password.html"))
	approvedJobsTmpl := template.Must(template.New("layout.html").Funcs(tmplFuncs).ParseFS(templateFS,
		"templates/layout.html",
		"templates/approved_jobs.html",
		"templates/partials/approved_job_rows.html",
	))
	rejectedJobsTmpl := template.Must(template.New("layout.html").Funcs(tmplFuncs).ParseFS(templateFS,
		"templates/layout.html",
		"templates/rejected_jobs.html",
		"templates/partials/job_rows.html",
	))
	statsTmpl := template.Must(template.New("layout.html").Funcs(tmplFuncs).ParseFS(templateFS,
		"templates/layout.html",
		"templates/stats.html",
	))

	srv := &Server{
		store:              st,
		userStore:          us,
		filterStore:        fs,
		templates:          tmpl,
		detailTmpl:         detail,
		settingsTmpl:       settings,
		profileTmpl:        profileTmpl,
		onboardingTmpl:     onboardingTmpl,
		loginTmpl:          loginTmpl,
		registerTmpl:       registerTmpl,
		verifyEmailTmpl:    verifyEmailTmpl,
		forgotPasswordTmpl: forgotPasswordTmpl,
		resetPasswordTmpl:  resetPasswordTmpl,
		approvedJobsTmpl:   approvedJobsTmpl,
		rejectedJobsTmpl:   rejectedJobsTmpl,
		statsTmpl:          statsTmpl,
		startTime:          time.Now(),
		cfg:                cfg,
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
		if cfg.Auth.OAuth.Enabled {
			srv.oauthProviders = oauthProviders(cfg.Auth, cfg.Server.BaseURL)
		}
		srv.baseURL = cfg.Server.BaseURL
	} else if cfg != nil {
		srv.baseURL = cfg.Server.BaseURL
	}

	// Wire the Drive OAuth config when credentials are present.
	if cfg != nil && cfg.GoogleDrive.ClientID != "" {
		srv.driveOAuthCfg = buildDriveOAuthConfig(cfg.GoogleDrive, srv.baseURL)
	}

	return srv
}

// WithDriveTokenStore sets the Drive token store used by the Google Drive
// export handlers. Call this after NewServerWithConfig when Drive export
// is configured.
func (s *Server) WithDriveTokenStore(ts DriveTokenStore) *Server {
	s.driveTokenStore = ts
	return s
}

// WithLastScrapeFn sets a function the server calls to obtain the last scrape
// time for the /health endpoint. Call this after NewServerWithConfig.
func (s *Server) WithLastScrapeFn(fn func() time.Time) *Server {
	s.lastScrapeFn = fn
	return s
}

// WithScrapeInterval sets the scrape interval so the dashboard can display a
// countdown to the next scheduled run.
func (s *Server) WithScrapeInterval(d time.Duration) *Server {
	s.scrapeInterval = d
	return s
}

// WithMailer sets the email sender used by auth handlers (registration, password
// reset, email verification). Call this after NewServerWithConfig.
func (s *Server) WithMailer(m EmailSender) *Server {
	s.mailer = m
	return s
}

// WithAdminStore sets the admin store used to power the /admin panel.
// Call this after NewServerWithConfig. The admin panel is only mounted
// when both this store and cfg.Admin.Password are non-empty.
func (s *Server) WithAdminStore(as admin.AdminStore) *Server {
	s.adminStore = as
	return s
}

// WithStatsStore sets the stats store used to power the /stats page.
// Call this after NewServerWithConfig.
func (s *Server) WithStatsStore(ss StatsStore) *Server {
	s.statsStore = ss
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

// WithTestDriveOAuthConfig replaces the Drive OAuth configuration.
// Intended for integration tests that need to point the Drive OAuth
// endpoints at a mock server.
func (s *Server) WithTestDriveOAuthConfig(cfg *oauth2.Config) *Server {
	s.driveOAuthCfg = cfg
	return s
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

		// Email/password auth routes.
		r.Get("/register", s.handleRegisterGet)
		r.Post("/register", s.handleRegisterPost)
		r.Post("/login", s.handleLoginPost)
		r.Get("/forgot-password", s.handleForgotPasswordGet)
		r.Post("/forgot-password", s.handleForgotPasswordPost)
		r.Get("/reset-password", s.handleResetPasswordGet)
		r.Post("/reset-password", s.handleResetPasswordPost)
		r.Get("/verify-email", s.handleVerifyEmail)
	}

	// Optional-auth routes — serve different content for logged-in vs. logged-out users.
	r.Group(func(r chi.Router) {
		if s.sessionStore != nil {
			r.Use(s.optionalAuth)
		}

		r.Get("/", s.handleDashboard)
		r.Get("/partials/job-table", s.handleJobTablePartial)
		r.Get("/partials/job-cards", s.handleJobCardsPartial)
		r.Get("/jobs/approved", s.handleApprovedJobs)
		r.Get("/jobs/rejected", s.handleRejectedJobs)
		r.Get("/partials/approved-job-table", s.handleApprovedJobTablePartial)
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

		// Google Drive export routes — only registered when Drive is configured.
		if s.driveOAuthCfg != nil && s.driveTokenStore != nil {
			r.Get("/output/{id}/resume.gdoc", s.handleSendResumeToGoogleDocs)
			r.Get("/output/{id}/cover_letter.gdoc", s.handleSendCoverToGoogleDocs)
			r.Get("/auth/google-drive", s.handleDriveOAuthStart)
			r.Get("/auth/google-drive/callback", s.handleDriveOAuthCallback)
		}

		r.Get("/settings", s.handleSettings)
		r.Post("/settings/resume", s.handleSaveResume)
		r.Post("/settings/ntfy", s.handleSaveNtfyTopic)
		r.Post("/settings/filters", s.handleAddFilter)
		r.Post("/settings/filters/remove", s.handleRemoveFilter)
		r.Post("/settings/banned-terms", s.handleAddBannedTerm)
		r.Post("/settings/banned-terms/remove", s.handleRemoveBannedTerm)

		r.Get("/onboarding", s.handleOnboardingGet)
		r.Post("/onboarding", s.handleOnboardingPost)

		r.Get("/profile", s.handleProfileGet)
		r.Post("/profile", s.handleProfileSave)

		r.Post("/logout", s.handleLogout)

		r.Get("/stats", s.handleStats)

		r.Route("/api/jobs", func(r chi.Router) {
			r.Get("/", s.handleListJobs)
			r.Get("/{id}", s.handleGetJob)
			r.Post("/{id}/approve", s.handleApproveJob)
			r.Post("/{id}/reject", s.handleRejectJob)
			r.Post("/{id}/retry", s.handleRetryJob)
			r.Post("/{id}/application-status", s.handleSetApplicationStatus)
		})

		// Google Drive OAuth routes — only registered when Drive credentials
		// and a token store are both configured.
		if s.driveOAuthCfg != nil && s.driveTokenStore != nil {
			r.Get("/auth/google-drive", s.handleDriveOAuthStart)
			r.Get("/auth/google-drive/callback", s.handleDriveOAuthCallback)
		}
	})

	// Admin panel — mounted at /admin, protected by HTTP Basic Auth.
	// Only registered when both the admin store and admin password are configured.
	if s.cfg != nil && s.adminStore != nil && s.cfg.Admin.Password != "" {
		adminH := admin.New(s.adminStore, s.cfg.Admin.Password)
		r.Mount("/admin", adminH.Routes())
	}

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
	CardDeck     jobRowsData
	Statuses     []models.JobStatus
	ActiveStatus string
	Search       string
	Sort         string
	Order        string
	Columns      []columnDef
	CSRFToken    string
	User         *models.User
	NextScrapeAt time.Time
}

type jobRowsData struct {
	Jobs      []models.Job
	CSRFToken string
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
	user := UserFromContext(r.Context())
	var userID int64
	if user != nil {
		userID = user.ID
	}
	var bannedTermStrings []string
	if s.filterStore != nil && user != nil {
		bannedTerms, err := s.filterStore.ListUserBannedTerms(r.Context(), userID)
		if err != nil {
			slog.Warn("failed to load banned terms for dashboard", "error", err, "user_id", userID)
		}
		bannedTermStrings = bannedTermsToStrings(bannedTerms)
	}
	f := store.ListJobsFilter{
		Status:          models.JobStatus(q.Get("status")),
		ExcludeStatuses: []models.JobStatus{models.StatusRejected},
		Search:          q.Get("q"),
		Sort:            sort,
		Order:           order,
		BannedTerms:     bannedTermStrings,
	}
	jobs, err := s.store.ListJobs(r.Context(), userID, f)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list jobs")
		return
	}
	if jobs == nil {
		jobs = []models.Job{}
	}
	cardF := store.ListJobsFilter{
		Sort:        sort,
		Order:       order,
		BannedTerms: bannedTermStrings,
		ExcludeStatuses: []models.JobStatus{
			models.StatusRejected,
			models.StatusApproved,
			models.StatusGenerating,
			models.StatusComplete,
			models.StatusFailed,
		},
	}
	cardJobs, err := s.store.ListJobs(r.Context(), userID, cardF)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list jobs")
		return
	}
	if cardJobs == nil {
		cardJobs = []models.Job{}
	}
	var nextScrape time.Time
	if s.lastScrapeFn != nil && s.scrapeInterval > 0 {
		if last := s.lastScrapeFn(); !last.IsZero() {
			nextScrape = last.Add(s.scrapeInterval)
		}
	}
	csrfToken := csrf.Token(r)
	data := dashboardData{
		Jobs:         jobs,
		CardDeck:     jobRowsData{Jobs: cardJobs, CSRFToken: csrfToken},
		Statuses:     dashboardStatuses,
		ActiveStatus: string(f.Status),
		Search:       f.Search,
		Sort:         sort,
		Order:        order,
		Columns:      buildColumns(sort, order),
		CSRFToken:    csrfToken,
		User:         user,
		NextScrapeAt: nextScrape,
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
	var bannedTermStrings []string
	if s.filterStore != nil {
		bannedTerms, err := s.filterStore.ListUserBannedTerms(r.Context(), user.ID)
		if err != nil {
			slog.Warn("failed to load banned terms", "error", err)
		} else {
			bannedTermStrings = bannedTermsToStrings(bannedTerms)
		}
	}
	q := r.URL.Query()
	sort, order := parseSortParams(q)
	f := store.ListJobsFilter{
		Status:          models.JobStatus(q.Get("status")),
		ExcludeStatuses: []models.JobStatus{models.StatusRejected},
		Search:          q.Get("q"),
		Sort:            sort,
		Order:           order,
		BannedTerms:     bannedTermStrings,
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
	if err := s.templates.ExecuteTemplate(w, "job_rows", jobRowsData{Jobs: jobs, CSRFToken: csrf.Token(r)}); err != nil {
		slog.Error("template render error", "error", err)
	}
}

func (s *Server) handleJobCardsPartial(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		return
	}
	var bannedTermStrings []string
	if s.filterStore != nil {
		bannedTerms, err := s.filterStore.ListUserBannedTerms(r.Context(), user.ID)
		if err != nil {
			slog.Warn("failed to load banned terms", "error", err)
		} else {
			bannedTermStrings = bannedTermsToStrings(bannedTerms)
		}
	}
	q := r.URL.Query()
	sort, order := parseSortParams(q)
	f := store.ListJobsFilter{
		Sort:        sort,
		Order:       order,
		BannedTerms: bannedTermStrings,
		ExcludeStatuses: []models.JobStatus{
			models.StatusRejected,
			models.StatusApproved,
			models.StatusGenerating,
			models.StatusComplete,
			models.StatusFailed,
		},
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
	if err := s.templates.ExecuteTemplate(w, "job_cards", jobRowsData{Jobs: jobs, CSRFToken: csrf.Token(r)}); err != nil {
		slog.Error("template render error", "error", err)
	}
}

// approvedPageData is the template data for the /jobs/approved page.
type approvedPageData struct {
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

// rejectedPageData is the template data for the /jobs/rejected page.
type rejectedPageData struct {
	Jobs      []models.Job
	Search    string
	Sort      string
	Order     string
	Columns   []columnDef
	CSRFToken string
	User      *models.User
}

func (s *Server) handleApprovedJobs(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	sort, order := parseSortParams(q)
	f := store.ListJobsFilter{
		Status: models.JobStatus(q.Get("status")),
		Search: q.Get("q"),
		Sort:   sort,
		Order:  order,
	}
	// When no status is specified, list all approved-pipeline statuses by
	// running multiple queries and merging. For simplicity, we pass the
	// filter as-is and let the store return jobs for the selected status.
	// The template renders tab links for all approvedPageStatuses.
	user := UserFromContext(r.Context())
	var userID int64
	if user != nil {
		userID = user.ID
	}
	// If no specific status requested, default to showing all approved-pipeline jobs.
	if f.Status == "" {
		var allJobs []models.Job
		for _, st := range approvedPageStatuses {
			jf := f
			jf.Status = st
			jobs, err := s.store.ListJobs(r.Context(), userID, jf)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "failed to list jobs")
				return
			}
			allJobs = append(allJobs, jobs...)
		}
		if allJobs == nil {
			allJobs = []models.Job{}
		}
		data := approvedPageData{
			Jobs:         allJobs,
			Statuses:     approvedPageStatuses,
			ActiveStatus: "",
			Search:       f.Search,
			Sort:         sort,
			Order:        order,
			Columns:      buildColumns(sort, order),
			CSRFToken:    csrf.Token(r),
			User:         user,
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := s.approvedJobsTmpl.ExecuteTemplate(w, "layout.html", data); err != nil {
			slog.Error("template render error", "error", err)
		}
		return
	}
	jobs, err := s.store.ListJobs(r.Context(), userID, f)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list jobs")
		return
	}
	if jobs == nil {
		jobs = []models.Job{}
	}
	data := approvedPageData{
		Jobs:         jobs,
		Statuses:     approvedPageStatuses,
		ActiveStatus: string(f.Status),
		Search:       f.Search,
		Sort:         sort,
		Order:        order,
		Columns:      buildColumns(sort, order),
		CSRFToken:    csrf.Token(r),
		User:         user,
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.approvedJobsTmpl.ExecuteTemplate(w, "layout.html", data); err != nil {
		slog.Error("template render error", "error", err)
	}
}

func (s *Server) handleRejectedJobs(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	sort, order := parseSortParams(q)
	f := store.ListJobsFilter{
		Status: models.StatusRejected,
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
	data := rejectedPageData{
		Jobs:      jobs,
		Search:    f.Search,
		Sort:      sort,
		Order:     order,
		Columns:   buildColumns(sort, order),
		CSRFToken: csrf.Token(r),
		User:      user,
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.rejectedJobsTmpl.ExecuteTemplate(w, "layout.html", data); err != nil {
		slog.Error("template render error", "error", err)
	}
}

func (s *Server) handleApprovedJobTablePartial(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
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
	var jobs []models.Job
	if f.Status == "" {
		for _, st := range approvedPageStatuses {
			jf := f
			jf.Status = st
			js, err := s.store.ListJobs(r.Context(), user.ID, jf)
			if err != nil {
				http.Error(w, "failed to list jobs", http.StatusInternalServerError)
				return
			}
			jobs = append(jobs, js...)
		}
	} else {
		var err error
		jobs, err = s.store.ListJobs(r.Context(), user.ID, f)
		if err != nil {
			http.Error(w, "failed to list jobs", http.StatusInternalServerError)
			return
		}
	}
	if jobs == nil {
		jobs = []models.Job{}
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.approvedJobsTmpl.ExecuteTemplate(w, "approved_job_rows", jobs); err != nil {
		slog.Error("template render error", "error", err)
	}
}

// validApplicationStatuses is the set of statuses accepted by
// handleSetApplicationStatus.
var validApplicationStatuses = map[models.ApplicationStatus]bool{
	models.AppStatusApplied:      true,
	models.AppStatusInterviewing: true,
	models.AppStatusLost:         true,
	models.AppStatusWon:          true,
}

func (s *Server) handleSetApplicationStatus(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	user := UserFromContext(r.Context())
	var userID int64
	if user != nil {
		userID = user.ID
	}

	if err := r.ParseForm(); err != nil {
		writeError(w, http.StatusBadRequest, "invalid form data")
		return
	}
	rawStatus := r.FormValue("application_status")
	appStatus := models.ApplicationStatus(rawStatus)
	if !validApplicationStatuses[appStatus] {
		writeError(w, http.StatusBadRequest, "invalid application_status: "+rawStatus)
		return
	}

	// Verify the job exists, belongs to the user, and is in an approved-pipeline stage.
	job, err := s.store.GetJob(r.Context(), userID, id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, "job not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get job")
		return
	}
	if !pipelineStatusAllowed(job.Status) {
		writeError(w, http.StatusForbidden, "job is not in an approved-pipeline stage")
		return
	}

	if err := s.store.UpdateApplicationStatus(r.Context(), userID, id, appStatus); err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, "job not found")
			return
		}
		if strings.Contains(err.Error(), "not in pipeline") || strings.Contains(err.Error(), "invalid") {
			writeError(w, http.StatusForbidden, "cannot update application status: "+err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to update application status")
		return
	}

	// Re-fetch the updated job to render the replacement row fragment.
	job, err = s.store.GetJob(r.Context(), userID, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to reload job")
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.approvedJobsTmpl.ExecuteTemplate(w, "approved_job_rows", []models.Job{*job}); err != nil {
		slog.Error("template render error", "error", err)
	}
}

// pipelineStatusAllowed returns true if the job status is in the
// approved-pipeline stages (approved, generating, complete, failed).
func pipelineStatusAllowed(s models.JobStatus) bool {
	switch s {
	case models.StatusApproved, models.StatusGenerating, models.StatusComplete, models.StatusFailed:
		return true
	}
	return false
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
		if strings.Contains(err.Error(), "invalid transition") {
			writeError(w, http.StatusConflict, "job cannot be approved from status "+string(job.Status))
			return
		}
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

func (s *Server) handleRetryJob(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	user := UserFromContext(r.Context())
	var userID int64
	if user != nil {
		userID = user.ID
	}
	// Guard: fetch job first for a clean 404 vs 409 response.
	job, err := s.store.GetJob(r.Context(), userID, id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, "job not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get job")
		return
	}
	if job.Status != models.StatusFailed {
		writeError(w, http.StatusConflict, "job cannot be retried from status "+string(job.Status))
		return
	}
	if err := s.store.RetryJob(r.Context(), userID, id); err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "not in failed") || strings.Contains(err.Error(), "not failed") {
			writeError(w, http.StatusConflict, "retry failed: "+err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to retry job")
		return
	}
	// For HTMX requests from the detail page, trigger a full page reload.
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", fmt.Sprintf("/jobs/%d", id))
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if strings.Contains(r.Header.Get("Accept"), "text/html") {
		http.Redirect(w, r, fmt.Sprintf("/jobs/%d", id), http.StatusSeeOther)
		return
	}
	job.Status = models.StatusApproved
	job.ErrorMsg = ""
	writeJSON(w, http.StatusOK, job)
}

// respondJobAction returns HTML for HTMX requests and JSON for API clients.
func (s *Server) respondJobAction(w http.ResponseWriter, r *http.Request, job *models.Job) {
	if r.Header.Get("HX-Request") == "true" {
		// Parse filter/sort state from the URL the client is currently viewing.
		currentURL := r.Header.Get("HX-Current-URL")
		var q url.Values
		if u, err := url.Parse(currentURL); err == nil {
			q = u.Query()
		} else {
			q = url.Values{}
		}
		sort, order := parseSortParams(q)
		var bannedTermStrings []string
		if s.filterStore != nil {
			bannedTerms, err := s.filterStore.ListUserBannedTerms(r.Context(), job.UserID)
			if err != nil {
				slog.Warn("failed to load banned terms", "error", err)
			} else {
				bannedTermStrings = bannedTermsToStrings(bannedTerms)
			}
		}
		f := store.ListJobsFilter{
			Status:      models.JobStatus(q.Get("status")),
			Search:      q.Get("q"),
			Sort:        sort,
			Order:       order,
			BannedTerms: bannedTermStrings,
		}
		userID := job.UserID
		jobs, err := s.store.ListJobs(r.Context(), userID, f)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list jobs")
			return
		}
		if jobs == nil {
			jobs = []models.Job{}
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		data := jobRowsData{Jobs: jobs, CSRFToken: csrf.Token(r)}

		if r.Header.Get("HX-Target") == "job-card-deck" {
			f.Status = ""
			f.ExcludeStatuses = []models.JobStatus{
				models.StatusRejected,
				models.StatusApproved,
				models.StatusGenerating,
				models.StatusComplete,
				models.StatusFailed,
			}
			// Re-fetch jobs with exclude filter applied for card deck.
			jobs, err = s.store.ListJobs(r.Context(), userID, f)
			if err != nil {
				http.Error(w, "failed to list jobs", http.StatusInternalServerError)
				return
			}
			if jobs == nil {
				jobs = []models.Job{}
			}
			data = jobRowsData{Jobs: jobs, CSRFToken: csrf.Token(r)}
			if err := s.templates.ExecuteTemplate(w, "job_cards", data); err != nil {
				slog.Error("template render error", "error", err)
			}
			return
		}
		if err := s.templates.ExecuteTemplate(w, "job_rows", data); err != nil {
			slog.Error("template render error", "error", err)
		}
		return
	}

	if strings.Contains(r.Header.Get("Accept"), "text/html") {
		back := r.Referer()
		if back == "" {
			back = "/"
		}
		http.Redirect(w, r, back, http.StatusSeeOther)
		return
	}

	writeJSON(w, http.StatusOK, job)
}

// ─── settings handlers ────────────────────────────────────────────────────────

type settingsData struct {
	Filters     []models.UserSearchFilter
	BannedTerms []models.UserBannedTerm
	Resume      string
	NtfyTopic   string
	Saved       bool
	CSRFToken   string
	User        *models.User
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

	bannedTerms, err := s.filterStore.ListUserBannedTerms(r.Context(), userID)
	if err != nil {
		slog.Error("failed to list banned terms", "error", err)
		http.Error(w, "failed to load settings", http.StatusInternalServerError)
		return
	}
	if bannedTerms == nil {
		bannedTerms = []models.UserBannedTerm{}
	}

	resume := ""
	ntfyTopic := ""
	if user != nil {
		resume = user.ResumeMarkdown
		ntfyTopic = user.NtfyTopic
	}

	data := settingsData{
		Filters:     filters,
		BannedTerms: bannedTerms,
		Resume:      resume,
		NtfyTopic:   ntfyTopic,
		Saved:       r.URL.Query().Get("saved") == "1",
		CSRFToken:   csrf.Token(r),
		User:        user,
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

// bannedTermsToStrings converts a slice of UserBannedTerm to a plain string slice
// for use in store.ListJobsFilter.BannedTerms.
func bannedTermsToStrings(terms []models.UserBannedTerm) []string {
	out := make([]string, len(terms))
	for i, t := range terms {
		out[i] = t.Term
	}
	return out
}

func (s *Server) handleAddBannedTerm(w http.ResponseWriter, r *http.Request) {
	if s.filterStore == nil {
		http.Error(w, "settings not configured", http.StatusServiceUnavailable)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	term := strings.TrimSpace(r.FormValue("term"))
	if term == "" {
		http.Error(w, "term is required", http.StatusBadRequest)
		return
	}
	user := UserFromContext(r.Context())
	var userID int64
	if user != nil {
		userID = user.ID
	}

	_, err := s.filterStore.CreateUserBannedTerm(r.Context(), userID, term)
	if err != nil {
		if errors.Is(err, store.ErrDuplicateBannedTerm) {
			// Duplicate: redirect silently so the UI remains usable.
			http.Redirect(w, r, "/settings?saved=1", http.StatusSeeOther)
			return
		}
		slog.Error("failed to create banned term", "error", err, "user_id", userID)
		http.Error(w, "failed to save banned term", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/settings?saved=1", http.StatusSeeOther)
}

func (s *Server) handleRemoveBannedTerm(w http.ResponseWriter, r *http.Request) {
	if s.filterStore == nil {
		http.Error(w, "settings not configured", http.StatusServiceUnavailable)
		return
	}
	termID, err := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid term id", http.StatusBadRequest)
		return
	}
	user := UserFromContext(r.Context())
	var userID int64
	if user != nil {
		userID = user.ID
	}

	if err := s.filterStore.DeleteUserBannedTerm(r.Context(), userID, termID); err != nil {
		slog.Error("failed to delete banned term", "error", err, "user_id", userID, "term_id", termID)
		http.Error(w, "failed to remove banned term", http.StatusInternalServerError)
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
		Job:                job,
		ResumeHTML:         template.HTML(job.ResumeHTML), //nolint:gosec // HTML is generated by Claude, not user input
		CoverHTML:          template.HTML(job.CoverHTML),  //nolint:gosec // HTML is generated by Claude, not user input
		CSRFToken:          csrf.Token(r),
		User:               user,
		GoogleDriveEnabled: s.driveOAuthCfg != nil,
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

// statsData is the template data for the /stats page.
type statsData struct {
	Stats       store.UserJobStats
	WeeklyTrend []store.WeeklyJobCount
	MaxWeekly   int
	SankeyLinks []store.SankeyLink
	CSRFToken   string
	User        *models.User
}

// handleStats renders the /stats page for an authenticated user.
func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	ctx := r.Context()

	stats, err := s.statsStore.GetUserJobStats(ctx, user.ID)
	if err != nil {
		slog.Error("handleStats: get user job stats", "error", err, "user_id", user.ID)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	rawWeekly, err := s.statsStore.GetJobsPerWeek(ctx, user.ID, 12)
	if err != nil {
		slog.Error("handleStats: get jobs per week", "error", err, "user_id", user.ID)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	// Build a map from week-start (truncated to day) → count so we can backfill
	// missing weeks.
	weekMap := make(map[time.Time]int, len(rawWeekly))
	for _, wc := range rawWeekly {
		key := wc.WeekStart.Truncate(24 * time.Hour)
		weekMap[key] = wc.Count
	}

	// Determine the Monday of the current week, then walk back 11 weeks so we
	// always emit exactly 12 entries.
	now := time.Now().UTC()
	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7 // Sunday → 7 so Monday is always day 1
	}
	currentMonday := now.AddDate(0, 0, -(weekday - 1))
	currentMonday = time.Date(currentMonday.Year(), currentMonday.Month(), currentMonday.Day(), 0, 0, 0, 0, time.UTC)
	startMonday := currentMonday.AddDate(0, 0, -11*7)

	weekly := make([]store.WeeklyJobCount, 0, 12)
	maxWeekly := 0
	for i := 0; i < 12; i++ {
		ws := startMonday.AddDate(0, 0, i*7)
		count := weekMap[ws]
		weekly = append(weekly, store.WeeklyJobCount{WeekStart: ws, Count: count})
		if count > maxWeekly {
			maxWeekly = count
		}
	}
	if maxWeekly == 0 {
		maxWeekly = 1
	}

	csrfToken := ""
	if s.sessionStore != nil {
		csrfToken = csrf.Token(r)
	}

	data := statsData{
		Stats:       stats,
		WeeklyTrend: weekly,
		MaxWeekly:   maxWeekly,
		SankeyLinks: store.BuildSankeyLinks(stats),
		CSRFToken:   csrfToken,
		User:        user,
	}

	if err := s.statsTmpl.ExecuteTemplate(w, "layout.html", data); err != nil {
		slog.Error("handleStats: render template", "error", err)
	}
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
