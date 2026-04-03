// Package store provides PostgreSQL-backed persistence for jobhuntr.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/whinchman/jobhuntr/internal/models"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// Store wraps a PostgreSQL database connection.
type Store struct {
	db *sql.DB
}

// ScrapeRun records the outcome of one scheduled scrape execution.
type ScrapeRun struct {
	ID             int64
	Source         string
	FilterKeywords string
	JobsFound      int
	JobsNew        int
	StartedAt      time.Time
	FinishedAt     time.Time
	Error          string
}

// ListJobsFilter controls which jobs ListJobs returns.
type ListJobsFilter struct {
	Status models.JobStatus
	Search string
	Limit  int
	Offset int
	Sort   string // column name (must be validated by caller)
	Order  string // "asc" or "desc" (must be validated by caller)
}

const schema = `
CREATE TABLE IF NOT EXISTS jobs (
    id               BIGSERIAL PRIMARY KEY,
    external_id      TEXT    NOT NULL,
    source           TEXT    NOT NULL,
    title            TEXT    NOT NULL DEFAULT '',
    company          TEXT    NOT NULL DEFAULT '',
    location         TEXT    NOT NULL DEFAULT '',
    description      TEXT    NOT NULL DEFAULT '',
    salary           TEXT    NOT NULL DEFAULT '',
    apply_url        TEXT    NOT NULL DEFAULT '',
    status           TEXT    NOT NULL DEFAULT 'discovered'
                     CHECK(status IN ('discovered','notified','approved','rejected','generating','complete','failed')),
    summary          TEXT    NOT NULL DEFAULT '',
    extracted_salary TEXT    NOT NULL DEFAULT '',
    resume_html      TEXT    NOT NULL DEFAULT '',
    cover_html       TEXT    NOT NULL DEFAULT '',
    resume_pdf       TEXT    NOT NULL DEFAULT '',
    cover_pdf        TEXT    NOT NULL DEFAULT '',
    error_msg        TEXT    NOT NULL DEFAULT '',
    discovered_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_jobs_status       ON jobs(status);
CREATE INDEX IF NOT EXISTS idx_jobs_discovered   ON jobs(discovered_at);

CREATE TABLE IF NOT EXISTS scrape_runs (
    id              BIGSERIAL PRIMARY KEY,
    source          TEXT    NOT NULL DEFAULT '',
    filter_keywords TEXT    NOT NULL DEFAULT '',
    jobs_found      INTEGER NOT NULL DEFAULT 0,
    jobs_new        INTEGER NOT NULL DEFAULT 0,
    started_at      TIMESTAMPTZ NOT NULL,
    finished_at     TIMESTAMPTZ NOT NULL,
    error           TEXT    NOT NULL DEFAULT ''
);
`

// validTransitions maps each status to the set of statuses it may transition to.
var validTransitions = map[models.JobStatus][]models.JobStatus{
	models.StatusDiscovered: {models.StatusNotified, models.StatusRejected},
	models.StatusNotified:   {models.StatusApproved, models.StatusRejected},
	models.StatusApproved:   {models.StatusGenerating},
	models.StatusGenerating: {models.StatusComplete, models.StatusFailed},
	models.StatusFailed:     {models.StatusGenerating},
}

// Open connects to the PostgreSQL database at dsn, applies the baseline schema,
// and runs any pending migrations.
func Open(dsn string) (*Store, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("store: open db: %w", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("store: ping db: %w", err)
	}

	// Apply baseline schema (idempotent: uses IF NOT EXISTS).
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("store: apply baseline schema: %w", err)
	}

	s := &Store{db: db}

	// Apply numbered migrations.
	if err := s.Migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("store: migrate: %w", err)
	}

	return s, nil
}

// Close closes the underlying database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// CreateJob inserts a job record, ignoring duplicates (UNIQUE user_id+external_id+source).
// Returns inserted=true if a new row was created, false if it already existed.
func (s *Store) CreateJob(ctx context.Context, userID int64, job *models.Job) (bool, error) {
	now := time.Now().UTC()
	var id int64
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO jobs
		  (user_id, external_id, source, title, company, location, description, salary, apply_url, status, discovered_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT (user_id, external_id, source) DO NOTHING
		RETURNING id`,
		userID, job.ExternalID, job.Source, job.Title, job.Company, job.Location,
		job.Description, job.Salary, job.ApplyURL, string(job.Status),
		now.Format(time.RFC3339), now.Format(time.RFC3339),
	).Scan(&id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// ON CONFLICT DO NOTHING — row already existed.
			return false, nil
		}
		return false, fmt.Errorf("store: create job: %w", err)
	}
	job.ID = id
	job.UserID = userID
	job.DiscoveredAt = now
	job.UpdatedAt = now
	return true, nil
}

// GetJob retrieves a single job by its primary key. When userID is 0 the
// query is not scoped by user (used by background workers). When userID > 0
// the job must belong to that user.
func (s *Store) GetJob(ctx context.Context, userID int64, id int64) (*models.Job, error) {
	q := `SELECT id, user_id, external_id, source, title, company, location, description, salary, apply_url,
	             status, summary, extracted_salary, resume_html, cover_html, resume_markdown, cover_markdown, resume_pdf, cover_pdf, error_msg,
	             discovered_at, updated_at
	      FROM jobs WHERE id = $1`
	args := []any{id}

	if userID != 0 {
		q += " AND user_id = $2"
		args = append(args, userID)
	}

	row := s.db.QueryRowContext(ctx, q, args...)
	job, err := scanJob(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("store: job %d not found", id)
		}
		return nil, fmt.Errorf("store: get job: %w", err)
	}
	return job, nil
}

// ListJobs returns jobs matching the given filter, ordered by discovered_at DESC.
// When userID is 0 the query is not scoped by user (used by background workers).
// When userID > 0 only that user's jobs are returned.
func (s *Store) ListJobs(ctx context.Context, userID int64, f ListJobsFilter) ([]models.Job, error) {
	var where []string
	var args []any
	argN := 1

	if userID != 0 {
		where = append(where, fmt.Sprintf("user_id = $%d", argN))
		args = append(args, userID)
		argN++
	}
	if f.Status != "" {
		where = append(where, fmt.Sprintf("status = $%d", argN))
		args = append(args, string(f.Status))
		argN++
	}
	if f.Search != "" {
		where = append(where, fmt.Sprintf("(title ILIKE $%d OR company ILIKE $%d OR description ILIKE $%d)", argN, argN+1, argN+2))
		like := "%" + f.Search + "%"
		args = append(args, like, like, like)
		argN += 3
	}

	q := "SELECT id, user_id, external_id, source, title, company, location, description, salary, apply_url, status, summary, extracted_salary, resume_html, cover_html, resume_markdown, cover_markdown, resume_pdf, cover_pdf, error_msg, discovered_at, updated_at FROM jobs"
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}
	sortCol := f.Sort
	if sortCol == "" {
		sortCol = "discovered_at"
	}
	sortDir := "DESC"
	if f.Order == "asc" {
		sortDir = "ASC"
	}
	q += " ORDER BY " + sortCol + " " + sortDir

	limit := f.Limit
	if limit <= 0 {
		limit = 100
	}
	q += fmt.Sprintf(" LIMIT %d OFFSET %d", limit, f.Offset)

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("store: list jobs: %w", err)
	}
	defer rows.Close()

	var jobs []models.Job
	for rows.Next() {
		job, err := scanJob(rows)
		if err != nil {
			return nil, fmt.Errorf("store: list jobs scan: %w", err)
		}
		jobs = append(jobs, *job)
	}
	return jobs, rows.Err()
}

// UpdateJobStatus transitions a job to a new status, enforcing valid transitions.
// When userID is 0 the update is not scoped by user (worker path).
func (s *Store) UpdateJobStatus(ctx context.Context, userID int64, id int64, newStatus models.JobStatus) error {
	job, err := s.GetJob(ctx, userID, id)
	if err != nil {
		return err
	}

	allowed := validTransitions[job.Status]
	valid := false
	for _, a := range allowed {
		if a == newStatus {
			valid = true
			break
		}
	}
	if !valid {
		return fmt.Errorf("store: invalid transition %s → %s", job.Status, newStatus)
	}

	var q string
	var args []any
	if userID != 0 {
		q = "UPDATE jobs SET status = $1, updated_at = $2 WHERE id = $3 AND user_id = $4"
		args = []any{string(newStatus), time.Now().UTC().Format(time.RFC3339), id, userID}
	} else {
		q = "UPDATE jobs SET status = $1, updated_at = $2 WHERE id = $3"
		args = []any{string(newStatus), time.Now().UTC().Format(time.RFC3339), id}
	}

	_, err = s.db.ExecContext(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("store: update job status: %w", err)
	}
	return nil
}

// UpdateJobSummary sets the AI-generated summary and extracted salary on a job.
// When userID is 0 the update is not scoped by user (worker path).
func (s *Store) UpdateJobSummary(ctx context.Context, userID int64, id int64, summary, extractedSalary string) error {
	var q string
	var args []any
	if userID != 0 {
		q = "UPDATE jobs SET summary = $1, extracted_salary = $2, updated_at = $3 WHERE id = $4 AND user_id = $5"
		args = []any{summary, extractedSalary, time.Now().UTC().Format(time.RFC3339), id, userID}
	} else {
		q = "UPDATE jobs SET summary = $1, extracted_salary = $2, updated_at = $3 WHERE id = $4"
		args = []any{summary, extractedSalary, time.Now().UTC().Format(time.RFC3339), id}
	}
	_, err := s.db.ExecContext(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("store: update job summary: %w", err)
	}
	return nil
}

// UpdateJobError sets the error message and transitions a job to failed status.
// When userID is 0 the update is not scoped by user (worker path).
func (s *Store) UpdateJobError(ctx context.Context, userID int64, id int64, errMsg string) error {
	var q string
	var args []any
	if userID != 0 {
		q = "UPDATE jobs SET status = $1, error_msg = $2, updated_at = $3 WHERE id = $4 AND user_id = $5"
		args = []any{string(models.StatusFailed), errMsg, time.Now().UTC().Format(time.RFC3339), id, userID}
	} else {
		q = "UPDATE jobs SET status = $1, error_msg = $2, updated_at = $3 WHERE id = $4"
		args = []any{string(models.StatusFailed), errMsg, time.Now().UTC().Format(time.RFC3339), id}
	}
	_, err := s.db.ExecContext(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("store: update job error: %w", err)
	}
	return nil
}

// UpdateJobGenerated sets the generated HTML, Markdown, and PDF paths on a job.
// When userID is 0 the update is not scoped by user (worker path).
func (s *Store) UpdateJobGenerated(ctx context.Context, userID int64, id int64, resumeHTML, coverHTML, resumeMarkdown, coverMarkdown, resumePDF, coverPDF string) error {
	var q string
	var args []any
	if userID != 0 {
		q = "UPDATE jobs SET resume_html=$1, cover_html=$2, resume_markdown=$3, cover_markdown=$4, resume_pdf=$5, cover_pdf=$6, updated_at=$7 WHERE id=$8 AND user_id=$9"
		args = []any{resumeHTML, coverHTML, resumeMarkdown, coverMarkdown, resumePDF, coverPDF, time.Now().UTC().Format(time.RFC3339), id, userID}
	} else {
		q = "UPDATE jobs SET resume_html=$1, cover_html=$2, resume_markdown=$3, cover_markdown=$4, resume_pdf=$5, cover_pdf=$6, updated_at=$7 WHERE id=$8"
		args = []any{resumeHTML, coverHTML, resumeMarkdown, coverMarkdown, resumePDF, coverPDF, time.Now().UTC().Format(time.RFC3339), id}
	}
	_, err := s.db.ExecContext(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("store: update job generated: %w", err)
	}
	return nil
}

// CreateScrapeRun inserts a new scrape run log entry.
func (s *Store) CreateScrapeRun(ctx context.Context, run *ScrapeRun) error {
	var id int64
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO scrape_runs (source, filter_keywords, jobs_found, jobs_new, started_at, finished_at, error)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id`,
		run.Source, run.FilterKeywords, run.JobsFound, run.JobsNew,
		run.StartedAt.UTC().Format(time.RFC3339), run.FinishedAt.UTC().Format(time.RFC3339),
		run.Error,
	).Scan(&id)
	if err != nil {
		return fmt.Errorf("store: create scrape run: %w", err)
	}
	run.ID = id
	return nil
}

// scanner is implemented by both *sql.Row and *sql.Rows.
type scanner interface {
	Scan(dest ...any) error
}

func scanJob(s scanner) (*models.Job, error) {
	var job models.Job
	var status string
	var discoveredAt, updatedAt string

	err := s.Scan(
		&job.ID, &job.UserID, &job.ExternalID, &job.Source,
		&job.Title, &job.Company, &job.Location,
		&job.Description, &job.Salary, &job.ApplyURL,
		&status, &job.Summary, &job.ExtractedSalary,
		&job.ResumeHTML, &job.CoverHTML, &job.ResumeMarkdown, &job.CoverMarkdown, &job.ResumePDF, &job.CoverPDF,
		&job.ErrorMsg,
		&discoveredAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}

	job.Status = models.JobStatus(status)

	if t, err := time.Parse(time.RFC3339, discoveredAt); err == nil {
		job.DiscoveredAt = t
	}
	if t, err := time.Parse(time.RFC3339, updatedAt); err == nil {
		job.UpdatedAt = t
	}
	return &job, nil
}
