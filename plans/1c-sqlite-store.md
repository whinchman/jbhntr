# Plan: 1C — SQLite Store

## Overview
Implement the SQLite persistence layer using modernc.org/sqlite (pure-Go, no CGO).
The store opens/creates the DB, auto-migrates schema, and exposes CRUD/query methods.

## Steps

### Step 1: Install dependency
- go get modernc.org/sqlite

### Step 2: Implement internal/store/store.go
Schema (auto-migrated on Open):
- jobs table: id, external_id, source, title, company, location, description, salary,
  apply_url, status, resume_html, cover_html, resume_pdf, cover_pdf, error_msg,
  discovered_at, updated_at. UNIQUE(external_id, source). CHECK(status IN (...)).
  Indexes on status and discovered_at.
- scrape_runs table: id, source, filter_keywords, jobs_found, jobs_new, started_at, finished_at, error

Methods:
- Open(path string) (*Store, error) — opens DB, enables WAL mode, runs migrations
- Close() error
- CreateJob(ctx, *models.Job) (inserted bool, error) — INSERT OR IGNORE, returns whether new
- GetJob(ctx, id int64) (*models.Job, error)
- ListJobs(ctx, ListJobsFilter) ([]models.Job, error) — filter by status, text search, pagination
- UpdateJobStatus(ctx, id int64, newStatus models.JobStatus) error — validates transition
- UpdateJobGenerated(ctx, id int64, resumeHTML, coverHTML, resumePDF, coverPDF string) error
- CreateScrapeRun(ctx, *ScrapeRun) error

Valid status transitions:
- discovered → notified, rejected
- notified → approved, rejected
- approved → generating
- generating → complete, failed
- failed → generating (retry)

### Step 3: Write tests in internal/store/store_test.go
- Use in-memory SQLite (:memory:) for all tests
- Table-driven tests for: CreateJob (insert + dedup), GetJob, ListJobs (filter/search/pagination),
  UpdateJobStatus (valid transitions, invalid transitions), UpdateJobGenerated, CreateScrapeRun

## Files Created/Modified
- internal/store/store.go (replaces stub doc.go)
- internal/store/store_test.go
