package store

import (
	"context"
	"errors"
	"testing"

	"github.com/whinchman/jobhuntr/internal/models"
)

// ─── CreateUserBannedTerm ─────────────────────────────────────────────────────

func TestCreateUserBannedTerm(t *testing.T) {
	ctx := context.Background()

	t.Run("inserts term and returns populated struct", func(t *testing.T) {
		s := openTestStore(t)
		user, _ := s.UpsertUser(ctx, &models.User{Provider: "google", ProviderID: "bt-create-1", Email: "bt1@test.com"})

		bt, err := s.CreateUserBannedTerm(ctx, user.ID, "Google")
		if err != nil {
			t.Fatalf("CreateUserBannedTerm error = %v", err)
		}
		if bt.ID <= 0 {
			t.Errorf("bt.ID = %d, want > 0", bt.ID)
		}
		if bt.UserID != user.ID {
			t.Errorf("bt.UserID = %d, want %d", bt.UserID, user.ID)
		}
		if bt.Term != "Google" {
			t.Errorf("bt.Term = %q, want Google", bt.Term)
		}
		if bt.CreatedAt.IsZero() {
			t.Error("bt.CreatedAt is zero")
		}
	})

	t.Run("returns ErrDuplicateBannedTerm on duplicate", func(t *testing.T) {
		s := openTestStore(t)
		user, _ := s.UpsertUser(ctx, &models.User{Provider: "google", ProviderID: "bt-dup", Email: "btdup@test.com"})

		_, err := s.CreateUserBannedTerm(ctx, user.ID, "Amazon")
		if err != nil {
			t.Fatalf("first CreateUserBannedTerm error = %v", err)
		}

		_, err = s.CreateUserBannedTerm(ctx, user.ID, "Amazon")
		if !errors.Is(err, ErrDuplicateBannedTerm) {
			t.Errorf("duplicate insert error = %v, want ErrDuplicateBannedTerm", err)
		}
	})

	t.Run("same term for different users is allowed", func(t *testing.T) {
		s := openTestStore(t)
		u1, _ := s.UpsertUser(ctx, &models.User{Provider: "google", ProviderID: "bt-multi-1", Email: "btm1@test.com"})
		u2, _ := s.UpsertUser(ctx, &models.User{Provider: "google", ProviderID: "bt-multi-2", Email: "btm2@test.com"})

		_, err := s.CreateUserBannedTerm(ctx, u1.ID, "Facebook")
		if err != nil {
			t.Fatalf("u1 CreateUserBannedTerm error = %v", err)
		}
		_, err = s.CreateUserBannedTerm(ctx, u2.ID, "Facebook")
		if err != nil {
			t.Fatalf("u2 CreateUserBannedTerm error = %v", err)
		}
	})
}

// ─── ListUserBannedTerms ──────────────────────────────────────────────────────

func TestListUserBannedTerms(t *testing.T) {
	ctx := context.Background()

	t.Run("returns terms ordered by created_at DESC", func(t *testing.T) {
		s := openTestStore(t)
		user, _ := s.UpsertUser(ctx, &models.User{Provider: "google", ProviderID: "bt-list-1", Email: "btl1@test.com"})

		s.CreateUserBannedTerm(ctx, user.ID, "Alpha")
		s.CreateUserBannedTerm(ctx, user.ID, "Beta")
		s.CreateUserBannedTerm(ctx, user.ID, "Gamma")

		terms, err := s.ListUserBannedTerms(ctx, user.ID)
		if err != nil {
			t.Fatalf("ListUserBannedTerms error = %v", err)
		}
		if len(terms) != 3 {
			t.Fatalf("len = %d, want 3", len(terms))
		}
		// Most recently inserted should come first.
		if terms[0].Term != "Gamma" {
			t.Errorf("terms[0].Term = %q, want Gamma (most recent)", terms[0].Term)
		}
	})

	t.Run("returns empty slice for user with no terms", func(t *testing.T) {
		s := openTestStore(t)
		user, _ := s.UpsertUser(ctx, &models.User{Provider: "google", ProviderID: "bt-list-empty", Email: "btle@test.com"})

		terms, err := s.ListUserBannedTerms(ctx, user.ID)
		if err != nil {
			t.Fatalf("ListUserBannedTerms error = %v", err)
		}
		if len(terms) != 0 {
			t.Errorf("expected empty slice, got %d terms", len(terms))
		}
	})

	t.Run("does not return other users terms", func(t *testing.T) {
		s := openTestStore(t)
		u1, _ := s.UpsertUser(ctx, &models.User{Provider: "google", ProviderID: "bt-iso-1", Email: "bti1@test.com"})
		u2, _ := s.UpsertUser(ctx, &models.User{Provider: "google", ProviderID: "bt-iso-2", Email: "bti2@test.com"})

		s.CreateUserBannedTerm(ctx, u1.ID, "u1-only")
		s.CreateUserBannedTerm(ctx, u2.ID, "u2-only")

		terms, err := s.ListUserBannedTerms(ctx, u1.ID)
		if err != nil {
			t.Fatalf("ListUserBannedTerms error = %v", err)
		}
		if len(terms) != 1 {
			t.Fatalf("len = %d, want 1", len(terms))
		}
		if terms[0].Term != "u1-only" {
			t.Errorf("terms[0].Term = %q, want u1-only", terms[0].Term)
		}
	})
}

// ─── DeleteUserBannedTerm ─────────────────────────────────────────────────────

func TestDeleteUserBannedTerm(t *testing.T) {
	ctx := context.Background()

	t.Run("deletes existing term", func(t *testing.T) {
		s := openTestStore(t)
		user, _ := s.UpsertUser(ctx, &models.User{Provider: "google", ProviderID: "bt-del-1", Email: "btd1@test.com"})

		bt, _ := s.CreateUserBannedTerm(ctx, user.ID, "ToDelete")

		err := s.DeleteUserBannedTerm(ctx, user.ID, bt.ID)
		if err != nil {
			t.Fatalf("DeleteUserBannedTerm error = %v", err)
		}

		terms, _ := s.ListUserBannedTerms(ctx, user.ID)
		if len(terms) != 0 {
			t.Errorf("expected 0 terms after delete, got %d", len(terms))
		}
	})

	t.Run("returns error for non-existent term ID", func(t *testing.T) {
		s := openTestStore(t)
		user, _ := s.UpsertUser(ctx, &models.User{Provider: "google", ProviderID: "bt-del-nope", Email: "btdn@test.com"})

		err := s.DeleteUserBannedTerm(ctx, user.ID, 99999)
		if err == nil {
			t.Error("DeleteUserBannedTerm(nonexistent) expected error, got nil")
		}
	})

	t.Run("returns error when term belongs to different user", func(t *testing.T) {
		s := openTestStore(t)
		u1, _ := s.UpsertUser(ctx, &models.User{Provider: "google", ProviderID: "bt-del-a", Email: "btda@test.com"})
		u2, _ := s.UpsertUser(ctx, &models.User{Provider: "google", ProviderID: "bt-del-b", Email: "btdb@test.com"})

		bt, _ := s.CreateUserBannedTerm(ctx, u1.ID, "OwnedByU1")

		err := s.DeleteUserBannedTerm(ctx, u2.ID, bt.ID)
		if err == nil {
			t.Error("DeleteUserBannedTerm(wrong user) expected error, got nil")
		}

		// Term should still exist for u1.
		terms, _ := s.ListUserBannedTerms(ctx, u1.ID)
		if len(terms) != 1 {
			t.Errorf("expected term to still exist for u1, got %d terms", len(terms))
		}
	})
}

// ─── ListJobs with BannedTerms ────────────────────────────────────────────────

func TestListJobs_BannedTerms(t *testing.T) {
	ctx := context.Background()

	setup := func(t *testing.T) *Store {
		s := openTestStore(t)
		jobs := []*models.Job{
			{ExternalID: "bt-j1", Source: "serpapi", Title: "Go Engineer", Company: "Acme Corp", Description: "Build cool stuff in Go.", Status: models.StatusDiscovered},
			{ExternalID: "bt-j2", Source: "serpapi", Title: "Python Dev", Company: "Google LLC", Description: "Python ML work.", Status: models.StatusDiscovered},
			{ExternalID: "bt-j3", Source: "serpapi", Title: "Staff Engineer", Company: "Meta Platforms", Description: "Social graph infra.", Status: models.StatusDiscovered},
			{ExternalID: "bt-j4", Source: "serpapi", Title: "Amazon SDE", Company: "Amazon", Description: "AWS backend work.", Status: models.StatusDiscovered},
		}
		for _, j := range jobs {
			s.CreateJob(ctx, 0, j)
		}
		return s
	}

	t.Run("no banned terms returns all jobs", func(t *testing.T) {
		s := setup(t)
		jobs, err := s.ListJobs(ctx, 0, ListJobsFilter{BannedTerms: nil})
		if err != nil {
			t.Fatalf("ListJobs error = %v", err)
		}
		if len(jobs) < 4 {
			t.Errorf("expected >= 4 jobs, got %d", len(jobs))
		}
	})

	t.Run("single banned term filters by title", func(t *testing.T) {
		s := setup(t)
		jobs, err := s.ListJobs(ctx, 0, ListJobsFilter{BannedTerms: []string{"Amazon"}})
		if err != nil {
			t.Fatalf("ListJobs error = %v", err)
		}
		for _, j := range jobs {
			if j.ExternalID == "bt-j4" {
				t.Errorf("job with banned term 'Amazon' in title still returned")
			}
		}
	})

	t.Run("banned term filters by company", func(t *testing.T) {
		s := setup(t)
		jobs, err := s.ListJobs(ctx, 0, ListJobsFilter{BannedTerms: []string{"Google"}})
		if err != nil {
			t.Fatalf("ListJobs error = %v", err)
		}
		for _, j := range jobs {
			if j.ExternalID == "bt-j2" {
				t.Errorf("job with banned term 'Google' in company still returned")
			}
		}
	})

	t.Run("banned term filters by description", func(t *testing.T) {
		s := setup(t)
		jobs, err := s.ListJobs(ctx, 0, ListJobsFilter{BannedTerms: []string{"Social graph"}})
		if err != nil {
			t.Fatalf("ListJobs error = %v", err)
		}
		for _, j := range jobs {
			if j.ExternalID == "bt-j3" {
				t.Errorf("job with banned term in description still returned")
			}
		}
	})

	t.Run("multiple banned terms filter independently", func(t *testing.T) {
		s := setup(t)
		jobs, err := s.ListJobs(ctx, 0, ListJobsFilter{BannedTerms: []string{"Google", "Meta"}})
		if err != nil {
			t.Fatalf("ListJobs error = %v", err)
		}
		for _, j := range jobs {
			if j.ExternalID == "bt-j2" {
				t.Errorf("Google job still returned after multi banned term filter")
			}
			if j.ExternalID == "bt-j3" {
				t.Errorf("Meta job still returned after multi banned term filter")
			}
		}
	})

	t.Run("banned term matching is case-insensitive", func(t *testing.T) {
		s := setup(t)
		// Use lowercase "google" to match "Google LLC" company.
		jobs, err := s.ListJobs(ctx, 0, ListJobsFilter{BannedTerms: []string{"google"}})
		if err != nil {
			t.Fatalf("ListJobs error = %v", err)
		}
		for _, j := range jobs {
			if j.ExternalID == "bt-j2" {
				t.Errorf("case-insensitive match missed: 'google' should filter 'Google LLC'")
			}
		}
	})
}
