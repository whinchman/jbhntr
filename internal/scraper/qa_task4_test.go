package scraper

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/whinchman/jobhuntr/internal/models"
)

// ─── QA: Adversarial scheduler tests for task4-multiuser-scraper ───────────

// TestQA_Scheduler_UserWithFiltersNoJobs verifies that the scheduler
// correctly handles a user who has filters but the source returns no jobs.
func TestQA_Scheduler_UserWithFiltersNoJobs(t *testing.T) {
	ctx := context.Background()

	src := &mockSource{
		results: [][]models.Job{
			{}, // user 1 filter 1: empty results
			{}, // user 1 filter 2: empty results
		},
	}
	ms := newMockStore()
	uf := &mockUserFilterReader{
		users: []int64{1},
		filters: map[int64][]models.UserSearchFilter{
			1: {userFilter("golang"), userFilter("rust")},
		},
	}

	sched := NewScheduler([]Source{src}, ms, uf, time.Hour, nil)
	newJobs, err := sched.RunOnce(ctx)

	if err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	if len(newJobs) != 0 {
		t.Errorf("len(newJobs) = %d, want 0", len(newJobs))
	}
	// Two scrape runs should be logged (one per filter).
	if len(ms.scrapeRuns) != 2 {
		t.Errorf("scrape runs = %d, want 2", len(ms.scrapeRuns))
	}
	for i, run := range ms.scrapeRuns {
		if run.JobsFound != 0 {
			t.Errorf("run[%d].JobsFound = %d, want 0", i, run.JobsFound)
		}
		if run.JobsNew != 0 {
			t.Errorf("run[%d].JobsNew = %d, want 0", i, run.JobsNew)
		}
	}
}

// TestQA_Scheduler_EmptyActiveUsers verifies that the scheduler does
// nothing when ListActiveUserIDs returns an empty list.
func TestQA_Scheduler_EmptyActiveUsers(t *testing.T) {
	ctx := context.Background()

	src := &mockSource{}
	ms := newMockStore()
	uf := &mockUserFilterReader{
		users:   []int64{},
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
	if len(ms.scrapeRuns) != 0 {
		t.Errorf("scrape runs = %d, want 0 (no users means no scrapes)", len(ms.scrapeRuns))
	}
	if len(ms.created) != 0 {
		t.Errorf("created jobs = %d, want 0", len(ms.created))
	}
	// LastScrapeAt should still be updated (RunOnce completed).
	if sched.LastScrapeAt().IsZero() {
		t.Error("LastScrapeAt() should be non-zero after RunOnce even with no users")
	}
}

// TestQA_Scheduler_SameJobThreeUsers verifies that the same external job
// appearing in search results for 3 different users creates 3 separate
// entries (per-user dedup).
func TestQA_Scheduler_SameJobThreeUsers(t *testing.T) {
	ctx := context.Background()

	src := &mockSource{
		results: [][]models.Job{
			{job("shared-x", "serpapi")}, // user 10
			{job("shared-x", "serpapi")}, // user 20
			{job("shared-x", "serpapi")}, // user 30
		},
	}
	ms := newMockStore()
	uf := &mockUserFilterReader{
		users: []int64{10, 20, 30},
		filters: map[int64][]models.UserSearchFilter{
			10: {userFilter("golang")},
			20: {userFilter("golang")},
			30: {userFilter("golang")},
		},
	}

	sched := NewScheduler([]Source{src}, ms, uf, time.Hour, nil)
	newJobs, err := sched.RunOnce(ctx)

	if err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	if len(newJobs) != 3 {
		t.Errorf("len(newJobs) = %d, want 3 (same job for 3 users)", len(newJobs))
	}
	if len(ms.created) != 3 {
		t.Fatalf("created = %d, want 3", len(ms.created))
	}

	// Verify correct userID assignment.
	expectedUsers := map[int64]bool{10: false, 20: false, 30: false}
	for _, c := range ms.created {
		if _, ok := expectedUsers[c.userID]; !ok {
			t.Errorf("unexpected userID %d in created jobs", c.userID)
		}
		expectedUsers[c.userID] = true
	}
	for uid, seen := range expectedUsers {
		if !seen {
			t.Errorf("userID %d not found in created jobs", uid)
		}
	}
}

// TestQA_Scheduler_DeletedUserWithFilters simulates the case where
// ListActiveUserIDs returns a user_id for a user that has been deleted
// but whose filters still exist. The scheduler should handle this
// gracefully (it just scrapes for that user_id; the store layer doesn't
// enforce FK between jobs.user_id and users.id).
func TestQA_Scheduler_DeletedUserWithFilters(t *testing.T) {
	ctx := context.Background()

	// Simulate user_id 999 that has filters but is "deleted".
	src := &mockSource{
		results: [][]models.Job{
			{job("orphan-job", "serpapi")},
		},
	}
	ms := newMockStore()
	uf := &mockUserFilterReader{
		users: []int64{999},
		filters: map[int64][]models.UserSearchFilter{
			999: {userFilter("test")},
		},
	}

	sched := NewScheduler([]Source{src}, ms, uf, time.Hour, nil)
	newJobs, err := sched.RunOnce(ctx)

	if err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	if len(newJobs) != 1 {
		t.Errorf("len(newJobs) = %d, want 1", len(newJobs))
	}
	if len(ms.created) != 1 {
		t.Fatalf("created = %d, want 1", len(ms.created))
	}
	if ms.created[0].userID != 999 {
		t.Errorf("created[0].userID = %d, want 999", ms.created[0].userID)
	}
}

// TestQA_Scheduler_MultipleUsersPartialFailure verifies that when one
// user's ListUserFilters fails and another user's source search fails,
// a third user's jobs are still collected.
func TestQA_Scheduler_MultipleUsersPartialFailure(t *testing.T) {
	ctx := context.Background()

	// User 1: ListUserFilters fails.
	// User 2: source.Search fails.
	// User 3: succeeds.
	ms := newMockStore()
	uf := &mockUserFilterReader{
		users: []int64{1, 2, 3},
		filters: map[int64][]models.UserSearchFilter{
			2: {userFilter("python")},
			3: {userFilter("go")},
		},
		filterErrors: map[int64]error{
			1: errors.New("user 1 filter error"),
		},
	}

	// Override source to return error for user 2's filter call.
	// The mockSource returns results in order; user 1 is skipped (filter error),
	// so call 0 goes to user 2 (nil result -> empty), call 1 to user 3.
	src2 := &mockSourceWithErrors{
		callResults: []mockCallResult{
			{err: errors.New("api error for user 2")}, // user 2
			{jobs: []models.Job{job("u3-job", "serpapi")}}, // user 3
		},
	}

	sched := NewScheduler([]Source{src2}, ms, uf, time.Hour, nil)
	newJobs, err := sched.RunOnce(ctx)

	if err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	if len(newJobs) != 1 {
		t.Errorf("len(newJobs) = %d, want 1 (only user 3's job)", len(newJobs))
	}
	if len(ms.created) == 1 && ms.created[0].userID != 3 {
		t.Errorf("created[0].userID = %d, want 3", ms.created[0].userID)
	}
}

// TestQA_Scheduler_UserFilterToSearchFilter verifies that all fields are
// correctly mapped from UserSearchFilter to SearchFilter.
func TestQA_Scheduler_UserFilterToSearchFilter(t *testing.T) {
	uf := models.UserSearchFilter{
		ID:        42,
		UserID:    7,
		Keywords:  "senior golang",
		Location:  "Remote, USA",
		MinSalary: 150000,
		MaxSalary: 250000,
		Title:     "Staff Engineer",
	}

	sf := userFilterToSearchFilter(uf)

	if sf.Keywords != uf.Keywords {
		t.Errorf("Keywords = %q, want %q", sf.Keywords, uf.Keywords)
	}
	if sf.Location != uf.Location {
		t.Errorf("Location = %q, want %q", sf.Location, uf.Location)
	}
	if sf.MinSalary != uf.MinSalary {
		t.Errorf("MinSalary = %d, want %d", sf.MinSalary, uf.MinSalary)
	}
	if sf.MaxSalary != uf.MaxSalary {
		t.Errorf("MaxSalary = %d, want %d", sf.MaxSalary, uf.MaxSalary)
	}
	if sf.Title != uf.Title {
		t.Errorf("Title = %q, want %q", sf.Title, uf.Title)
	}
}

// ─── Helper mock that supports per-call errors ─────────────────────────────

type mockCallResult struct {
	jobs []models.Job
	err  error
}

type mockSourceWithErrors struct {
	callResults []mockCallResult
	callIndex   int
}

func (m *mockSourceWithErrors) Name() string { return "mock-errors" }

func (m *mockSourceWithErrors) Search(_ context.Context, _ models.SearchFilter) ([]models.Job, error) {
	if m.callIndex >= len(m.callResults) {
		return nil, nil
	}
	r := m.callResults[m.callIndex]
	m.callIndex++
	return r.jobs, r.err
}
