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

// UserFilterReader provides access to per-user search filters.
// The scheduler uses this to iterate users and their search queries.
type UserFilterReader interface {
	ListActiveUserIDs(ctx context.Context) ([]int64, error)
	ListUserFilters(ctx context.Context, userID int64) ([]models.UserSearchFilter, error)
}

// UserReader provides access to user records.
// The scheduler uses this to fetch the ntfy topic per user before notifying.
type UserReader interface {
	GetUser(ctx context.Context, id int64) (*models.User, error)
}

// Scheduler periodically searches all configured filters and persists new jobs.
type Scheduler struct {
	source      Source
	store       StoreWriter
	userFilters UserFilterReader
	userReader  UserReader
	notifier    notifier.Notifier
	summarizer  generator.Summarizer
	interval    time.Duration
	logger      *slog.Logger

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
func NewScheduler(source Source, st StoreWriter, uf UserFilterReader, interval time.Duration, logger *slog.Logger) *Scheduler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Scheduler{
		source:      source,
		store:       st,
		userFilters: uf,
		interval:    interval,
		logger:      logger,
	}
}

// WithNotifier sets an optional Notifier on the Scheduler.
func (s *Scheduler) WithNotifier(n notifier.Notifier) *Scheduler {
	s.notifier = n
	return s
}

// WithUserReader sets the UserReader used to fetch per-user ntfy topics.
// Required when a Notifier is set; without it notifications are skipped.
func (s *Scheduler) WithUserReader(ur UserReader) *Scheduler {
	s.userReader = ur
	return s
}

// WithSummarizer sets an optional Summarizer on the Scheduler.
// When set, each newly discovered job is summarized via Claude.
func (s *Scheduler) WithSummarizer(sum generator.Summarizer) *Scheduler {
	s.summarizer = sum
	return s
}

// RunOnce executes one full scrape cycle across all users and their search
// filters. It returns the slice of newly discovered (inserted) jobs.
// Individual user/filter errors are logged and skipped so that one user's
// failure does not block other users.
func (s *Scheduler) RunOnce(ctx context.Context) ([]models.Job, error) {
	var newJobs []models.Job

	userIDs, err := s.userFilters.ListActiveUserIDs(ctx)
	if err != nil {
		return nil, fmt.Errorf("scheduler: list active users: %w", err)
	}

	for _, userID := range userIDs {
		filters, err := s.userFilters.ListUserFilters(ctx, userID)
		if err != nil {
			s.logger.Error("failed to list filters for user", "user_id", userID, "error", err)
			continue
		}

		// Fetch the user's ntfy topic once per user, before iterating filters.
		ntfyTopic := ""
		if s.notifier != nil && s.userReader != nil {
			user, err := s.userReader.GetUser(ctx, userID)
			if err != nil {
				s.logger.Error("failed to get user for ntfy topic", "user_id", userID, "error", err)
			} else {
				ntfyTopic = user.NtfyTopic
			}
		}

		for _, uf := range filters {
			filter := userFilterToSearchFilter(uf)
			jobs, err := s.runFilter(ctx, userID, ntfyTopic, filter)
			if err != nil {
				s.logger.Error("scrape filter failed", "user_id", userID, "filter", filter.Keywords, "error", err)
				continue
			}
			newJobs = append(newJobs, jobs...)
		}
	}

	s.mu.Lock()
	s.lastScrapeAt = time.Now()
	s.mu.Unlock()

	return newJobs, nil
}

// userFilterToSearchFilter converts a per-user database filter to the
// SearchFilter type used by the Source interface.
func userFilterToSearchFilter(uf models.UserSearchFilter) models.SearchFilter {
	return models.SearchFilter{
		Keywords:  uf.Keywords,
		Location:  uf.Location,
		MinSalary: uf.MinSalary,
		MaxSalary: uf.MaxSalary,
		Title:     uf.Title,
	}
}

// runFilter runs one search filter for a specific user: searches, stores
// results, logs the run.
func (s *Scheduler) runFilter(ctx context.Context, userID int64, ntfyTopic string, filter models.SearchFilter) ([]models.Job, error) {
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
		inserted, err := s.store.CreateJob(ctx, userID, job)
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
		"user_id", userID,
		"filter", filter.Keywords,
		"found", run.JobsFound,
		"new", run.JobsNew,
		"duration", run.FinishedAt.Sub(started),
	)

	newJobs = s.summarizeNewJobs(ctx, userID, newJobs)
	s.notifyNewJobs(ctx, userID, ntfyTopic, newJobs)

	return newJobs, nil
}

// summarizeNewJobs calls the Summarizer for each newly discovered job and
// updates the summary and extracted salary in the store. The returned slice
// is the same slice with Summary and ExtractedSalary fields populated where
// summarization succeeded. If no Summarizer is configured it is a no-op.
func (s *Scheduler) summarizeNewJobs(ctx context.Context, userID int64, newJobs []models.Job) []models.Job {
	if s.summarizer == nil {
		return newJobs
	}
	for i, job := range newJobs {
		summary, salary, err := s.summarizer.Summarize(ctx, job)
		if err != nil {
			s.logger.Error("failed to summarize job", "job_id", job.ID, "error", err)
			continue
		}
		if err := s.store.UpdateJobSummary(ctx, userID, job.ID, summary, salary); err != nil {
			s.logger.Error("failed to save job summary", "job_id", job.ID, "error", err)
			continue
		}
		newJobs[i].Summary = summary
		newJobs[i].ExtractedSalary = salary
	}
	return newJobs
}

// notifyNewJobs sends a notification for each newly discovered job and marks
// it as notified in the store. If no Notifier is configured it is a no-op.
// topic is the per-user ntfy topic; an empty topic causes Notify to skip silently.
func (s *Scheduler) notifyNewJobs(ctx context.Context, userID int64, topic string, newJobs []models.Job) {
	if s.notifier == nil {
		return
	}
	for _, job := range newJobs {
		if err := s.notifier.Notify(ctx, job, topic); err != nil {
			s.logger.Error("failed to send notification", "job_id", job.ID, "error", err)
			continue
		}
		if topic != "" {
			if err := s.store.UpdateJobStatus(ctx, userID, job.ID, models.StatusNotified); err != nil {
				s.logger.Error("failed to update job status to notified", "job_id", job.ID, "error", err)
			}
		}
	}
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
