// Package store provides SQLite-backed persistence for jobhuntr.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/whinchman/jobhuntr/internal/models"
	_ "modernc.org/sqlite"
)

// Store wraps a SQLite database connection.
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
}

const schema = `
CREATE TABLE IF NOT EXISTS jobs (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    external_id   TEXT    NOT NULL,
    source        TEXT    NOT NULL,
    title         TEXT    NOT NULL DEFAULT '',
    company       TEXT    NOT NULL DEFAULT '',
    location      TEXT    NOT NULL DEFAULT '',
    description   TEXT    NOT NULL DEFAULT '',
    salary        TEXT    NOT NULL DEFAULT '',
    apply_url     TEXT    NOT NULL DEFAULT '',
    status        TEXT    NOT NULL DEFAULT 'discovered'
                  CHECK(status IN ('discovered','notified','approved','rejected','generating','complete','failed')),
    resume_html   TEXT    NOT NULL DEFAULT '',
    cover_html    TEXT    NOT NULL DEFAULT '',
    resume_pdf    TEXT    NOT NULL DEFAULT '',
    cover_pdf     TEXT    NOT NULL DEFAULT '',
    error_msg     TEXT    NOT NULL DEFAULT '',
    discovered_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
    updated_at    DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
    UNIQUE(external_id, source)
);
CREATE INDEX IF NOT EXISTS idx_jobs_status       ON jobs(status);
CREATE INDEX IF NOT EXISTS idx_jobs_discovered   ON jobs(discovered_at);

CREATE TABLE IF NOT EXISTS scrape_runs (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    source          TEXT    NOT NULL DEFAULT '',
    filter_keywords TEXT    NOT NULL DEFAULT '',
    jobs_found      INTEGER NOT NULL DEFAULT 0,
    jobs_new        INTEGER NOT NULL DEFAULT 0,
    started_at      DATETIME NOT NULL,
    finished_at     DATETIME NOT NULL,
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

// Open opens (or creates) the SQLite database at path, applies the schema,
// and enables WAL mode for better concurrent read performance.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("store: open db: %w", err)
	}

	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("store: enable WAL: %w", err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("store: enable foreign keys: %w", err)
	}

	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("store: migrate schema: %w", err)
	}

	return &Store{db: db}, nil
}

// Close closes the underlying database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// CreateJob inserts a job record, ignoring duplicates (UNIQUE external_id+source).
// Returns inserted=true if a new row was created, false if it already existed.
func (s *Store) CreateJob(ctx context.Context, job *models.Job) (bool, error) {
	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO jobs
		  (external_id, source, title, company, location, description, salary, apply_url, status, discovered_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		job.ExternalID, job.Source, job.Title, job.Company, job.Location,
		job.Description, job.Salary, job.ApplyURL, string(job.Status),
		now.Format(time.RFC3339), now.Format(time.RFC3339),
	)
	if err != nil {
		return false, fmt.Errorf("store: create job: %w", err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("store: create job rows affected: %w", err)
	}
	if affected == 0 {
		return false, nil
	}

	id, err := res.LastInsertId()
	if err != nil {
		return true, fmt.Errorf("store: create job last id: %w", err)
	}
	job.ID = id
	job.DiscoveredAt = now
	job.UpdatedAt = now
	return true, nil
}

// GetJob retrieves a single job by its primary key.
func (s *Store) GetJob(ctx context.Context, id int64) (*models.Job, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, external_id, source, title, company, location, description, salary, apply_url,
		       status, resume_html, cover_html, resume_pdf, cover_pdf, error_msg,
		       discovered_at, updated_at
		FROM jobs WHERE id = ?`, id)

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
func (s *Store) ListJobs(ctx context.Context, f ListJobsFilter) ([]models.Job, error) {
	var where []string
	var args []any

	if f.Status != "" {
		where = append(where, "status = ?")
		args = append(args, string(f.Status))
	}
	if f.Search != "" {
		where = append(where, "(title LIKE ? OR company LIKE ? OR description LIKE ?)")
		like := "%" + f.Search + "%"
		args = append(args, like, like, like)
	}

	q := "SELECT id, external_id, source, title, company, location, description, salary, apply_url, status, resume_html, cover_html, resume_pdf, cover_pdf, error_msg, discovered_at, updated_at FROM jobs"
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}
	q += " ORDER BY discovered_at DESC"

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
func (s *Store) UpdateJobStatus(ctx context.Context, id int64, newStatus models.JobStatus) error {
	job, err := s.GetJob(ctx, id)
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

	_, err = s.db.ExecContext(ctx,
		"UPDATE jobs SET status = ?, updated_at = ? WHERE id = ?",
		string(newStatus), time.Now().UTC().Format(time.RFC3339), id,
	)
	if err != nil {
		return fmt.Errorf("store: update job status: %w", err)
	}
	return nil
}

// UpdateJobError sets the error message and transitions a job to failed status.
func (s *Store) UpdateJobError(ctx context.Context, id int64, errMsg string) error {
	_, err := s.db.ExecContext(ctx,
		"UPDATE jobs SET status = ?, error_msg = ?, updated_at = ? WHERE id = ?",
		string(models.StatusFailed), errMsg, time.Now().UTC().Format(time.RFC3339), id,
	)
	if err != nil {
		return fmt.Errorf("store: update job error: %w", err)
	}
	return nil
}

// UpdateJobGenerated sets the generated HTML and PDF paths on a job.
func (s *Store) UpdateJobGenerated(ctx context.Context, id int64, resumeHTML, coverHTML, resumePDF, coverPDF string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE jobs SET resume_html=?, cover_html=?, resume_pdf=?, cover_pdf=?, updated_at=?
		WHERE id=?`,
		resumeHTML, coverHTML, resumePDF, coverPDF,
		time.Now().UTC().Format(time.RFC3339), id,
	)
	if err != nil {
		return fmt.Errorf("store: update job generated: %w", err)
	}
	return nil
}

// CreateScrapeRun inserts a new scrape run log entry.
func (s *Store) CreateScrapeRun(ctx context.Context, run *ScrapeRun) error {
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO scrape_runs (source, filter_keywords, jobs_found, jobs_new, started_at, finished_at, error)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		run.Source, run.FilterKeywords, run.JobsFound, run.JobsNew,
		run.StartedAt.UTC().Format(time.RFC3339), run.FinishedAt.UTC().Format(time.RFC3339),
		run.Error,
	)
	if err != nil {
		return fmt.Errorf("store: create scrape run: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("store: create scrape run last id: %w", err)
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
		&job.ID, &job.ExternalID, &job.Source,
		&job.Title, &job.Company, &job.Location,
		&job.Description, &job.Salary, &job.ApplyURL,
		&status,
		&job.ResumeHTML, &job.CoverHTML, &job.ResumePDF, &job.CoverPDF,
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
