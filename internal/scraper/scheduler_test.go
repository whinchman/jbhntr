package scraper

import (
	"context"
	"errors"
	"fmt"
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
	created    []createdJob
	scrapeRuns []*store.ScrapeRun
	// seen maps userID|ExternalID|Source → whether it was "new"
	seen map[string]bool
}

type createdJob struct {
	userID int64
	job    *models.Job
}

func newMockStore() *mockStore {
	return &mockStore{seen: make(map[string]bool)}
}

func (m *mockStore) CreateJob(_ context.Context, userID int64, job *models.Job) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := fmt.Sprintf("%d|%s|%s", userID, job.ExternalID, job.Source)
	if m.seen[key] {
		return false, nil
	}
	m.seen[key] = true
	job.ID = int64(len(m.created) + 1)
	m.created = append(m.created, createdJob{userID: userID, job: job})
	return true, nil
}

func (m *mockStore) CreateScrapeRun(_ context.Context, run *store.ScrapeRun) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	run.ID = int64(len(m.scrapeRuns) + 1)
	m.scrapeRuns = append(m.scrapeRuns, run)
	return nil
}

func (m *mockStore) UpdateJobStatus(_ context.Context, _ int64, _ int64, _ models.JobStatus) error {
	return nil
}

func (m *mockStore) UpdateJobSummary(_ context.Context, _ int64, _ int64, _, _ string) error {
	return nil
}

// ─── mock UserFilterReader ──────────────────────────────────────────────────

type mockUserFilterReader struct {
	users   []int64
	filters map[int64][]models.UserSearchFilter
	err     error // error returned by ListActiveUserIDs
	// filterErrors maps userID to error for ListUserFilters
	filterErrors map[int64]error
}

func (m *mockUserFilterReader) ListActiveUserIDs(_ context.Context) ([]int64, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.users, nil
}

func (m *mockUserFilterReader) ListUserFilters(_ context.Context, userID int64) ([]models.UserSearchFilter, error) {
	if m.filterErrors != nil {
		if err, ok := m.filterErrors[userID]; ok {
			return nil, err
		}
	}
	return m.filters[userID], nil
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func job(ext, source string) models.Job {
	return models.Job{ExternalID: ext, Source: source, Title: "T", Company: "C", Status: models.StatusDiscovered}
}

func userFilter(keywords string) models.UserSearchFilter {
	return models.UserSearchFilter{Keywords: keywords}
}

// ─── tests ───────────────────────────────────────────────────────────────────

func TestScheduler_RunOnce(t *testing.T) {
	ctx := context.Background()

	t.Run("returns new jobs and logs scrape run", func(t *testing.T) {
		src := &mockSource{
			results: [][]models.Job{
				{job("a", "serpapi"), job("b", "serpapi")}, // user 1, filter 1
				{job("b", "serpapi"), job("c", "serpapi")}, // user 1, filter 2 — "b" dup for same user
			},
		}
		ms := newMockStore()
		uf := &mockUserFilterReader{
			users: []int64{1},
			filters: map[int64][]models.UserSearchFilter{
				1: {userFilter("golang"), userFilter("staff engineer")},
			},
		}

		sched := NewScheduler(src, ms, uf, time.Hour, nil)
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

	t.Run("source error logs and continues", func(t *testing.T) {
		src := &mockSource{err: errors.New("api down")}
		ms := newMockStore()
		uf := &mockUserFilterReader{
			users: []int64{1},
			filters: map[int64][]models.UserSearchFilter{
				1: {userFilter("golang")},
			},
		}

		sched := NewScheduler(src, ms, uf, time.Hour, nil)
		newJobs, err := sched.RunOnce(ctx)

		// RunOnce no longer returns error for individual filter failures.
		if err != nil {
			t.Fatalf("RunOnce() unexpected error = %v", err)
		}
		if len(newJobs) != 0 {
			t.Errorf("len(newJobs) = %d, want 0", len(newJobs))
		}
		if len(ms.scrapeRuns) != 1 {
			t.Errorf("scrape runs = %d, want 1", len(ms.scrapeRuns))
		}
		if ms.scrapeRuns[0].Error == "" {
			t.Error("scrape run should have non-empty error field")
		}
	})

	t.Run("no active users returns empty slice", func(t *testing.T) {
		src := &mockSource{}
		ms := newMockStore()
		uf := &mockUserFilterReader{
			users:   nil,
			filters: map[int64][]models.UserSearchFilter{},
		}
		sched := NewScheduler(src, ms, uf, time.Hour, nil)

		newJobs, err := sched.RunOnce(ctx)
		if err != nil {
			t.Fatalf("RunOnce() error = %v", err)
		}
		if len(newJobs) != 0 {
			t.Errorf("len(newJobs) = %d, want 0", len(newJobs))
		}
	})

	t.Run("ListActiveUserIDs error returns error", func(t *testing.T) {
		src := &mockSource{}
		ms := newMockStore()
		uf := &mockUserFilterReader{
			err: errors.New("db down"),
		}
		sched := NewScheduler(src, ms, uf, time.Hour, nil)

		_, err := sched.RunOnce(ctx)
		if err == nil {
			t.Error("RunOnce() expected error when ListActiveUserIDs fails, got nil")
		}
	})

	t.Run("per-user dedup allows same job for different users", func(t *testing.T) {
		src := &mockSource{
			results: [][]models.Job{
				{job("x", "serpapi")}, // user 1
				{job("x", "serpapi")}, // user 2 — same external job
			},
		}
		ms := newMockStore()
		uf := &mockUserFilterReader{
			users: []int64{1, 2},
			filters: map[int64][]models.UserSearchFilter{
				1: {userFilter("golang")},
				2: {userFilter("golang")},
			},
		}

		sched := NewScheduler(src, ms, uf, time.Hour, nil)
		newJobs, err := sched.RunOnce(ctx)

		if err != nil {
			t.Fatalf("RunOnce() error = %v", err)
		}
		if len(newJobs) != 2 {
			t.Errorf("len(newJobs) = %d, want 2 (same job for two users)", len(newJobs))
		}
		if len(ms.created) != 2 {
			t.Errorf("created jobs = %d, want 2", len(ms.created))
		}
	})

	t.Run("one user error does not block other users", func(t *testing.T) {
		src := &mockSource{
			results: [][]models.Job{
				{job("y", "serpapi")}, // user 2 succeeds
			},
		}
		ms := newMockStore()
		uf := &mockUserFilterReader{
			users: []int64{1, 2},
			filters: map[int64][]models.UserSearchFilter{
				2: {userFilter("golang")},
			},
			filterErrors: map[int64]error{
				1: errors.New("user 1 db error"),
			},
		}

		sched := NewScheduler(src, ms, uf, time.Hour, nil)
		newJobs, err := sched.RunOnce(ctx)

		if err != nil {
			t.Fatalf("RunOnce() error = %v", err)
		}
		if len(newJobs) != 1 {
			t.Errorf("len(newJobs) = %d, want 1 (user 2's job)", len(newJobs))
		}
	})

	t.Run("userID is passed correctly to CreateJob", func(t *testing.T) {
		src := &mockSource{
			results: [][]models.Job{
				{job("u42", "serpapi")},
			},
		}
		ms := newMockStore()
		uf := &mockUserFilterReader{
			users: []int64{42},
			filters: map[int64][]models.UserSearchFilter{
				42: {userFilter("golang")},
			},
		}

		sched := NewScheduler(src, ms, uf, time.Hour, nil)
		_, err := sched.RunOnce(ctx)
		if err != nil {
			t.Fatalf("RunOnce() error = %v", err)
		}

		if len(ms.created) != 1 {
			t.Fatalf("expected 1 created job, got %d", len(ms.created))
		}
		if ms.created[0].userID != 42 {
			t.Errorf("CreateJob userID = %d, want 42", ms.created[0].userID)
		}
	})
}

func TestScheduler_LastScrapeAt(t *testing.T) {
	t.Run("zero before first run", func(t *testing.T) {
		uf := &mockUserFilterReader{}
		sched := NewScheduler(&mockSource{}, newMockStore(), uf, time.Hour, nil)
		if !sched.LastScrapeAt().IsZero() {
			t.Error("LastScrapeAt() should be zero before first RunOnce")
		}
	})

	t.Run("non-zero after RunOnce", func(t *testing.T) {
		src := &mockSource{results: [][]models.Job{{}}}
		uf := &mockUserFilterReader{
			users: []int64{1},
			filters: map[int64][]models.UserSearchFilter{
				1: {userFilter("go")},
			},
		}
		sched := NewScheduler(src, newMockStore(), uf, time.Hour, nil)

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
		uf := &mockUserFilterReader{
			users: []int64{1},
			filters: map[int64][]models.UserSearchFilter{
				1: {userFilter("test")},
			},
		}
		sched := NewScheduler(src, ms, uf, 50*time.Millisecond, nil)

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
