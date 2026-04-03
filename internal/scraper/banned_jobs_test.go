package scraper

import (
	"fmt"
	"testing"

	"github.com/whinchman/jobhuntr/internal/models"
)

// bannedTerm is a convenience constructor for test banned term values.
func bannedTerm(term string) models.UserBannedTerm {
	return models.UserBannedTerm{Term: term}
}

// jobFull creates a Job with title, company, and description populated.
func jobFull(title, company, description string) models.Job {
	return models.Job{
		ExternalID:  title + "-" + company,
		Source:      "serpapi",
		Title:       title,
		Company:     company,
		Description: description,
		Status:      models.StatusDiscovered,
	}
}

// ─── filterBannedJobs ─────────────────────────────────────────────────────────

func TestFilterBannedJobs(t *testing.T) {
	t.Run("empty terms returns all jobs unchanged", func(t *testing.T) {
		jobs := []models.Job{
			jobFull("Go Engineer", "Acme", "Build cool stuff"),
			jobFull("Python Dev", "Google LLC", "ML work"),
		}
		got := filterBannedJobs(jobs, nil)
		if len(got) != 2 {
			t.Errorf("len(got) = %d, want 2", len(got))
		}
	})

	t.Run("empty terms slice (non-nil) returns all jobs unchanged", func(t *testing.T) {
		jobs := []models.Job{
			jobFull("Go Engineer", "Acme", "Build cool stuff"),
		}
		got := filterBannedJobs(jobs, []models.UserBannedTerm{})
		if len(got) != 1 {
			t.Errorf("len(got) = %d, want 1", len(got))
		}
	})

	t.Run("empty job list returns empty slice", func(t *testing.T) {
		terms := []models.UserBannedTerm{bannedTerm("Google")}
		got := filterBannedJobs([]models.Job{}, terms)
		if len(got) != 0 {
			t.Errorf("len(got) = %d, want 0", len(got))
		}
	})

	t.Run("filters job matching banned term in title", func(t *testing.T) {
		jobs := []models.Job{
			jobFull("Amazon SDE", "TechCorp", "Backend engineering"),
			jobFull("Go Engineer", "Acme", "Build cool stuff"),
		}
		terms := []models.UserBannedTerm{bannedTerm("Amazon")}
		got := filterBannedJobs(jobs, terms)
		if len(got) != 1 {
			t.Fatalf("len(got) = %d, want 1", len(got))
		}
		if got[0].Title != "Go Engineer" {
			t.Errorf("got[0].Title = %q, want Go Engineer", got[0].Title)
		}
	})

	t.Run("filters job matching banned term in company", func(t *testing.T) {
		jobs := []models.Job{
			jobFull("Staff Engineer", "Google LLC", "Infrastructure work"),
			jobFull("Go Engineer", "Acme", "Build cool stuff"),
		}
		terms := []models.UserBannedTerm{bannedTerm("Google")}
		got := filterBannedJobs(jobs, terms)
		if len(got) != 1 {
			t.Fatalf("len(got) = %d, want 1", len(got))
		}
		if got[0].Title != "Go Engineer" {
			t.Errorf("got[0].Title = %q, want Go Engineer", got[0].Title)
		}
	})

	t.Run("filters job matching banned term in description", func(t *testing.T) {
		jobs := []models.Job{
			jobFull("Engineer", "Acme", "Work with staffing agencies"),
			jobFull("Go Dev", "Acme", "Build cool stuff"),
		}
		terms := []models.UserBannedTerm{bannedTerm("staffing agencies")}
		got := filterBannedJobs(jobs, terms)
		if len(got) != 1 {
			t.Fatalf("len(got) = %d, want 1", len(got))
		}
		if got[0].Title != "Go Dev" {
			t.Errorf("got[0].Title = %q, want Go Dev", got[0].Title)
		}
	})

	t.Run("matching is case-insensitive (lowercase term matches mixed-case job)", func(t *testing.T) {
		jobs := []models.Job{
			jobFull("Python Dev", "Google LLC", "ML platform work"),
			jobFull("Go Engineer", "Acme", "Build cool stuff"),
		}
		terms := []models.UserBannedTerm{bannedTerm("google")}
		got := filterBannedJobs(jobs, terms)
		if len(got) != 1 {
			t.Fatalf("len(got) = %d, want 1 (case-insensitive match missed)", len(got))
		}
		if got[0].Title != "Go Engineer" {
			t.Errorf("got[0].Title = %q, want Go Engineer", got[0].Title)
		}
	})

	t.Run("matching is case-insensitive (uppercase term matches lowercase job)", func(t *testing.T) {
		jobs := []models.Job{
			jobFull("amazon sde", "amazon", "aws backend"),
			jobFull("Go Engineer", "Acme", "Build cool stuff"),
		}
		terms := []models.UserBannedTerm{bannedTerm("AMAZON")}
		got := filterBannedJobs(jobs, terms)
		if len(got) != 1 {
			t.Fatalf("len(got) = %d, want 1", len(got))
		}
	})

	t.Run("job not matching any term passes through", func(t *testing.T) {
		jobs := []models.Job{
			jobFull("Go Engineer", "Acme", "Build backend services"),
		}
		terms := []models.UserBannedTerm{bannedTerm("Facebook"), bannedTerm("Consulting")}
		got := filterBannedJobs(jobs, terms)
		if len(got) != 1 {
			t.Errorf("len(got) = %d, want 1 (non-matching job should pass through)", len(got))
		}
	})

	t.Run("multiple banned terms filter independently", func(t *testing.T) {
		jobs := []models.Job{
			jobFull("Python Dev", "Google LLC", "ML work"),
			jobFull("Staff Engineer", "Meta Platforms", "Social graph"),
			jobFull("Go Engineer", "Acme", "Build cool stuff"),
		}
		terms := []models.UserBannedTerm{bannedTerm("Google"), bannedTerm("Meta")}
		got := filterBannedJobs(jobs, terms)
		if len(got) != 1 {
			t.Fatalf("len(got) = %d, want 1", len(got))
		}
		if got[0].Title != "Go Engineer" {
			t.Errorf("got[0].Title = %q, want Go Engineer", got[0].Title)
		}
	})

	t.Run("all jobs banned returns empty slice", func(t *testing.T) {
		jobs := []models.Job{
			jobFull("Python Dev", "Google LLC", "ML work"),
			jobFull("Staff Engineer", "Meta Platforms", "Social graph"),
		}
		terms := []models.UserBannedTerm{bannedTerm("Google"), bannedTerm("Meta")}
		got := filterBannedJobs(jobs, terms)
		if len(got) != 0 {
			t.Errorf("len(got) = %d, want 0", len(got))
		}
	})

	t.Run("substring match works (partial term match)", func(t *testing.T) {
		// "Corp" should match "TechCorp" in company
		jobs := []models.Job{
			jobFull("Engineer", "TechCorp", "Backend work"),
			jobFull("Go Dev", "Acme", "Build stuff"),
		}
		terms := []models.UserBannedTerm{bannedTerm("Corp")}
		got := filterBannedJobs(jobs, terms)
		if len(got) != 1 {
			t.Fatalf("len(got) = %d, want 1 (substring match in company)", len(got))
		}
		if got[0].Title != "Go Dev" {
			t.Errorf("got[0].Title = %q, want Go Dev", got[0].Title)
		}
	})
}

// ─── RunOnce with banned term filtering ───────────────────────────────────────

// mockUserFilterReaderWithBanned extends mockUserFilterReader to support
// per-user banned terms.
type mockUserFilterReaderWithBanned struct {
	mockUserFilterReader
	bannedTerms map[int64][]models.UserBannedTerm
	bannedErr   error
}

func (m *mockUserFilterReaderWithBanned) ListUserBannedTerms(_ context.Context, userID int64) ([]models.UserBannedTerm, error) {
	if m.bannedErr != nil {
		return nil, m.bannedErr
	}
	return m.bannedTerms[userID], nil
}

func TestScheduler_RunOnce_BannedTermFiltering(t *testing.T) {
	t.Run("banned jobs are excluded before CreateJob", func(t *testing.T) {
		src := &mockSource{
			results: [][]models.Job{
				{
					jobFull("Amazon SDE", "Amazon", "AWS backend work"),
					jobFull("Go Engineer", "Acme Corp", "Build cool stuff"),
				},
			},
		}
		ms := newMockStore()
		uf := &mockUserFilterReaderWithBanned{
			mockUserFilterReader: mockUserFilterReader{
				users: []int64{1},
				filters: map[int64][]models.UserSearchFilter{
					1: {userFilter("golang")},
				},
			},
			bannedTerms: map[int64][]models.UserBannedTerm{
				1: {bannedTerm("Amazon")},
			},
		}

		sched := NewScheduler(src, ms, uf, 0, nil)
		newJobs, err := sched.RunOnce(t.Context())
		if err != nil {
			t.Fatalf("RunOnce error = %v", err)
		}

		// Only "Go Engineer" should be returned; Amazon SDE is banned.
		if len(newJobs) != 1 {
			t.Fatalf("len(newJobs) = %d, want 1", len(newJobs))
		}
		if newJobs[0].Title != "Go Engineer" {
			t.Errorf("newJobs[0].Title = %q, want Go Engineer", newJobs[0].Title)
		}

		// Only 1 job should be created in the store.
		if len(ms.created) != 1 {
			t.Errorf("created = %d, want 1", len(ms.created))
		}
	})

	t.Run("ListUserBannedTerms error is non-fatal — all jobs pass through", func(t *testing.T) {
		src := &mockSource{
			results: [][]models.Job{
				{
					jobFull("Go Engineer", "Acme", "Backend"),
					jobFull("Python Dev", "Google", "ML"),
				},
			},
		}
		ms := newMockStore()
		uf := &mockUserFilterReaderWithBanned{
			mockUserFilterReader: mockUserFilterReader{
				users: []int64{1},
				filters: map[int64][]models.UserSearchFilter{
					1: {userFilter("golang")},
				},
			},
			bannedErr: errBannedLookupFailed,
		}

		sched := NewScheduler(src, ms, uf, 0, nil)
		newJobs, err := sched.RunOnce(t.Context())
		if err != nil {
			t.Fatalf("RunOnce error = %v", err)
		}

		// Non-fatal: both jobs should still be stored.
		if len(newJobs) != 2 {
			t.Errorf("len(newJobs) = %d, want 2 (banned term error is non-fatal)", len(newJobs))
		}
	})

	t.Run("user with no banned terms sees all jobs", func(t *testing.T) {
		src := &mockSource{
			results: [][]models.Job{
				{
					jobFull("Go Engineer", "Acme", "Backend"),
					jobFull("Python Dev", "Google", "ML"),
				},
			},
		}
		ms := newMockStore()
		uf := &mockUserFilterReaderWithBanned{
			mockUserFilterReader: mockUserFilterReader{
				users: []int64{1},
				filters: map[int64][]models.UserSearchFilter{
					1: {userFilter("golang")},
				},
			},
			bannedTerms: map[int64][]models.UserBannedTerm{
				1: {}, // empty — no banned terms
			},
		}

		sched := NewScheduler(src, ms, uf, 0, nil)
		newJobs, err := sched.RunOnce(t.Context())
		if err != nil {
			t.Fatalf("RunOnce error = %v", err)
		}
		if len(newJobs) != 2 {
			t.Errorf("len(newJobs) = %d, want 2", len(newJobs))
		}
	})
}

// errBannedLookupFailed is a sentinel error for banned term lookup failures in tests.
var errBannedLookupFailed = fmt.Errorf("banned term lookup failed")
