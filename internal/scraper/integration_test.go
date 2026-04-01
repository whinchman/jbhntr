package scraper

import (
	"context"
	"testing"
	"time"

	"github.com/whinchman/jobhuntr/internal/models"
	"github.com/whinchman/jobhuntr/internal/store"
)

func TestIntegration_SchedulerCreatesJobsForCorrectUser(t *testing.T) {
	ctx := context.Background()

	// Open a real in-memory SQLite store.
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	// Create two users.
	userA, err := db.UpsertUser(ctx, &models.User{
		Provider: "google", ProviderID: "ua", Email: "a@test.com", DisplayName: "User A",
	})
	if err != nil {
		t.Fatalf("upsert user A: %v", err)
	}

	userB, err := db.UpsertUser(ctx, &models.User{
		Provider: "google", ProviderID: "ub", Email: "b@test.com", DisplayName: "User B",
	})
	if err != nil {
		t.Fatalf("upsert user B: %v", err)
	}

	// Create search filters for each user.
	if err := db.CreateUserFilter(ctx, userA.ID, &models.UserSearchFilter{Keywords: "golang"}); err != nil {
		t.Fatalf("create filter A: %v", err)
	}
	if err := db.CreateUserFilter(ctx, userB.ID, &models.UserSearchFilter{Keywords: "python"}); err != nil {
		t.Fatalf("create filter B: %v", err)
	}

	// Mock source returns different jobs depending on call index.
	// ListActiveUserIDs returns IDs in ascending order, and each user
	// has one filter, so call sequence is: userA's filter, then userB's filter.
	src := &mockSource{
		results: [][]models.Job{
			{{ExternalID: "go-1", Source: "serpapi", Title: "Go Dev", Company: "GoCo", Status: models.StatusDiscovered}},
			{{ExternalID: "py-1", Source: "serpapi", Title: "Python Dev", Company: "PyCo", Status: models.StatusDiscovered}},
		},
	}

	sched := NewScheduler(src, db, db, time.Hour, nil)

	newJobs, err := sched.RunOnce(ctx)
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	t.Run("returns correct number of new jobs", func(t *testing.T) {
		if len(newJobs) != 2 {
			t.Errorf("len(newJobs) = %d, want 2", len(newJobs))
		}
	})

	t.Run("user A has only Go job", func(t *testing.T) {
		jobsA, err := db.ListJobs(ctx, userA.ID, store.ListJobsFilter{})
		if err != nil {
			t.Fatalf("ListJobs(A): %v", err)
		}
		if len(jobsA) != 1 {
			t.Fatalf("user A jobs = %d, want 1", len(jobsA))
		}
		if jobsA[0].ExternalID != "go-1" {
			t.Errorf("user A job ExternalID = %q, want go-1", jobsA[0].ExternalID)
		}
		if jobsA[0].Title != "Go Dev" {
			t.Errorf("user A job Title = %q, want Go Dev", jobsA[0].Title)
		}
	})

	t.Run("user B has only Python job", func(t *testing.T) {
		jobsB, err := db.ListJobs(ctx, userB.ID, store.ListJobsFilter{})
		if err != nil {
			t.Fatalf("ListJobs(B): %v", err)
		}
		if len(jobsB) != 1 {
			t.Fatalf("user B jobs = %d, want 1", len(jobsB))
		}
		if jobsB[0].ExternalID != "py-1" {
			t.Errorf("user B job ExternalID = %q, want py-1", jobsB[0].ExternalID)
		}
		if jobsB[0].Title != "Python Dev" {
			t.Errorf("user B job Title = %q, want Python Dev", jobsB[0].Title)
		}
	})

	t.Run("user A does not have Python job", func(t *testing.T) {
		jobsA, err := db.ListJobs(ctx, userA.ID, store.ListJobsFilter{})
		if err != nil {
			t.Fatalf("ListJobs(A): %v", err)
		}
		for _, j := range jobsA {
			if j.ExternalID == "py-1" {
				t.Error("user A should not have py-1 job")
			}
		}
	})

	t.Run("user B does not have Go job", func(t *testing.T) {
		jobsB, err := db.ListJobs(ctx, userB.ID, store.ListJobsFilter{})
		if err != nil {
			t.Fatalf("ListJobs(B): %v", err)
		}
		for _, j := range jobsB {
			if j.ExternalID == "go-1" {
				t.Error("user B should not have go-1 job")
			}
		}
	})

	t.Run("unscoped query returns all jobs", func(t *testing.T) {
		allJobs, err := db.ListJobs(ctx, 0, store.ListJobsFilter{})
		if err != nil {
			t.Fatalf("ListJobs(0): %v", err)
		}
		if len(allJobs) != 2 {
			t.Errorf("total jobs = %d, want 2", len(allJobs))
		}
	})
}
