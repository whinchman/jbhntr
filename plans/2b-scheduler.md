# Plan: 2B — Scrape Scheduler

## Overview
A background scheduler that periodically searches all configured filters via the Source
interface, deduplicates via the store, logs scrape runs, and returns newly discovered jobs.

## Steps

### Step 1: Implement internal/scraper/scheduler.go
Scheduler struct fields:
- source Source
- store  (interface with just the methods we need)
- filters []models.SearchFilter
- interval time.Duration
- logger *slog.Logger

StoreWriter interface (for testability):
- CreateJob(ctx, *models.Job) (bool, error)
- CreateScrapeRun(ctx, *store.ScrapeRun) error
- UpdateJobStatus(ctx, int64, models.JobStatus) error

Methods:
- NewScheduler(source, store, filters, interval, logger) *Scheduler
- RunOnce(ctx) ([]models.Job, error): iterate filters, search, store, count, log run
- Start(ctx): time.NewTicker loop calling RunOnce on each tick

### Step 2: Tests (internal/scraper/scheduler_test.go)
Mock Source and mock StoreWriter using simple structs.
- RunOnce with 2 filters, 3 total jobs (1 dup) → 2 new jobs returned
- RunOnce on source error → error returned, scrape run logged with error
- Start/Stop: Start then cancel context, verify no panic

## Files Created/Modified
- internal/scraper/scheduler.go
- internal/scraper/scheduler_test.go
- cmd/jobhuntr/main.go (wire scheduler)
