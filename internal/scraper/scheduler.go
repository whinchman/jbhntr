package scraper

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/whinchman/jobhuntr/internal/generator"
	"github.com/whinchman/jobhuntr/internal/models"
	"github.com/whinchman/jobhuntr/internal/notifier"
	"github.com/whinchman/jobhuntr/internal/store"
)

// StoreWriter is the subset of store.Store needed by the Scheduler.
type StoreWriter interface {
	CreateJob(ctx context.Context, userID int64, job *models.Job) (bool, error)
	CreateScrapeRun(ctx context.Context, run *store.ScrapeRun) error
	UpdateJobStatus(ctx context.Context, userID int64, id int64, status models.JobStatus) error
	UpdateJobSummary(ctx context.Context, userID int64, id int64, summary, extractedSalary string) error
}

// Scheduler periodically searches all configured filters and persists new jobs.
type Scheduler struct {
	source     Source
	store      StoreWriter
	notifier   notifier.Notifier
	summarizer generator.Summarizer
	filters    []models.SearchFilter
	interval   time.Duration
	logger     *slog.Logger

	mu           sync.Mutex
	lastScrapeAt time.Time
}

// LastScrapeAt returns the time of the most recent completed scrape cycle, or
// the zero value if no scrape has run yet.
func (s *Scheduler) LastScrapeAt() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastScrapeAt
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

// WithNotifier sets an optional Notifier on the Scheduler.
func (s *Scheduler) WithNotifier(n notifier.Notifier) *Scheduler {
	s.notifier = n
	return s
}

// WithSummarizer sets an optional Summarizer on the Scheduler.
// When set, each newly discovered job is summarized via Claude.
func (s *Scheduler) WithSummarizer(sum generator.Summarizer) *Scheduler {
	s.summarizer = sum
	return s
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

	s.mu.Lock()
	s.lastScrapeAt = time.Now()
	s.mu.Unlock()

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
		// TODO(task4): pass real userID from per-user filter iteration
		inserted, err := s.store.CreateJob(ctx, 0, job)
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

	if s.summarizer != nil {
		for i, job := range newJobs {
			summary, salary, err := s.summarizer.Summarize(ctx, job)
			if err != nil {
				s.logger.Error("failed to summarize job", "job_id", job.ID, "error", err)
				continue
			}
			// TODO(task4): pass real userID from per-user filter iteration
			if err := s.store.UpdateJobSummary(ctx, 0, job.ID, summary, salary); err != nil {
				s.logger.Error("failed to save job summary", "job_id", job.ID, "error", err)
				continue
			}
			newJobs[i].Summary = summary
			newJobs[i].ExtractedSalary = salary
		}
	}

	if s.notifier != nil {
		for _, job := range newJobs {
			if err := s.notifier.Notify(ctx, job); err != nil {
				s.logger.Error("failed to send notification", "job_id", job.ID, "error", err)
				continue
			}
			// TODO(task4): pass real userID from per-user filter iteration
			if err := s.store.UpdateJobStatus(ctx, 0, job.ID, models.StatusNotified); err != nil {
				s.logger.Error("failed to update job status to notified", "job_id", job.ID, "error", err)
			}
		}
	}

	return newJobs, nil
}

// Start runs an initial scrape immediately, then repeats at each interval tick.
// It returns when ctx is cancelled.
func (s *Scheduler) Start(ctx context.Context) {
	s.logger.Info("scheduler started", "interval", s.interval)

	// Run first scrape immediately instead of waiting for the first tick.
	s.runAndLog(ctx)

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("scheduler stopped")
			return
		case <-ticker.C:
			s.runAndLog(ctx)
		}
	}
}

func (s *Scheduler) runAndLog(ctx context.Context) {
	newJobs, err := s.RunOnce(ctx)
	if err != nil {
		s.logger.Error("scrape run failed", "error", err)
		return
	}
	s.logger.Info("scrape run complete", "new_jobs", len(newJobs))
}
