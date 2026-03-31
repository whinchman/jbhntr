package scraper

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/whinchman/jobhuntr/internal/models"
	"github.com/whinchman/jobhuntr/internal/store"
)

// ─── mock Source ─────────────────────────────────────────────────────────────

type mockSource struct {
	mu      sync.Mutex
	results [][]models.Job // results[i] returned on the i-th call
	calls   int
	err     error
}

func (m *mockSource) Search(_ context.Context, _ models.SearchFilter) ([]models.Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return nil, m.err
	}
	if m.calls >= len(m.results) {
		return nil, nil
	}
	res := m.results[m.calls]
	m.calls++
	return res, nil
}

// ─── mock StoreWriter ────────────────────────────────────────────────────────

type mockStore struct {
	mu         sync.Mutex
	created    []*models.Job
	scrapeRuns []*store.ScrapeRun
	// insertedIDs maps ExternalID+Source → whether it was "new"
	seen map[string]bool
}

func newMockStore() *mockStore {
	return &mockStore{seen: make(map[string]bool)}
}

func (m *mockStore) CreateJob(_ context.Context, job *models.Job) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := job.ExternalID + "|" + job.Source
	if m.seen[key] {
		return false, nil
	}
	m.seen[key] = true
	job.ID = int64(len(m.created) + 1)
	m.created = append(m.created, job)
	return true, nil
}

func (m *mockStore) CreateScrapeRun(_ context.Context, run *store.ScrapeRun) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	run.ID = int64(len(m.scrapeRuns) + 1)
	m.scrapeRuns = append(m.scrapeRuns, run)
	return nil
}

func (m *mockStore) UpdateJobStatus(_ context.Context, _ int64, _ models.JobStatus) error {
	return nil
}

func (m *mockStore) UpdateJobSummary(_ context.Context, _ int64, _, _ string) error {
	return nil
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func job(ext, source string) models.Job {
	return models.Job{ExternalID: ext, Source: source, Title: "T", Company: "C", Status: models.StatusDiscovered}
}

// ─── tests ───────────────────────────────────────────────────────────────────

func TestScheduler_RunOnce(t *testing.T) {
	ctx := context.Background()

	t.Run("returns new jobs and logs scrape run", func(t *testing.T) {
		src := &mockSource{
			results: [][]models.Job{
				{job("a", "serpapi"), job("b", "serpapi")}, // filter 1
				{job("b", "serpapi"), job("c", "serpapi")}, // filter 2 — "b" is dup
			},
		}
		ms := newMockStore()
		filters := []models.SearchFilter{
			{Keywords: "golang"},
			{Keywords: "staff engineer"},
		}

		sched := NewScheduler(src, ms, filters, time.Hour, nil)
		newJobs, err := sched.RunOnce(ctx)

		if err != nil {
			t.Fatalf("RunOnce() error = %v", err)
		}
		if len(newJobs) != 3 {
			t.Errorf("len(newJobs) = %d, want 3 (a, b, c)", len(newJobs))
		}
		if len(ms.scrapeRuns) != 2 {
			t.Errorf("scrape runs = %d, want 2 (one per filter)", len(ms.scrapeRuns))
		}
		if ms.scrapeRuns[0].JobsNew != 2 {
			t.Errorf("run[0].JobsNew = %d, want 2", ms.scrapeRuns[0].JobsNew)
		}
		if ms.scrapeRuns[1].JobsNew != 1 {
			t.Errorf("run[1].JobsNew = %d, want 1 (b was dup)", ms.scrapeRuns[1].JobsNew)
		}
	})

	t.Run("source error returns error and logs run with error field", func(t *testing.T) {
		src := &mockSource{err: errors.New("api down")}
		ms := newMockStore()

		sched := NewScheduler(src, ms, []models.SearchFilter{{Keywords: "golang"}}, time.Hour, nil)
		_, err := sched.RunOnce(ctx)

		if err == nil {
			t.Error("RunOnce() expected error, got nil")
		}
		if len(ms.scrapeRuns) != 1 {
			t.Errorf("scrape runs = %d, want 1", len(ms.scrapeRuns))
		}
		if ms.scrapeRuns[0].Error == "" {
			t.Error("scrape run should have non-empty error field")
		}
	})

	t.Run("no filters returns empty slice", func(t *testing.T) {
		src := &mockSource{}
		ms := newMockStore()
		sched := NewScheduler(src, ms, nil, time.Hour, nil)

		newJobs, err := sched.RunOnce(ctx)
		if err != nil {
			t.Fatalf("RunOnce() error = %v", err)
		}
		if len(newJobs) != 0 {
			t.Errorf("len(newJobs) = %d, want 0", len(newJobs))
		}
	})
}

func TestScheduler_LastScrapeAt(t *testing.T) {
	t.Run("zero before first run", func(t *testing.T) {
		sched := NewScheduler(&mockSource{}, newMockStore(), nil, time.Hour, nil)
		if !sched.LastScrapeAt().IsZero() {
			t.Error("LastScrapeAt() should be zero before first RunOnce")
		}
	})

	t.Run("non-zero after RunOnce", func(t *testing.T) {
		src := &mockSource{results: [][]models.Job{{}}}
		sched := NewScheduler(src, newMockStore(), []models.SearchFilter{{Keywords: "go"}}, time.Hour, nil)

		before := time.Now()
		if _, err := sched.RunOnce(context.Background()); err != nil {
			t.Fatalf("RunOnce: %v", err)
		}
		after := time.Now()

		got := sched.LastScrapeAt()
		if got.IsZero() {
			t.Error("LastScrapeAt() still zero after RunOnce")
		}
		if got.Before(before) || got.After(after) {
			t.Errorf("LastScrapeAt() = %v, want between %v and %v", got, before, after)
		}
	})
}

func TestScheduler_Start(t *testing.T) {
	t.Run("cancels cleanly without panic", func(t *testing.T) {
		src := &mockSource{results: [][]models.Job{{}}}
		ms := newMockStore()
		sched := NewScheduler(src, ms, []models.SearchFilter{{Keywords: "test"}}, 50*time.Millisecond, nil)

		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Millisecond)
		defer cancel()

		done := make(chan struct{})
		go func() {
			sched.Start(ctx)
			close(done)
		}()

		select {
		case <-done:
			// clean exit
		case <-time.After(2 * time.Second):
			t.Error("Start() did not return after context cancellation")
		}
	})
}
