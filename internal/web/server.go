// Package web provides the HTTP API server for jobhuntr.
package web

import (
	"context"
	"embed"
	"encoding/json"
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"gopkg.in/yaml.v3"

	"github.com/whinchman/jobhuntr/internal/config"
	"github.com/whinchman/jobhuntr/internal/models"
	"github.com/whinchman/jobhuntr/internal/store"
)

// jobDetailData is the template data for the job detail page.
type jobDetailData struct {
	Job        *models.Job
	ResumeHTML template.HTML
	CoverHTML  template.HTML
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
	GetJob(ctx context.Context, id int64) (*models.Job, error)
	ListJobs(ctx context.Context, f store.ListJobsFilter) ([]models.Job, error)
	UpdateJobStatus(ctx context.Context, id int64, status models.JobStatus) error
}

// allStatuses lists job statuses shown as tabs in the dashboard.
var allStatuses = []models.JobStatus{
	models.StatusDiscovered, models.StatusNotified, models.StatusApproved,
	models.StatusGenerating, models.StatusComplete, models.StatusFailed, models.StatusRejected,
}

// Server holds the HTTP dependencies.
type Server struct {
	store        JobStore
	templates    *template.Template
	detailTmpl   *template.Template
	settingsTmpl *template.Template

	startTime    time.Time
	lastScrapeFn func() time.Time // optional; returns last scrape time

	mu         sync.Mutex // guards cfg, configPath, resumePath
	cfg        *config.Config
	configPath string
	resumePath string
}

// NewServer constructs a Server and parses embedded templates.
// cfg, configPath and resumePath may be zero/nil when settings are not needed.
func NewServer(st JobStore) *Server {
	return NewServerWithConfig(st, nil, "", "")
}

// NewServerWithConfig constructs a Server with config and file paths for the
// settings page. Pass nil cfg or empty paths to disable settings persistence.
func NewServerWithConfig(st JobStore, cfg *config.Config, configPath, resumePath string) *Server {
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
	return &Server{
		store:        st,
		templates:    tmpl,
		detailTmpl:   detail,
		settingsTmpl: settings,
		startTime:    time.Now(),
		cfg:          cfg,
		configPath:   configPath,
		resumePath:   resumePath,
	}
}

// WithLastScrapeFn sets a function the server calls to obtain the last scrape
// time for the /health endpoint. Call this after NewServerWithConfig.
func (s *Server) WithLastScrapeFn(fn func() time.Time) *Server {
	s.lastScrapeFn = fn
	return s
}

// Handler builds and returns the chi router.
func (s *Server) Handler() http.Handler {
	r := chi.NewRouter()
	r.Use(slogRequestLogger)
	r.Use(chimw.Recoverer)

	r.Get("/", s.handleDashboard)
	r.Get("/partials/job-table", s.handleJobTablePartial)
	r.Get("/health", s.handleHealth)

	r.Get("/jobs/{id}", s.handleJobDetail)
	r.Get("/output/{id}/resume.pdf", s.handleDownloadResume)
	r.Get("/output/{id}/cover_letter.pdf", s.handleDownloadCover)

	r.Get("/settings", s.handleSettings)
	r.Post("/settings/resume", s.handleSaveResume)
	r.Post("/settings/filters", s.handleAddFilter)
	r.Post("/settings/filters/remove", s.handleRemoveFilter)

	r.Route("/api/jobs", func(r chi.Router) {
		r.Get("/", s.handleListJobs)
		r.Get("/{id}", s.handleGetJob)
		r.Post("/{id}/approve", s.handleApproveJob)
		r.Post("/{id}/reject", s.handleRejectJob)
	})

	return r
}

type dashboardData struct {
	Jobs         []models.Job
	Statuses     []models.JobStatus
	ActiveStatus string
	Search       string
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := store.ListJobsFilter{
		Status: models.JobStatus(q.Get("status")),
		Search: q.Get("q"),
	}
	jobs, err := s.store.ListJobs(r.Context(), f)
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
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.templates.ExecuteTemplate(w, "layout.html", data); err != nil {
		slog.Error("template render error", "error", err)
	}
}

func (s *Server) handleJobTablePartial(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := store.ListJobsFilter{
		Status: models.JobStatus(q.Get("status")),
		Search: q.Get("q"),
	}
	jobs, err := s.store.ListJobs(r.Context(), f)
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

	jobs, err := s.store.ListJobs(r.Context(), f)
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
	job, err := s.store.GetJob(r.Context(), id)
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

	job, err := s.store.GetJob(r.Context(), id)
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

	if err := s.store.UpdateJobStatus(r.Context(), id, models.StatusApproved); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update job status")
		return
	}
	job.Status = models.StatusApproved
	writeJSON(w, http.StatusOK, job)
}

func (s *Server) handleRejectJob(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}

	job, err := s.store.GetJob(r.Context(), id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, "job not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get job")
		return
	}

	if err := s.store.UpdateJobStatus(r.Context(), id, models.StatusRejected); err != nil {
		if strings.Contains(err.Error(), "invalid transition") {
			writeError(w, http.StatusConflict, "job cannot be rejected from status "+string(job.Status))
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to update job status")
		return
	}
	job.Status = models.StatusRejected
	writeJSON(w, http.StatusOK, job)
}

// ─── settings handlers ────────────────────────────────────────────────────────

type settingsData struct {
	Filters []config.SearchFilter
	Resume  string
	Saved   bool
}

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	filters := s.filtersSnapshot()
	s.mu.Unlock()

	resume := ""
	if s.resumePath != "" {
		if b, err := os.ReadFile(s.resumePath); err == nil {
			resume = string(b)
		}
	}

	data := settingsData{
		Filters: filters,
		Resume:  resume,
		Saved:   r.URL.Query().Get("saved") == "1",
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.settingsTmpl.ExecuteTemplate(w, "layout.html", data); err != nil {
		slog.Error("settings template render error", "error", err)
	}
}

func (s *Server) handleSaveResume(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	content := r.FormValue("resume")

	if s.resumePath == "" {
		http.Error(w, "resume path not configured", http.StatusInternalServerError)
		return
	}
	if err := os.WriteFile(s.resumePath, []byte(content), 0o644); err != nil {
		slog.Error("failed to write resume", "error", err)
		http.Error(w, "failed to save resume", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/settings?saved=1", http.StatusSeeOther)
}

func (s *Server) handleAddFilter(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	minSalary := 0
	if raw := r.FormValue("min_salary"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			minSalary = n
		}
	}
	f := config.SearchFilter{
		Keywords:  r.FormValue("keywords"),
		Location:  r.FormValue("location"),
		MinSalary: minSalary,
	}

	s.mu.Lock()
	if s.cfg != nil {
		s.cfg.SearchFilters = append(s.cfg.SearchFilters, f)
	}
	err := s.writeConfig()
	s.mu.Unlock()

	if err != nil {
		slog.Error("failed to write config", "error", err)
		http.Error(w, "failed to save filter", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/settings?saved=1", http.StatusSeeOther)
}

func (s *Server) handleRemoveFilter(w http.ResponseWriter, r *http.Request) {
	idx, err := strconv.Atoi(r.URL.Query().Get("index"))
	if err != nil {
		http.Error(w, "invalid index", http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	if s.cfg == nil || idx < 0 || idx >= len(s.cfg.SearchFilters) {
		s.mu.Unlock()
		http.Error(w, "index out of range", http.StatusBadRequest)
		return
	}
	s.cfg.SearchFilters = append(s.cfg.SearchFilters[:idx], s.cfg.SearchFilters[idx+1:]...)
	writeErr := s.writeConfig()
	s.mu.Unlock()

	if writeErr != nil {
		slog.Error("failed to write config", "error", writeErr)
		http.Error(w, "failed to save config", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/settings?saved=1", http.StatusSeeOther)
}

// filtersSnapshot returns a copy of the current search filters. Caller must
// hold s.mu.
func (s *Server) filtersSnapshot() []config.SearchFilter {
	if s.cfg == nil {
		return nil
	}
	out := make([]config.SearchFilter, len(s.cfg.SearchFilters))
	copy(out, s.cfg.SearchFilters)
	return out
}

// writeConfig marshals s.cfg to YAML and writes it to s.configPath.
// Caller must hold s.mu.
func (s *Server) writeConfig() error {
	if s.cfg == nil || s.configPath == "" {
		return nil
	}
	data, err := yaml.Marshal(s.cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(s.configPath, data, 0o644)
}

// ─── detail / download handlers ───────────────────────────────────────────────

func (s *Server) handleJobDetail(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	job, err := s.store.GetJob(r.Context(), id)
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
	job, err := s.store.GetJob(r.Context(), id)
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
	job, err := s.store.GetJob(r.Context(), id)
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
