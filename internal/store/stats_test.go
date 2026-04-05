package store

import (
	"context"
	"testing"
	"time"

	"github.com/whinchman/jobhuntr/internal/models"
)

// createUserForStats is a helper that creates a unique test user and returns it.
func createUserForStats(t *testing.T, s *Store, providerID, email string) *models.User {
	t.Helper()
	u, err := s.UpsertUser(context.Background(), &models.User{
		Provider:   "google",
		ProviderID: providerID,
		Email:      email,
	})
	if err != nil {
		t.Fatalf("UpsertUser(%q) error = %v", providerID, err)
	}
	return u
}

// insertJobWithStatus inserts a job for userID with the given pipeline status.
func insertJobWithStatus(t *testing.T, s *Store, userID int64, ext string, status models.JobStatus) *models.Job {
	t.Helper()
	j := &models.Job{
		ExternalID: ext,
		Source:     "serpapi",
		Title:      "Test Job",
		Company:    "Acme",
		Location:   "Remote",
		Status:     status,
	}
	inserted, err := s.CreateJob(context.Background(), userID, j)
	if err != nil {
		t.Fatalf("CreateJob(%q) error = %v", ext, err)
	}
	if !inserted {
		t.Fatalf("CreateJob(%q) returned false (duplicate?)", ext)
	}
	return j
}

// setAppStatus sets the application_status for a job owned by userID.
func setAppStatus(t *testing.T, s *Store, userID, jobID int64, as models.ApplicationStatus) {
	t.Helper()
	if err := s.UpdateApplicationStatus(context.Background(), userID, jobID, as); err != nil {
		t.Fatalf("UpdateApplicationStatus(%d, %s) error = %v", jobID, as, err)
	}
}

// ─── GetUserJobStats ──────────────────────────────────────────────────────────

func TestGetUserJobStats_EmptyUser(t *testing.T) {
	s := openTestStore(t)
	u := createUserForStats(t, s, "stats-empty-1", "statsempty1@test.com")

	got, err := s.GetUserJobStats(context.Background(), u.ID)
	if err != nil {
		t.Fatalf("GetUserJobStats error = %v", err)
	}
	if got.TotalFound != 0 {
		t.Errorf("TotalFound = %d, want 0", got.TotalFound)
	}
	if got.TotalApproved != 0 {
		t.Errorf("TotalApproved = %d, want 0", got.TotalApproved)
	}
	if got.TotalRejected != 0 {
		t.Errorf("TotalRejected = %d, want 0", got.TotalRejected)
	}
	if got.TotalApplied != 0 {
		t.Errorf("TotalApplied = %d, want 0", got.TotalApplied)
	}
	if got.TotalInterviewing != 0 {
		t.Errorf("TotalInterviewing = %d, want 0", got.TotalInterviewing)
	}
	if got.TotalWon != 0 {
		t.Errorf("TotalWon = %d, want 0", got.TotalWon)
	}
	if got.TotalLost != 0 {
		t.Errorf("TotalLost = %d, want 0", got.TotalLost)
	}
}

func TestGetUserJobStats_MultiStatus(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	u := createUserForStats(t, s, "stats-multi-1", "statsmulti1@test.com")

	// Insert jobs at various pipeline statuses.
	insertJobWithStatus(t, s, u.ID, "ms-disc-1", models.StatusDiscovered)
	insertJobWithStatus(t, s, u.ID, "ms-disc-2", models.StatusDiscovered)

	rejected := insertJobWithStatus(t, s, u.ID, "ms-notified-1", models.StatusNotified)
	// transition notified → rejected
	if err := s.UpdateJobStatus(ctx, u.ID, rejected.ID, models.StatusRejected); err != nil {
		t.Fatalf("UpdateJobStatus rejected error = %v", err)
	}

	// approved job → set application_status = applied
	approved1 := insertJobWithStatus(t, s, u.ID, "ms-approved-1", models.StatusNotified)
	if err := s.UpdateJobStatus(ctx, u.ID, approved1.ID, models.StatusApproved); err != nil {
		t.Fatalf("UpdateJobStatus approved1 error = %v", err)
	}
	setAppStatus(t, s, u.ID, approved1.ID, models.AppStatusApplied)

	// approved job → set application_status = interviewing
	approved2 := insertJobWithStatus(t, s, u.ID, "ms-approved-2", models.StatusNotified)
	if err := s.UpdateJobStatus(ctx, u.ID, approved2.ID, models.StatusApproved); err != nil {
		t.Fatalf("UpdateJobStatus approved2 error = %v", err)
	}
	setAppStatus(t, s, u.ID, approved2.ID, models.AppStatusInterviewing)

	// approved job → set application_status = won
	approved3 := insertJobWithStatus(t, s, u.ID, "ms-approved-3", models.StatusNotified)
	if err := s.UpdateJobStatus(ctx, u.ID, approved3.ID, models.StatusApproved); err != nil {
		t.Fatalf("UpdateJobStatus approved3 error = %v", err)
	}
	setAppStatus(t, s, u.ID, approved3.ID, models.AppStatusWon)

	// approved job → set application_status = lost
	approved4 := insertJobWithStatus(t, s, u.ID, "ms-approved-4", models.StatusNotified)
	if err := s.UpdateJobStatus(ctx, u.ID, approved4.ID, models.StatusApproved); err != nil {
		t.Fatalf("UpdateJobStatus approved4 error = %v", err)
	}
	setAppStatus(t, s, u.ID, approved4.ID, models.AppStatusLost)

	got, err := s.GetUserJobStats(ctx, u.ID)
	if err != nil {
		t.Fatalf("GetUserJobStats error = %v", err)
	}

	// Total = 2 discovered + 1 rejected + 4 approved = 7
	if got.TotalFound != 7 {
		t.Errorf("TotalFound = %d, want 7", got.TotalFound)
	}
	// approved count = 4 (all 4 approved jobs, status='approved')
	if got.TotalApproved != 4 {
		t.Errorf("TotalApproved = %d, want 4", got.TotalApproved)
	}
	// rejected = 1
	if got.TotalRejected != 1 {
		t.Errorf("TotalRejected = %d, want 1", got.TotalRejected)
	}
	// applied = 1
	if got.TotalApplied != 1 {
		t.Errorf("TotalApplied = %d, want 1", got.TotalApplied)
	}
	// interviewing = 1
	if got.TotalInterviewing != 1 {
		t.Errorf("TotalInterviewing = %d, want 1", got.TotalInterviewing)
	}
	// won = 1
	if got.TotalWon != 1 {
		t.Errorf("TotalWon = %d, want 1", got.TotalWon)
	}
	// lost = 1
	if got.TotalLost != 1 {
		t.Errorf("TotalLost = %d, want 1", got.TotalLost)
	}
}

func TestGetUserJobStats_UserScoping(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	u1 := createUserForStats(t, s, "stats-scope-u1", "statsscope1@test.com")
	u2 := createUserForStats(t, s, "stats-scope-u2", "statsscope2@test.com")

	// u1 gets 3 jobs
	insertJobWithStatus(t, s, u1.ID, "scope-u1-a", models.StatusDiscovered)
	insertJobWithStatus(t, s, u1.ID, "scope-u1-b", models.StatusDiscovered)
	insertJobWithStatus(t, s, u1.ID, "scope-u1-c", models.StatusDiscovered)

	// u2 gets 1 job
	insertJobWithStatus(t, s, u2.ID, "scope-u2-a", models.StatusDiscovered)

	st1, err := s.GetUserJobStats(ctx, u1.ID)
	if err != nil {
		t.Fatalf("GetUserJobStats u1 error = %v", err)
	}
	if st1.TotalFound != 3 {
		t.Errorf("u1 TotalFound = %d, want 3", st1.TotalFound)
	}

	st2, err := s.GetUserJobStats(ctx, u2.ID)
	if err != nil {
		t.Fatalf("GetUserJobStats u2 error = %v", err)
	}
	if st2.TotalFound != 1 {
		t.Errorf("u2 TotalFound = %d, want 1", st2.TotalFound)
	}
}

// ─── GetJobsPerWeek ───────────────────────────────────────────────────────────

func TestGetJobsPerWeek_EmptyResult(t *testing.T) {
	s := openTestStore(t)
	u := createUserForStats(t, s, "weekly-empty-1", "weeklyempty1@test.com")

	got, err := s.GetJobsPerWeek(context.Background(), u.ID, 4)
	if err != nil {
		t.Fatalf("GetJobsPerWeek error = %v", err)
	}
	if len(got) != 0 {
		t.Errorf("len = %d, want 0", len(got))
	}
}

func TestGetJobsPerWeek_WeeklyCounts(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	u := createUserForStats(t, s, "weekly-counts-1", "weeklycounts1@test.com")

	// Insert two jobs with NOW() timestamps (default).  They should both fall in
	// the current week and appear as a single WeeklyJobCount with Count=2.
	insertJobWithStatus(t, s, u.ID, "wk-a", models.StatusDiscovered)
	insertJobWithStatus(t, s, u.ID, "wk-b", models.StatusDiscovered)

	got, err := s.GetJobsPerWeek(ctx, u.ID, 4)
	if err != nil {
		t.Fatalf("GetJobsPerWeek error = %v", err)
	}
	if len(got) == 0 {
		t.Fatal("expected at least one week bucket, got 0")
	}

	// The total across all returned buckets must equal 2.
	total := 0
	for _, wc := range got {
		total += wc.Count
		// WeekStart should be a Monday (ISO week start in Postgres date_trunc).
		if wc.WeekStart.IsZero() {
			t.Error("WeekStart is zero")
		}
	}
	if total != 2 {
		t.Errorf("total across weeks = %d, want 2", total)
	}
}

func TestGetJobsPerWeek_OldJobsExcluded(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	u := createUserForStats(t, s, "weekly-old-1", "weeklyold1@test.com")

	// Insert a job in the normal way (discovered_at = NOW()).
	insertJobWithStatus(t, s, u.ID, "wk-recent", models.StatusDiscovered)

	// Use a raw INSERT to backdate a job well outside the 1-week window.
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO jobs (user_id, external_id, source, title, company, location, status, discovered_at, updated_at)
		VALUES ($1, 'wk-old', 'serpapi', 'Old Job', 'Acme', 'Remote', 'discovered', NOW() - INTERVAL '10 weeks', NOW())`,
		u.ID,
	)
	if err != nil {
		t.Fatalf("insert backdated job error = %v", err)
	}

	got, err := s.GetJobsPerWeek(ctx, u.ID, 1)
	if err != nil {
		t.Fatalf("GetJobsPerWeek error = %v", err)
	}

	total := 0
	for _, wc := range got {
		total += wc.Count
	}

	// Only the recent job should appear; the 10-week-old job is excluded.
	if total != 1 {
		t.Errorf("total = %d, want 1 (old job should be excluded)", total)
	}
}

func TestGetJobsPerWeek_WeekStartIsTime(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	u := createUserForStats(t, s, "weekly-ts-1", "weeklyts1@test.com")

	insertJobWithStatus(t, s, u.ID, "wk-ts", models.StatusDiscovered)

	got, err := s.GetJobsPerWeek(ctx, u.ID, 4)
	if err != nil {
		t.Fatalf("GetJobsPerWeek error = %v", err)
	}
	if len(got) == 0 {
		t.Fatal("expected at least one bucket")
	}

	ws := got[len(got)-1].WeekStart
	// WeekStart should not be zero and should be in the past (relative to now).
	if ws.IsZero() {
		t.Error("WeekStart is zero")
	}
	if ws.After(time.Now()) {
		t.Errorf("WeekStart %v is in the future", ws)
	}
}

// ─── BuildSankeyLinks ─────────────────────────────────────────────────────────

// TestBuildSankeyLinks_ZeroStats verifies that a fully-zero UserJobStats
// produces an empty slice (no zero-value edges emitted).
func TestBuildSankeyLinks_ZeroStats(t *testing.T) {
	links := BuildSankeyLinks(UserJobStats{})
	if len(links) != 0 {
		t.Errorf("expected empty slice for zero stats, got %d links: %v", len(links), links)
	}
}

// TestBuildSankeyLinks_FullFunnel verifies that known non-zero counts produce
// the correct set of edges with the correct values.
func TestBuildSankeyLinks_FullFunnel(t *testing.T) {
	stats := UserJobStats{
		TotalFound:        10,
		TotalApproved:     6,
		TotalRejected:     2,
		TotalApplied:      4,
		TotalInterviewing: 2,
		TotalWon:          1,
		TotalLost:         3,
	}
	// pending = 10 - 6 - 2 = 2
	links := BuildSankeyLinks(stats)

	type edge struct{ src, tgt string }
	want := map[edge]int{
		{src: "Discovered", tgt: "Approved"}:     6,
		{src: "Discovered", tgt: "Rejected"}:     2,
		{src: "Discovered", tgt: "Pending"}:      2,
		{src: "Approved", tgt: "Applied"}:        4,
		{src: "Approved", tgt: "Interviewing"}:   2,
		{src: "Applied", tgt: "Won"}:             1,
		{src: "Applied", tgt: "Lost"}:            3,
	}

	if len(links) != len(want) {
		t.Fatalf("got %d links, want %d; links = %v", len(links), len(want), links)
	}
	for _, l := range links {
		k := edge{src: l.Source, tgt: l.Target}
		wantVal, ok := want[k]
		if !ok {
			t.Errorf("unexpected edge %q → %q", l.Source, l.Target)
			continue
		}
		if l.Value != wantVal {
			t.Errorf("edge %q → %q: value = %d, want %d", l.Source, l.Target, l.Value, wantVal)
		}
	}
}

// TestBuildSankeyLinks_PendingCalc verifies that the Pending node value equals
// TotalFound - TotalApproved - TotalRejected.
func TestBuildSankeyLinks_PendingCalc(t *testing.T) {
	stats := UserJobStats{
		TotalFound:    7,
		TotalApproved: 3,
		TotalRejected: 1,
		// pending = 7 - 3 - 1 = 3
	}
	links := BuildSankeyLinks(stats)

	for _, l := range links {
		if l.Source == "Discovered" && l.Target == "Pending" {
			if l.Value != 3 {
				t.Errorf("Pending value = %d, want 3", l.Value)
			}
			return
		}
	}
	t.Error("expected a Discovered → Pending link but found none")
}

// TestBuildSankeyLinks_NegativePendingClamped verifies that when pending would
// be negative it is clamped to zero and no Pending edge is emitted.
func TestBuildSankeyLinks_NegativePendingClamped(t *testing.T) {
	stats := UserJobStats{
		TotalFound:    5,
		TotalApproved: 4,
		TotalRejected: 3,
		// pending = 5 - 4 - 3 = -2 → clamped to 0 → no edge
	}
	links := BuildSankeyLinks(stats)

	for _, l := range links {
		if l.Target == "Pending" {
			t.Errorf("expected no Pending link but got %q → %q = %d", l.Source, l.Target, l.Value)
		}
	}
}
