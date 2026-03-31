// Package web provides the HTTP API server for jobhuntr.
package web

import (
	"context"
	"embed"
	"encoding/json"
	"html/template"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/whinchman/jobhuntr/internal/models"
	"github.com/whinchman/jobhuntr/internal/store"
)

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
	store     JobStore
	templates *template.Template
}

// NewServer constructs a Server and parses embedded templates.
func NewServer(st JobStore) *Server {
	tmpl := template.Must(template.ParseFS(templateFS,
		"templates/layout.html",
		"templates/dashboard.html",
		"templates/partials/job_rows.html",
	))
	return &Server{store: st, templates: tmpl}
}

// Handler builds and returns the chi router.
func (s *Server) Handler() http.Handler {
	r := chi.NewRouter()
	r.Use(slogRequestLogger)
	r.Use(chimw.Recoverer)

	r.Get("/", s.handleDashboard)
	r.Get("/partials/job-table", s.handleJobTablePartial)
	r.Get("/health", s.handleHealth)

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
