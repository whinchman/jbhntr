package store

import (
	"context"
	"testing"
	"time"

	"github.com/whinchman/jobhuntr/internal/models"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open(:memory:) error = %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func sampleJob(ext, source string) *models.Job {
	return &models.Job{
		ExternalID:  ext,
		Source:      source,
		Title:       "Senior Go Engineer",
		Company:     "Acme Corp",
		Location:    "Remote",
		Description: "Build cool stuff in Go.",
		Salary:      "$150k-$200k",
		ApplyURL:    "https://example.com/apply",
		Status:      models.StatusDiscovered,
	}
}

// ─── CreateJob ────────────────────────────────────────────────────────────────

func TestCreateJob(t *testing.T) {
	ctx := context.Background()

	t.Run("inserts new job and returns true", func(t *testing.T) {
		s := openTestStore(t)
		job := sampleJob("ext-1", "serpapi")

		inserted, err := s.CreateJob(ctx, 0, job)
		if err != nil {
			t.Fatalf("CreateJob error = %v", err)
		}
		if !inserted {
			t.Error("CreateJob returned false for new job, want true")
		}
		if job.ID == 0 {
			t.Error("job.ID not set after insert")
		}
	})

	t.Run("deduplicates on external_id+source and returns false", func(t *testing.T) {
		s := openTestStore(t)
		job1 := sampleJob("ext-dup", "serpapi")
		job2 := sampleJob("ext-dup", "serpapi")

		inserted1, _ := s.CreateJob(ctx, 0, job1)
		inserted2, err := s.CreateJob(ctx, 0, job2)

		if err != nil {
			t.Fatalf("CreateJob (dup) error = %v", err)
		}
		if !inserted1 {
			t.Error("first insert returned false")
		}
		if inserted2 {
			t.Error("duplicate insert returned true, want false")
		}
	})

	t.Run("same external_id different source is allowed", func(t *testing.T) {
		s := openTestStore(t)
		j1 := sampleJob("ext-x", "serpapi")
		j2 := sampleJob("ext-x", "linkedin")

		i1, _ := s.CreateJob(ctx, 0, j1)
		i2, err := s.CreateJob(ctx, 0, j2)

		if err != nil {
			t.Fatalf("CreateJob error = %v", err)
		}
		if !i1 || !i2 {
			t.Error("both different-source inserts should return true")
		}
	})
}

// ─── GetJob ───────────────────────────────────────────────────────────────────

func TestGetJob(t *testing.T) {
	ctx := context.Background()

	t.Run("returns inserted job", func(t *testing.T) {
		s := openTestStore(t)
		job := sampleJob("ext-get", "serpapi")
		s.CreateJob(ctx, 0, job)

		got, err := s.GetJob(ctx, 0, job.ID)
		if err != nil {
			t.Fatalf("GetJob error = %v", err)
		}
		if got.ExternalID != job.ExternalID {
			t.Errorf("ExternalID = %q, want %q", got.ExternalID, job.ExternalID)
		}
		if got.Title != job.Title {
			t.Errorf("Title = %q, want %q", got.Title, job.Title)
		}
		if got.Status != models.StatusDiscovered {
			t.Errorf("Status = %q, want discovered", got.Status)
		}
	})

	t.Run("returns error for unknown id", func(t *testing.T) {
		s := openTestStore(t)
		_, err := s.GetJob(ctx, 0, 99999)
		if err == nil {
			t.Error("GetJob(nonexistent) expected error, got nil")
		}
	})
}

// ─── ListJobs ─────────────────────────────────────────────────────────────────

func TestListJobs(t *testing.T) {
	ctx := context.Background()

	setup := func(t *testing.T) *Store {
		s := openTestStore(t)
		jobs := []*models.Job{
			{ExternalID: "a", Source: "serpapi", Title: "Go Engineer", Company: "Acme", Location: "Remote", Status: models.StatusDiscovered},
			{ExternalID: "b", Source: "serpapi", Title: "Python Dev", Company: "Beta", Location: "NYC", Status: models.StatusNotified},
			{ExternalID: "c", Source: "serpapi", Title: "Staff Engineer", Company: "Acme", Location: "Remote", Status: models.StatusApproved},
		}
		for _, j := range jobs {
			s.CreateJob(ctx, 0, j)
		}
		return s
	}

	t.Run("filter by status", func(t *testing.T) {
		s := setup(t)
		results, err := s.ListJobs(ctx, 0, ListJobsFilter{Status: models.StatusDiscovered})
		if err != nil {
			t.Fatalf("ListJobs error = %v", err)
		}
		if len(results) != 1 {
			t.Errorf("len = %d, want 1", len(results))
		}
		if results[0].ExternalID != "a" {
			t.Errorf("ExternalID = %q, want a", results[0].ExternalID)
		}
	})

	t.Run("no filter returns all", func(t *testing.T) {
		s := setup(t)
		results, err := s.ListJobs(ctx, 0, ListJobsFilter{})
		if err != nil {
			t.Fatalf("ListJobs error = %v", err)
		}
		if len(results) != 3 {
			t.Errorf("len = %d, want 3", len(results))
		}
	})

	t.Run("text search on title", func(t *testing.T) {
		s := setup(t)
		results, err := s.ListJobs(ctx, 0, ListJobsFilter{Search: "Staff"})
		if err != nil {
			t.Fatalf("ListJobs error = %v", err)
		}
		if len(results) != 1 {
			t.Errorf("len = %d, want 1", len(results))
		}
	})

	t.Run("pagination limit", func(t *testing.T) {
		s := setup(t)
		results, err := s.ListJobs(ctx, 0, ListJobsFilter{Limit: 2})
		if err != nil {
			t.Fatalf("ListJobs error = %v", err)
		}
		if len(results) != 2 {
			t.Errorf("len = %d, want 2", len(results))
		}
	})

	t.Run("pagination offset", func(t *testing.T) {
		s := setup(t)
		all, _ := s.ListJobs(ctx, 0, ListJobsFilter{})
		page2, err := s.ListJobs(ctx, 0, ListJobsFilter{Limit: 2, Offset: 2})
		if err != nil {
			t.Fatalf("ListJobs error = %v", err)
		}
		if len(page2) != 1 {
			t.Errorf("page2 len = %d, want 1", len(page2))
		}
		if page2[0].ID == all[0].ID || page2[0].ID == all[1].ID {
			t.Error("page2 should not contain first-page items")
		}
	})
}

// ─── UpdateJobStatus ──────────────────────────────────────────────────────────

func TestUpdateJobStatus(t *testing.T) {
	ctx := context.Background()

	transitions := []struct {
		from models.JobStatus
		to   models.JobStatus
		ok   bool
	}{
		{models.StatusDiscovered, models.StatusNotified, true},
		{models.StatusDiscovered, models.StatusRejected, true},
		{models.StatusNotified, models.StatusApproved, true},
		{models.StatusNotified, models.StatusRejected, true},
		{models.StatusApproved, models.StatusGenerating, true},
		{models.StatusGenerating, models.StatusComplete, true},
		{models.StatusGenerating, models.StatusFailed, true},
		{models.StatusFailed, models.StatusGenerating, true},
		// invalid
		{models.StatusDiscovered, models.StatusComplete, false},
		{models.StatusComplete, models.StatusDiscovered, false},
		{models.StatusRejected, models.StatusApproved, false},
	}

	for _, tc := range transitions {
		name := string(tc.from) + "→" + string(tc.to)
		t.Run(name, func(t *testing.T) {
			s := openTestStore(t)
			job := &models.Job{
				ExternalID: "trans-test", Source: "serpapi",
				Title: "T", Company: "C", Location: "L", Status: tc.from,
			}
			s.CreateJob(ctx, 0, job)

			err := s.UpdateJobStatus(ctx, 0, job.ID, tc.to)
			if tc.ok && err != nil {
				t.Errorf("UpdateJobStatus(%s→%s) unexpected error: %v", tc.from, tc.to, err)
			}
			if !tc.ok && err == nil {
				t.Errorf("UpdateJobStatus(%s→%s) expected error, got nil", tc.from, tc.to)
			}
		})
	}
}

// ─── UpdateJobGenerated ───────────────────────────────────────────────────────

func TestUpdateJobGenerated(t *testing.T) {
	ctx := context.Background()

	t.Run("sets generated fields", func(t *testing.T) {
		s := openTestStore(t)
		job := sampleJob("gen-test", "serpapi")
		s.CreateJob(ctx, 0, job)

		err := s.UpdateJobGenerated(ctx, 0, job.ID, "<h1>Resume</h1>", "<h1>Cover</h1>", "/out/resume.pdf", "/out/cover.pdf")
		if err != nil {
			t.Fatalf("UpdateJobGenerated error = %v", err)
		}

		got, _ := s.GetJob(ctx, 0, job.ID)
		if got.ResumeHTML != "<h1>Resume</h1>" {
			t.Errorf("ResumeHTML = %q", got.ResumeHTML)
		}
		if got.ResumePDF != "/out/resume.pdf" {
			t.Errorf("ResumePDF = %q", got.ResumePDF)
		}
	})
}

// ─── CreateScrapeRun ──────────────────────────────────────────────────────────

func TestCreateScrapeRun(t *testing.T) {
	ctx := context.Background()

	t.Run("inserts scrape run", func(t *testing.T) {
		s := openTestStore(t)
		run := &ScrapeRun{
			Source:         "serpapi",
			FilterKeywords: "senior golang engineer",
			JobsFound:      10,
			JobsNew:        3,
			StartedAt:      time.Now().UTC(),
			FinishedAt:     time.Now().UTC(),
		}
		if err := s.CreateScrapeRun(ctx, run); err != nil {
			t.Fatalf("CreateScrapeRun error = %v", err)
		}
		if run.ID == 0 {
			t.Error("ScrapeRun.ID not set after insert")
		}
	})

	t.Run("inserts scrape run with error", func(t *testing.T) {
		s := openTestStore(t)
		run := &ScrapeRun{
			Source:     "serpapi",
			StartedAt:  time.Now().UTC(),
			FinishedAt: time.Now().UTC(),
			Error:      "rate limit exceeded",
		}
		if err := s.CreateScrapeRun(ctx, run); err != nil {
			t.Fatalf("CreateScrapeRun error = %v", err)
		}
	})
}

// ─── CreateJob_WithUserID ────────────────────────────────────────────────────

func TestCreateJob_WithUserID(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)

	// Create a user first.
	user := &models.User{
		Provider:   "google",
		ProviderID: "user-1",
		Email:      "test@example.com",
	}
	user, err := s.UpsertUser(ctx, user)
	if err != nil {
		t.Fatalf("UpsertUser error = %v", err)
	}

	job := sampleJob("ext-user", "serpapi")
	inserted, err := s.CreateJob(ctx, user.ID, job)
	if err != nil {
		t.Fatalf("CreateJob error = %v", err)
	}
	if !inserted {
		t.Error("CreateJob returned false, want true")
	}

	got, err := s.GetJob(ctx, user.ID, job.ID)
	if err != nil {
		t.Fatalf("GetJob error = %v", err)
	}
	if got.UserID != user.ID {
		t.Errorf("UserID = %d, want %d", got.UserID, user.ID)
	}
}

// ─── ListJobs_UserIsolation ─────────────────────────────────────────────────

func TestListJobs_UserIsolation(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)

	// Create two users.
	u1, err := s.UpsertUser(ctx, &models.User{Provider: "google", ProviderID: "u1", Email: "u1@test.com"})
	if err != nil {
		t.Fatalf("UpsertUser u1 error = %v", err)
	}
	u2, err := s.UpsertUser(ctx, &models.User{Provider: "google", ProviderID: "u2", Email: "u2@test.com"})
	if err != nil {
		t.Fatalf("UpsertUser u2 error = %v", err)
	}

	// Create jobs for each user.
	j1 := sampleJob("iso-1", "serpapi")
	j2 := sampleJob("iso-2", "serpapi")
	s.CreateJob(ctx, u1.ID, j1)
	s.CreateJob(ctx, u2.ID, j2)

	t.Run("user1 sees only own jobs", func(t *testing.T) {
		jobs, err := s.ListJobs(ctx, u1.ID, ListJobsFilter{})
		if err != nil {
			t.Fatalf("ListJobs error = %v", err)
		}
		if len(jobs) != 1 {
			t.Fatalf("len = %d, want 1", len(jobs))
		}
		if jobs[0].ExternalID != "iso-1" {
			t.Errorf("ExternalID = %q, want iso-1", jobs[0].ExternalID)
		}
	})

	t.Run("user2 sees only own jobs", func(t *testing.T) {
		jobs, err := s.ListJobs(ctx, u2.ID, ListJobsFilter{})
		if err != nil {
			t.Fatalf("ListJobs error = %v", err)
		}
		if len(jobs) != 1 {
			t.Fatalf("len = %d, want 1", len(jobs))
		}
		if jobs[0].ExternalID != "iso-2" {
			t.Errorf("ExternalID = %q, want iso-2", jobs[0].ExternalID)
		}
	})

	t.Run("userID 0 sees all jobs", func(t *testing.T) {
		jobs, err := s.ListJobs(ctx, 0, ListJobsFilter{})
		if err != nil {
			t.Fatalf("ListJobs error = %v", err)
		}
		if len(jobs) != 2 {
			t.Errorf("len = %d, want 2", len(jobs))
		}
	})
}
