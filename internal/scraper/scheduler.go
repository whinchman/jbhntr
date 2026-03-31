package scraper

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/whinchman/jobhuntr/internal/models"
	"github.com/whinchman/jobhuntr/internal/store"
)

// StoreWriter is the subset of store.Store needed by the Scheduler.
type StoreWriter interface {
	CreateJob(ctx context.Context, job *models.Job) (bool, error)
	CreateScrapeRun(ctx context.Context, run *store.ScrapeRun) error
}

// Scheduler periodically searches all configured filters and persists new jobs.
type Scheduler struct {
	source   Source
	store    StoreWriter
	filters  []models.SearchFilter
	interval time.Duration
	logger   *slog.Logger
}

// NewScheduler constructs a Scheduler. If logger is nil, slog.Default() is used.
func NewScheduler(source Source, st StoreWriter, filters []models.SearchFilter, interval time.Duration, logger *slog.Logger) *Scheduler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Scheduler{
		source:   source,
		store:    st,
		filters:  filters,
		interval: interval,
		logger:   logger,
	}
}

// RunOnce executes one full scrape cycle across all search filters.
// It returns the slice of newly discovered (inserted) jobs.
func (s *Scheduler) RunOnce(ctx context.Context) ([]models.Job, error) {
	var newJobs []models.Job

	for _, filter := range s.filters {
		jobs, err := s.runFilter(ctx, filter)
		if err != nil {
			return newJobs, err
		}
		newJobs = append(newJobs, jobs...)
	}

	return newJobs, nil
}

// runFilter runs one search filter: searches, stores results, logs the run.
func (s *Scheduler) runFilter(ctx context.Context, filter models.SearchFilter) ([]models.Job, error) {
	started := time.Now()
	run := &store.ScrapeRun{
		Source:         "serpapi",
		FilterKeywords: filter.Keywords,
		StartedAt:      started,
	}

	results, searchErr := s.source.Search(ctx, filter)
	if searchErr != nil {
		run.FinishedAt = time.Now()
		run.Error = searchErr.Error()
		if logErr := s.store.CreateScrapeRun(ctx, run); logErr != nil {
			s.logger.Error("failed to log scrape run", "error", logErr, "filter", filter.Keywords)
		}
		return nil, fmt.Errorf("scheduler: search filter %q: %w", filter.Keywords, searchErr)
	}

	run.JobsFound = len(results)

	var newJobs []models.Job
	for i := range results {
		job := &results[i]
		if job.Status == "" {
			job.Status = models.StatusDiscovered
		}
		inserted, err := s.store.CreateJob(ctx, job)
		if err != nil {
			s.logger.Error("failed to store job", "error", err, "external_id", job.ExternalID)
			continue
		}
		if inserted {
			newJobs = append(newJobs, *job)
		}
	}

	run.JobsNew = len(newJobs)
	run.FinishedAt = time.Now()

	if err := s.store.CreateScrapeRun(ctx, run); err != nil {
		s.logger.Error("failed to log scrape run", "error", err, "filter", filter.Keywords)
	}

	s.logger.Info("scrape complete",
		"filter", filter.Keywords,
		"found", run.JobsFound,
		"new", run.JobsNew,
		"duration", run.FinishedAt.Sub(started),
	)

	return newJobs, nil
}

// Start launches a background goroutine that calls RunOnce at each interval tick.
// It returns when ctx is cancelled.
func (s *Scheduler) Start(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	s.logger.Info("scheduler started", "interval", s.interval)

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("scheduler stopped")
			return
		case <-ticker.C:
			newJobs, err := s.RunOnce(ctx)
			if err != nil {
				s.logger.Error("scrape run failed", "error", err)
				continue
			}
			s.logger.Info("scrape run complete", "new_jobs", len(newJobs))
		}
	}
}
