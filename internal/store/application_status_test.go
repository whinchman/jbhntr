package store

import (
	"context"
	"testing"

	"github.com/whinchman/jobhuntr/internal/models"
)

// ─── UpdateApplicationStatus ─────────────────────────────────────────────────

// approvedJob creates a job in the approved pipeline stage (required for
// UpdateApplicationStatus to accept it).
func approvedJob(t *testing.T, s *Store, userID int64, ext string) *models.Job {
	t.Helper()
	ctx := context.Background()
	j := &models.Job{
		ExternalID: ext,
		Source:     "serpapi",
		Title:      "Test Job",
		Company:    "Acme",
		Location:   "Remote",
		Status:     models.StatusApproved,
	}
	inserted, err := s.CreateJob(ctx, userID, j)
	if err != nil {
		t.Fatalf("CreateJob error = %v", err)
	}
	if !inserted {
		t.Fatal("CreateJob returned false; expected new row")
	}
	return j
}

func TestUpdateApplicationStatus_SetsStatusAndTimestamp(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)

	u, err := s.UpsertUser(ctx, &models.User{Provider: "google", ProviderID: "as-u1", Email: "as1@test.com"})
	if err != nil {
		t.Fatalf("UpsertUser error = %v", err)
	}

	j := approvedJob(t, s, u.ID, "as-sets-1")

	if err := s.UpdateApplicationStatus(ctx, u.ID, j.ID, models.AppStatusApplied); err != nil {
		t.Fatalf("UpdateApplicationStatus error = %v", err)
	}

	got, err := s.GetJob(ctx, u.ID, j.ID)
	if err != nil {
		t.Fatalf("GetJob error = %v", err)
	}

	if got.ApplicationStatus != models.AppStatusApplied {
		t.Errorf("ApplicationStatus = %q, want %q", got.ApplicationStatus, models.AppStatusApplied)
	}
	if got.AppliedAt == nil {
		t.Error("AppliedAt should be non-nil after setting applied status")
	}
	if got.InterviewingAt != nil {
		t.Error("InterviewingAt should remain nil when status is applied")
	}
	if got.LostAt != nil {
		t.Error("LostAt should remain nil when status is applied")
	}
	if got.WonAt != nil {
		t.Error("WonAt should remain nil when status is applied")
	}
}

func TestUpdateApplicationStatus_PreservesOriginalTimestamp(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)

	u, err := s.UpsertUser(ctx, &models.User{Provider: "google", ProviderID: "as-u2", Email: "as2@test.com"})
	if err != nil {
		t.Fatalf("UpsertUser error = %v", err)
	}

	j := approvedJob(t, s, u.ID, "as-preserve-1")

	// Set to applied for the first time — stamps applied_at.
	if err := s.UpdateApplicationStatus(ctx, u.ID, j.ID, models.AppStatusApplied); err != nil {
		t.Fatalf("first UpdateApplicationStatus error = %v", err)
	}

	first, err := s.GetJob(ctx, u.ID, j.ID)
	if err != nil {
		t.Fatalf("GetJob error = %v", err)
	}
	if first.AppliedAt == nil {
		t.Fatal("AppliedAt should be set after first update")
	}
	originalAppliedAt := *first.AppliedAt

	// Advance to interviewing, then back to applied — applied_at must not change.
	if err := s.UpdateApplicationStatus(ctx, u.ID, j.ID, models.AppStatusInterviewing); err != nil {
		t.Fatalf("second UpdateApplicationStatus error = %v", err)
	}
	if err := s.UpdateApplicationStatus(ctx, u.ID, j.ID, models.AppStatusApplied); err != nil {
		t.Fatalf("third UpdateApplicationStatus error = %v", err)
	}

	second, err := s.GetJob(ctx, u.ID, j.ID)
	if err != nil {
		t.Fatalf("GetJob error = %v", err)
	}
	if second.AppliedAt == nil {
		t.Fatal("AppliedAt should still be set")
	}
	if !second.AppliedAt.Equal(originalAppliedAt) {
		t.Errorf("AppliedAt changed: got %v, want %v (original should be preserved)", second.AppliedAt, originalAppliedAt)
	}
}

func TestUpdateApplicationStatus_RejectsNonApprovedJob(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)

	u, err := s.UpsertUser(ctx, &models.User{Provider: "google", ProviderID: "as-u3", Email: "as3@test.com"})
	if err != nil {
		t.Fatalf("UpsertUser error = %v", err)
	}

	// Use a job in discovered status — not in the pipeline.
	j := &models.Job{
		ExternalID: "as-reject-1",
		Source:     "serpapi",
		Title:      "Test Job",
		Company:    "Acme",
		Location:   "Remote",
		Status:     models.StatusDiscovered,
	}
	s.CreateJob(ctx, u.ID, j)

	err = s.UpdateApplicationStatus(ctx, u.ID, j.ID, models.AppStatusApplied)
	if err == nil {
		t.Error("UpdateApplicationStatus should return an error for a job in discovered status")
	}

	// Also test notified status.
	j2 := &models.Job{
		ExternalID: "as-reject-2",
		Source:     "serpapi",
		Title:      "Test Job 2",
		Company:    "Acme",
		Location:   "Remote",
		Status:     models.StatusNotified,
	}
	s.CreateJob(ctx, u.ID, j2)

	err = s.UpdateApplicationStatus(ctx, u.ID, j2.ID, models.AppStatusApplied)
	if err == nil {
		t.Error("UpdateApplicationStatus should return an error for a job in notified status")
	}

	// Rejected should also be disallowed.
	j3 := &models.Job{
		ExternalID: "as-reject-3",
		Source:     "serpapi",
		Title:      "Test Job 3",
		Company:    "Acme",
		Location:   "Remote",
		Status:     models.StatusRejected,
	}
	s.CreateJob(ctx, u.ID, j3)

	err = s.UpdateApplicationStatus(ctx, u.ID, j3.ID, models.AppStatusApplied)
	if err == nil {
		t.Error("UpdateApplicationStatus should return an error for a job in rejected status")
	}
}

// ─── ListJobs ApplicationStatus filter ────────────────────────────────────────

func TestListJobsByApplicationStatus(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)

	u, err := s.UpsertUser(ctx, &models.User{Provider: "google", ProviderID: "as-list-u1", Email: "aslist@test.com"})
	if err != nil {
		t.Fatalf("UpsertUser error = %v", err)
	}

	// Create three approved jobs.
	j1 := approvedJob(t, s, u.ID, "list-as-1")
	j2 := approvedJob(t, s, u.ID, "list-as-2")
	j3 := approvedJob(t, s, u.ID, "list-as-3")

	// Set different application statuses.
	if err := s.UpdateApplicationStatus(ctx, u.ID, j1.ID, models.AppStatusApplied); err != nil {
		t.Fatalf("UpdateApplicationStatus j1 error = %v", err)
	}
	if err := s.UpdateApplicationStatus(ctx, u.ID, j2.ID, models.AppStatusApplied); err != nil {
		t.Fatalf("UpdateApplicationStatus j2 error = %v", err)
	}
	if err := s.UpdateApplicationStatus(ctx, u.ID, j3.ID, models.AppStatusInterviewing); err != nil {
		t.Fatalf("UpdateApplicationStatus j3 error = %v", err)
	}

	t.Run("filter by applied returns two jobs", func(t *testing.T) {
		jobs, err := s.ListJobs(ctx, u.ID, ListJobsFilter{ApplicationStatus: models.AppStatusApplied})
		if err != nil {
			t.Fatalf("ListJobs error = %v", err)
		}
		if len(jobs) != 2 {
			t.Errorf("len = %d, want 2", len(jobs))
		}
		for _, j := range jobs {
			if j.ApplicationStatus != models.AppStatusApplied {
				t.Errorf("job %d has ApplicationStatus=%q, want %q", j.ID, j.ApplicationStatus, models.AppStatusApplied)
			}
		}
	})

	t.Run("filter by interviewing returns one job", func(t *testing.T) {
		jobs, err := s.ListJobs(ctx, u.ID, ListJobsFilter{ApplicationStatus: models.AppStatusInterviewing})
		if err != nil {
			t.Fatalf("ListJobs error = %v", err)
		}
		if len(jobs) != 1 {
			t.Errorf("len = %d, want 1", len(jobs))
		}
		if len(jobs) == 1 && jobs[0].ID != j3.ID {
			t.Errorf("job ID = %d, want %d", jobs[0].ID, j3.ID)
		}
	})

	t.Run("filter by won returns no jobs", func(t *testing.T) {
		jobs, err := s.ListJobs(ctx, u.ID, ListJobsFilter{ApplicationStatus: models.AppStatusWon})
		if err != nil {
			t.Fatalf("ListJobs error = %v", err)
		}
		if len(jobs) != 0 {
			t.Errorf("len = %d, want 0", len(jobs))
		}
	})

	t.Run("no application status filter returns all three", func(t *testing.T) {
		jobs, err := s.ListJobs(ctx, u.ID, ListJobsFilter{})
		if err != nil {
			t.Fatalf("ListJobs error = %v", err)
		}
		if len(jobs) != 3 {
			t.Errorf("len = %d, want 3", len(jobs))
		}
	})
}

// ─── UpdateApplicationStatus: user scoping ────────────────────────────────────

// TestUpdateApplicationStatus_UserScoping verifies that UpdateApplicationStatus
// enforces user ownership: a second user cannot update a job that belongs to
// the first user. The store uses GetJob internally, which scopes by user_id,
// so the call must return an error and leave the job unchanged.
func TestUpdateApplicationStatus_UserScoping(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)

	owner, err := s.UpsertUser(ctx, &models.User{Provider: "google", ProviderID: "scope-u1", Email: "scope1@test.com"})
	if err != nil {
		t.Fatalf("UpsertUser owner error = %v", err)
	}

	attacker, err := s.UpsertUser(ctx, &models.User{Provider: "google", ProviderID: "scope-u2", Email: "scope2@test.com"})
	if err != nil {
		t.Fatalf("UpsertUser attacker error = %v", err)
	}

	// Create an approved job owned by owner.
	j := approvedJob(t, s, owner.ID, "scope-isolation-1")

	// attacker attempts to update the owner's job — must fail.
	err = s.UpdateApplicationStatus(ctx, attacker.ID, j.ID, models.AppStatusApplied)
	if err == nil {
		t.Error("UpdateApplicationStatus should return an error when userID does not match job owner")
	}

	// Confirm the job is unchanged.
	got, err := s.GetJob(ctx, owner.ID, j.ID)
	if err != nil {
		t.Fatalf("GetJob error = %v", err)
	}
	if got.ApplicationStatus != "" {
		t.Errorf("ApplicationStatus = %q, want empty (unchanged after cross-user attempt)", got.ApplicationStatus)
	}
}
