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

func (m *mockSource) Name() string { return "mock" }

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

// ─── namedMockSource: like mockSource but returns a configurable Name ─────────

type namedMockSource struct {
	name    string
	mu      sync.Mutex
	results [][]models.Job
	calls   int
	err     error
}

func (m *namedMockSource) Name() string { return m.name }

func (m *namedMockSource) Search(_ context.Context, _ models.SearchFilter) ([]models.Job, error) {
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

// dedupMockStore extends mockStore to simulate the DB-level cross-source dedup
// via the partial unique index on (user_id, dedup_hash) WHERE dedup_hash != ''.
type dedupMockStore struct {
	mockStore
	// dedupSeen maps userID|DedupHash → true once inserted
	dedupSeen map[string]bool
}

func newDedupMockStore() *dedupMockStore {
	return &dedupMockStore{
		mockStore: mockStore{seen: make(map[string]bool)},
		dedupSeen: make(map[string]bool),
	}
}

func (m *dedupMockStore) CreateJob(ctx context.Context, userID int64, job *models.Job) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Primary dedup: (userID, externalID, source)
	key := fmt.Sprintf("%d|%s|%s", userID, job.ExternalID, job.Source)
	if m.seen[key] {
		return false, nil
	}
	// Cross-source dedup via DedupHash (only when non-empty).
	if job.DedupHash != "" {
		dedupKey := fmt.Sprintf("%d|%s", userID, job.DedupHash)
		if m.dedupSeen[dedupKey] {
			return false, nil
		}
		m.dedupSeen[dedupKey] = true
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

func (m *mockUserFilterReader) ListUserBannedTerms(_ context.Context, _ int64) ([]models.UserBannedTerm, error) {
	return nil, nil
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func job(ext, source string) models.Job {
	return models.Job{ExternalID: ext, Source: source, Title: "T", Company: "C", Status: models.StatusDiscovered}
}

// jobNamed creates a job with distinct title and company (used in dedup tests).
func jobNamed(ext, source, title, company string) models.Job {
	return models.Job{ExternalID: ext, Source: source, Title: title, Company: company, Status: models.StatusDiscovered}
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
				{job("a", "mock"), job("b", "mock")}, // user 1, filter 1
				{job("b", "mock"), job("c", "mock")}, // user 1, filter 2 — "b" dup for same user
			},
		}
		ms := newMockStore()
		uf := &mockUserFilterReader{
			users: []int64{1},
			filters: map[int64][]models.UserSearchFilter{
				1: {userFilter("golang"), userFilter("staff engineer")},
			},
		}

		sched := NewScheduler([]Source{src}, ms, uf, time.Hour, nil)
		newJobs, err := sched.RunOnce(ctx)

		if err != nil {
			t.Fatalf("RunOnce() error = %v", err)
		}
		if len(newJobs) != 3 {
			t.Errorf("len(newJobs) = %d, want 3 (a, b, c)", len(newJobs))
		}
		// One ScrapeRun per source per filter: 1 source × 2 filters = 2 runs.
		if len(ms.scrapeRuns) != 2 {
			t.Errorf("scrape runs = %d, want 2 (one per filter)", len(ms.scrapeRuns))
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

		sched := NewScheduler([]Source{src}, ms, uf, time.Hour, nil)
		newJobs, err := sched.RunOnce(ctx)

		// RunOnce does not return error for individual source failures.
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
		sched := NewScheduler([]Source{src}, ms, uf, time.Hour, nil)

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
		sched := NewScheduler([]Source{src}, ms, uf, time.Hour, nil)

		_, err := sched.RunOnce(ctx)
		if err == nil {
			t.Error("RunOnce() expected error when ListActiveUserIDs fails, got nil")
		}
	})

	t.Run("per-user dedup allows same job for different users", func(t *testing.T) {
		src := &mockSource{
			results: [][]models.Job{
				{job("x", "mock")}, // user 1
				{job("x", "mock")}, // user 2 — same external job
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

		sched := NewScheduler([]Source{src}, ms, uf, time.Hour, nil)
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
				{job("y", "mock")}, // user 2 succeeds
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

		sched := NewScheduler([]Source{src}, ms, uf, time.Hour, nil)
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
				{job("u42", "mock")},
			},
		}
		ms := newMockStore()
		uf := &mockUserFilterReader{
			users: []int64{42},
			filters: map[int64][]models.UserSearchFilter{
				42: {userFilter("golang")},
			},
		}

		sched := NewScheduler([]Source{src}, ms, uf, time.Hour, nil)
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

	// ─── Multi-source tests ───────────────────────────────────────────────────

	t.Run("multisource: two sources return different jobs, both inserted", func(t *testing.T) {
		src1 := &namedMockSource{
			name: "src1",
			results: [][]models.Job{
				{jobNamed("ext-1", "src1", "Go Engineer", "Acme Corp")},
			},
		}
		src2 := &namedMockSource{
			name: "src2",
			results: [][]models.Job{
				{jobNamed("ext-2", "src2", "Rust Engineer", "Beta Corp")},
			},
		}
		ms := newMockStore()
		uf := &mockUserFilterReader{
			users: []int64{1},
			filters: map[int64][]models.UserSearchFilter{
				1: {userFilter("engineer")},
			},
		}

		sched := NewScheduler([]Source{src1, src2}, ms, uf, time.Hour, nil)
		newJobs, err := sched.RunOnce(ctx)

		if err != nil {
			t.Fatalf("RunOnce() error = %v", err)
		}
		if len(newJobs) != 2 {
			t.Errorf("len(newJobs) = %d, want 2 (one from each source)", len(newJobs))
		}
		if len(ms.created) != 2 {
			t.Errorf("created jobs = %d, want 2", len(ms.created))
		}
		// Two ScrapeRun records: one per source per filter.
		if len(ms.scrapeRuns) != 2 {
			t.Errorf("scrape runs = %d, want 2 (one per source)", len(ms.scrapeRuns))
		}
	})

	t.Run("multisource: same title+company from two sources → only one insert (dedup)", func(t *testing.T) {
		// Both sources return a job for the same real-world posting.
		src1 := &namedMockSource{
			name: "src1",
			results: [][]models.Job{
				{jobNamed("ext-s1", "src1", "Senior Go Engineer", "Acme Corp")},
			},
		}
		src2 := &namedMockSource{
			name: "src2",
			results: [][]models.Job{
				// Same title+company, different externalID/source (as would happen cross-source).
				{jobNamed("ext-s2", "src2", "Senior Go Engineer", "Acme Corp")},
			},
		}
		ms := newDedupMockStore()
		uf := &mockUserFilterReader{
			users: []int64{1},
			filters: map[int64][]models.UserSearchFilter{
				1: {userFilter("golang")},
			},
		}

		sched := NewScheduler([]Source{src1, src2}, ms, uf, time.Hour, nil)
		newJobs, err := sched.RunOnce(ctx)

		if err != nil {
			t.Fatalf("RunOnce() error = %v", err)
		}
		// Only one job should be inserted; the second triggers the dedup collision.
		if len(newJobs) != 1 {
			t.Errorf("len(newJobs) = %d, want 1 (dedup should suppress the second)", len(newJobs))
		}
		if len(ms.created) != 1 {
			t.Errorf("created jobs = %d, want 1", len(ms.created))
		}
		// Two ScrapeRuns are still written (one per source).
		if len(ms.scrapeRuns) != 2 {
			t.Errorf("scrape runs = %d, want 2 (one per source)", len(ms.scrapeRuns))
		}
	})

	t.Run("multisource: one source errors, other source's jobs are stored", func(t *testing.T) {
		srcOk := &namedMockSource{
			name: "ok-src",
			results: [][]models.Job{
				{jobNamed("ext-ok", "ok-src", "DevOps Engineer", "Reliable Co")},
			},
		}
		srcBad := &namedMockSource{
			name: "bad-src",
			err:  errors.New("bad-src API timeout"),
		}
		ms := newMockStore()
		uf := &mockUserFilterReader{
			users: []int64{1},
			filters: map[int64][]models.UserSearchFilter{
				1: {userFilter("devops")},
			},
		}

		sched := NewScheduler([]Source{srcOk, srcBad}, ms, uf, time.Hour, nil)
		newJobs, err := sched.RunOnce(ctx)

		if err != nil {
			t.Fatalf("RunOnce() error = %v", err)
		}
		// The good source's job should still be stored.
		if len(newJobs) != 1 {
			t.Errorf("len(newJobs) = %d, want 1 (ok-src job)", len(newJobs))
		}
		if len(ms.created) != 1 {
			t.Errorf("created jobs = %d, want 1", len(ms.created))
		}
		// Two ScrapeRun records: one per source. The bad one has Error set.
		if len(ms.scrapeRuns) != 2 {
			t.Errorf("scrape runs = %d, want 2 (one per source)", len(ms.scrapeRuns))
		}
		// Find the error scrape run.
		var errRun *store.ScrapeRun
		for _, r := range ms.scrapeRuns {
			if r.Error != "" {
				errRun = r
			}
		}
		if errRun == nil {
			t.Error("expected a ScrapeRun with Error set for the failing source")
		} else if errRun.Source != "bad-src" {
			t.Errorf("error ScrapeRun.Source = %q, want %q", errRun.Source, "bad-src")
		}
	})
}

func TestScheduler_LastScrapeAt(t *testing.T) {
	t.Run("zero before first run", func(t *testing.T) {
		uf := &mockUserFilterReader{}
		sched := NewScheduler([]Source{&mockSource{}}, newMockStore(), uf, time.Hour, nil)
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
		sched := NewScheduler([]Source{src}, newMockStore(), uf, time.Hour, nil)

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

func TestNewScheduler_DuplicateSourceNamePanics(t *testing.T) {
	src1 := &namedMockSource{name: "dup"}
	src2 := &namedMockSource{name: "dup"}
	ms := newMockStore()
	uf := &mockUserFilterReader{}

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("NewScheduler did not panic with duplicate source names")
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("panic value is not a string: %v", r)
		}
		want := `scraper: duplicate source name "dup"`
		if msg != want {
			t.Errorf("panic message = %q, want %q", msg, want)
		}
	}()

	NewScheduler([]Source{src1, src2}, ms, uf, time.Hour, nil)
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
		sched := NewScheduler([]Source{src}, ms, uf, 50*time.Millisecond, nil)

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
