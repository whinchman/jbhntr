package store

import (
	"context"
	"os"
	"testing"

	"github.com/whinchman/jobhuntr/internal/models"
)

// ─── QA: Adversarial tests for task4-multiuser-scraper ─────────────────────

// TestQA_UserWithFiltersButNoJobs verifies that a user who has filters
// configured but has never had a scrape run (no jobs in DB) can still
// be listed by ListActiveUserIDs and have their filters fetched.
func TestQA_UserWithFiltersButNoJobs(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)

	user, err := s.UpsertUser(ctx, &models.User{
		Provider:   "google",
		ProviderID: "qa-no-jobs",
		Email:      "nojobs@test.com",
	})
	if err != nil {
		t.Fatalf("UpsertUser error = %v", err)
	}

	err = s.CreateUserFilter(ctx, user.ID, &models.UserSearchFilter{
		Keywords: "golang remote",
		Location: "Remote",
	})
	if err != nil {
		t.Fatalf("CreateUserFilter error = %v", err)
	}

	// User should appear in active user list.
	ids, err := s.ListActiveUserIDs(ctx)
	if err != nil {
		t.Fatalf("ListActiveUserIDs error = %v", err)
	}
	if len(ids) != 1 || ids[0] != user.ID {
		t.Errorf("ListActiveUserIDs = %v, want [%d]", ids, user.ID)
	}

	// User should have filters.
	filters, err := s.ListUserFilters(ctx, user.ID)
	if err != nil {
		t.Fatalf("ListUserFilters error = %v", err)
	}
	if len(filters) != 1 {
		t.Fatalf("len(filters) = %d, want 1", len(filters))
	}
	if filters[0].Keywords != "golang remote" {
		t.Errorf("Keywords = %q, want golang remote", filters[0].Keywords)
	}

	// No jobs for this user.
	jobs, err := s.ListJobs(ctx, user.ID, ListJobsFilter{})
	if err != nil {
		t.Fatalf("ListJobs error = %v", err)
	}
	if len(jobs) != 0 {
		t.Errorf("len(jobs) = %d, want 0", len(jobs))
	}
}

// TestQA_ListActiveUserIDs_NoUsersWithFilters verifies that
// ListActiveUserIDs returns empty when no users have any filters.
func TestQA_ListActiveUserIDs_NoUsersWithFilters(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)

	// Create three users with no filters.
	for _, pid := range []string{"qa-empty-1", "qa-empty-2", "qa-empty-3"} {
		_, err := s.UpsertUser(ctx, &models.User{
			Provider:   "google",
			ProviderID: pid,
			Email:      pid + "@test.com",
		})
		if err != nil {
			t.Fatalf("UpsertUser(%s) error = %v", pid, err)
		}
	}

	ids, err := s.ListActiveUserIDs(ctx)
	if err != nil {
		t.Fatalf("ListActiveUserIDs error = %v", err)
	}
	if len(ids) != 0 {
		t.Errorf("ListActiveUserIDs = %v, want empty", ids)
	}
}

// TestQA_SameJobThreeUsers verifies that the same external job can be
// inserted for 3 different users without constraint violations. This is
// the core BUG-001 fix verification.
func TestQA_SameJobThreeUsers(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)

	users := make([]*models.User, 3)
	for i := 0; i < 3; i++ {
		u, err := s.UpsertUser(ctx, &models.User{
			Provider:   "google",
			ProviderID: "qa-3user-" + string(rune('a'+i)),
			Email:      string(rune('a'+i)) + "@test.com",
		})
		if err != nil {
			t.Fatalf("UpsertUser[%d] error = %v", i, err)
		}
		users[i] = u
	}

	// Insert the exact same external job for all 3 users.
	for i, u := range users {
		job := sampleJob("shared-ext-123", "serpapi")
		inserted, err := s.CreateJob(ctx, u.ID, job)
		if err != nil {
			t.Fatalf("CreateJob user[%d] error = %v", i, err)
		}
		if !inserted {
			t.Errorf("CreateJob user[%d] returned false, want true (per-user dedup should allow this)", i)
		}
	}

	// Verify all 3 rows exist.
	for i, u := range users {
		jobs, err := s.ListJobs(ctx, u.ID, ListJobsFilter{})
		if err != nil {
			t.Fatalf("ListJobs user[%d] error = %v", i, err)
		}
		if len(jobs) != 1 {
			t.Errorf("ListJobs user[%d] len = %d, want 1", i, len(jobs))
		}
		if len(jobs) > 0 && jobs[0].ExternalID != "shared-ext-123" {
			t.Errorf("ListJobs user[%d] ExternalID = %q, want shared-ext-123", i, jobs[0].ExternalID)
		}
	}

	// Verify total count with unscoped query is 3.
	allJobs, err := s.ListJobs(ctx, 0, ListJobsFilter{})
	if err != nil {
		t.Fatalf("ListJobs(0) error = %v", err)
	}
	if len(allJobs) != 3 {
		t.Errorf("total jobs = %d, want 3", len(allJobs))
	}
}

// TestQA_DeletedUserFiltersStillExist verifies behavior when a user is
// deleted but their filters still exist in the database (orphaned filters).
// Since user_search_filters has a FK to users, the user delete should be
// blocked by the FK constraint.
func TestQA_DeletedUserFiltersStillExist(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)

	user, err := s.UpsertUser(ctx, &models.User{
		Provider:   "google",
		ProviderID: "qa-delete-user",
		Email:      "delete@test.com",
	})
	if err != nil {
		t.Fatalf("UpsertUser error = %v", err)
	}

	err = s.CreateUserFilter(ctx, user.ID, &models.UserSearchFilter{Keywords: "orphan-test"})
	if err != nil {
		t.Fatalf("CreateUserFilter error = %v", err)
	}

	// Attempt to delete the user directly.
	_, deleteErr := s.db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", user.ID)

	// With FK on user_search_filters.user_id REFERENCES users(id), the
	// delete should be blocked (no CASCADE defined in migration 002).
	if deleteErr == nil {
		// The delete succeeded, which means orphaned filters exist.
		ids, err := s.ListActiveUserIDs(ctx)
		if err != nil {
			t.Fatalf("ListActiveUserIDs error = %v", err)
		}
		t.Logf("NOTE: User delete succeeded despite FK. Orphaned filters produce user_id=%d in ListActiveUserIDs: %v", user.ID, ids)
	} else {
		// FK violation blocked the delete. This is the expected behavior.
		t.Logf("User delete correctly blocked by FK constraint: %v", deleteErr)

		// Verify user and filters still exist.
		ids, err := s.ListActiveUserIDs(ctx)
		if err != nil {
			t.Fatalf("ListActiveUserIDs error = %v", err)
		}
		if len(ids) != 1 || ids[0] != user.ID {
			t.Errorf("ListActiveUserIDs = %v, want [%d]", ids, user.ID)
		}
	}
}

// TestQA_Migration004_DataSurvival verifies that existing job data
// survives across multiple Open calls (migration idempotency).
func TestQA_Migration004_DataSurvival(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping Postgres store tests")
	}

	ctx := context.Background()

	s, err := Open(dsn)
	if err != nil {
		t.Fatalf("Open error = %v", err)
	}

	// Create a user and add jobs with various statuses and data.
	user, err := s.UpsertUser(ctx, &models.User{
		Provider:   "google",
		ProviderID: "qa-mig-user",
		Email:      "mig@test.com",
	})
	if err != nil {
		t.Fatalf("UpsertUser error = %v", err)
	}

	testJobs := []*models.Job{
		{ExternalID: "mig-1", Source: "serpapi", Title: "Go Engineer", Company: "Acme", Location: "Remote", Status: models.StatusDiscovered, Description: "Build things"},
		{ExternalID: "mig-2", Source: "serpapi", Title: "Staff SRE", Company: "Beta Corp", Location: "NYC", Status: models.StatusDiscovered, Salary: "$200k"},
		{ExternalID: "mig-3", Source: "linkedin", Title: "Principal", Company: "Gamma", Location: "SF", Status: models.StatusDiscovered, ApplyURL: "https://gamma.com"},
	}

	for _, j := range testJobs {
		inserted, err := s.CreateJob(ctx, user.ID, j)
		if err != nil {
			t.Fatalf("CreateJob(%s) error = %v", j.ExternalID, err)
		}
		if !inserted {
			t.Fatalf("CreateJob(%s) returned false", j.ExternalID)
		}
	}

	// Also insert a legacy job (user_id=0).
	legacyJob := &models.Job{ExternalID: "legacy-1", Source: "serpapi", Title: "Legacy Job", Company: "Old Corp", Status: models.StatusDiscovered}
	inserted, err := s.CreateJob(ctx, 0, legacyJob)
	if err != nil {
		t.Fatalf("CreateJob(legacy) error = %v", err)
	}
	if !inserted {
		t.Fatal("CreateJob(legacy) returned false")
	}

	s.Close()

	// Reopen the database. Migrations should be idempotent.
	s2, err := Open(dsn)
	if err != nil {
		t.Fatalf("second Open error = %v", err)
	}
	defer s2.Close()

	// Verify all user jobs survive.
	userJobs, err := s2.ListJobs(ctx, user.ID, ListJobsFilter{})
	if err != nil {
		t.Fatalf("ListJobs(user) error = %v", err)
	}
	if len(userJobs) != 3 {
		t.Errorf("user jobs after reopen = %d, want 3", len(userJobs))
	}

	// Verify per-user dedup still works after reopen.
	user2, err := s2.UpsertUser(ctx, &models.User{
		Provider:   "google",
		ProviderID: "qa-mig-user2",
		Email:      "mig2@test.com",
	})
	if err != nil {
		t.Fatalf("UpsertUser user2 error = %v", err)
	}

	// Same external_id as user1's job, different user.
	dupJob := &models.Job{ExternalID: "mig-1", Source: "serpapi", Title: "Go Engineer", Company: "Acme", Location: "Remote", Status: models.StatusDiscovered}
	ins, err := s2.CreateJob(ctx, user2.ID, dupJob)
	if err != nil {
		t.Fatalf("CreateJob(user2 dup) error = %v", err)
	}
	if !ins {
		t.Error("per-user dedup should allow same external_id for different user after reopen")
	}
}

// TestQA_BUG001_TwoUsersSameJob is the explicit BUG-001 verification:
// create two users, add the same external job for both, verify no UNIQUE
// constraint violation.
func TestQA_BUG001_TwoUsersSameJob(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)

	u1, err := s.UpsertUser(ctx, &models.User{Provider: "google", ProviderID: "bug001-u1", Email: "b1@test.com"})
	if err != nil {
		t.Fatalf("UpsertUser u1 error = %v", err)
	}
	u2, err := s.UpsertUser(ctx, &models.User{Provider: "google", ProviderID: "bug001-u2", Email: "b2@test.com"})
	if err != nil {
		t.Fatalf("UpsertUser u2 error = %v", err)
	}

	// Both users discover the exact same job.
	j1 := sampleJob("bug001-shared", "serpapi")
	ins1, err := s.CreateJob(ctx, u1.ID, j1)
	if err != nil {
		t.Fatalf("CreateJob u1 error = %v", err)
	}
	if !ins1 {
		t.Error("CreateJob u1 returned false, want true")
	}

	j2 := sampleJob("bug001-shared", "serpapi")
	ins2, err := s.CreateJob(ctx, u2.ID, j2)
	if err != nil {
		t.Fatalf("CreateJob u2 error = %v (BUG-001 still present?)", err)
	}
	if !ins2 {
		t.Error("CreateJob u2 returned false, want true (BUG-001 still present: old UNIQUE constraint is blocking per-user dedup)")
	}

	// Verify both jobs exist in their respective user scopes.
	u1Jobs, _ := s.ListJobs(ctx, u1.ID, ListJobsFilter{})
	u2Jobs, _ := s.ListJobs(ctx, u2.ID, ListJobsFilter{})

	if len(u1Jobs) != 1 {
		t.Errorf("u1 jobs = %d, want 1", len(u1Jobs))
	}
	if len(u2Jobs) != 1 {
		t.Errorf("u2 jobs = %d, want 1", len(u2Jobs))
	}

	// Verify same-user dedup still works.
	j3 := sampleJob("bug001-shared", "serpapi")
	ins3, err := s.CreateJob(ctx, u1.ID, j3)
	if err != nil {
		t.Fatalf("CreateJob u1 dup error = %v", err)
	}
	if ins3 {
		t.Error("duplicate insert for same user should return false")
	}
}

// TestQA_PerUserDedupConsistency verifies that the dedup index
// correctly distinguishes (user_id, external_id, source) tuples.
func TestQA_PerUserDedupConsistency(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)

	u1, _ := s.UpsertUser(ctx, &models.User{Provider: "google", ProviderID: "dedup-c1", Email: "dc1@test.com"})
	u2, _ := s.UpsertUser(ctx, &models.User{Provider: "google", ProviderID: "dedup-c2", Email: "dc2@test.com"})

	// Same external_id, same source, different users -> both succeed.
	j1 := sampleJob("dedup-same", "serpapi")
	j2 := sampleJob("dedup-same", "serpapi")
	ins1, _ := s.CreateJob(ctx, u1.ID, j1)
	ins2, _ := s.CreateJob(ctx, u2.ID, j2)
	if !ins1 || !ins2 {
		t.Errorf("different users same job: ins1=%v ins2=%v, want both true", ins1, ins2)
	}

	// Same user, same external_id, different source -> both succeed.
	j3 := sampleJob("dedup-diff-src", "serpapi")
	j4 := sampleJob("dedup-diff-src", "linkedin")
	ins3, _ := s.CreateJob(ctx, u1.ID, j3)
	ins4, _ := s.CreateJob(ctx, u1.ID, j4)
	if !ins3 || !ins4 {
		t.Errorf("same user different source: ins3=%v ins4=%v, want both true", ins3, ins4)
	}

	// Same user, same external_id, same source -> dedup blocks.
	j5 := sampleJob("dedup-same", "serpapi")
	ins5, err := s.CreateJob(ctx, u1.ID, j5)
	if err != nil {
		t.Fatalf("CreateJob dedup error = %v", err)
	}
	if ins5 {
		t.Error("same user+external_id+source should be deduped (ins5=true, want false)")
	}
}
