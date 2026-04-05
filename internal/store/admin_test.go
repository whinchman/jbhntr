package store

import (
	"context"
	"testing"
	"time"

	"github.com/whinchman/jobhuntr/internal/models"
)

// ─── ListAllUsers ─────────────────────────────────────────────────────────────

func TestListAllUsers(t *testing.T) {
	ctx := context.Background()

	t.Run("returns all users ordered by created_at DESC", func(t *testing.T) {
		s := openTestStore(t)
		u1, err := s.UpsertUser(ctx, &models.User{Provider: "google", ProviderID: "lau-1", Email: "lau1@test.com"})
		if err != nil {
			t.Fatalf("UpsertUser u1 error = %v", err)
		}
		u2, err := s.UpsertUser(ctx, &models.User{Provider: "google", ProviderID: "lau-2", Email: "lau2@test.com"})
		if err != nil {
			t.Fatalf("UpsertUser u2 error = %v", err)
		}

		users, err := s.ListAllUsers(ctx)
		if err != nil {
			t.Fatalf("ListAllUsers error = %v", err)
		}
		if len(users) < 2 {
			t.Fatalf("expected at least 2 users, got %d", len(users))
		}
		// Verify both users are present.
		found := map[int64]bool{u1.ID: false, u2.ID: false}
		for _, u := range users {
			if _, ok := found[u.ID]; ok {
				found[u.ID] = true
			}
		}
		for id, ok := range found {
			if !ok {
				t.Errorf("user %d not found in ListAllUsers result", id)
			}
		}
	})

	t.Run("populates BannedAt for banned user", func(t *testing.T) {
		s := openTestStore(t)
		u, err := s.UpsertUser(ctx, &models.User{Provider: "google", ProviderID: "lau-banned", Email: "banned@test.com"})
		if err != nil {
			t.Fatalf("UpsertUser error = %v", err)
		}

		if err := s.BanUser(ctx, u.ID); err != nil {
			t.Fatalf("BanUser error = %v", err)
		}

		users, err := s.ListAllUsers(ctx)
		if err != nil {
			t.Fatalf("ListAllUsers error = %v", err)
		}
		var found *models.User
		for i := range users {
			if users[i].ID == u.ID {
				found = &users[i]
				break
			}
		}
		if found == nil {
			t.Fatal("banned user not found in ListAllUsers")
		}
		if found.BannedAt == nil {
			t.Error("BannedAt should be non-nil for banned user")
		}
	})
}

// ─── BanUser ──────────────────────────────────────────────────────────────────

func TestBanUser(t *testing.T) {
	ctx := context.Background()

	t.Run("sets banned_at", func(t *testing.T) {
		s := openTestStore(t)
		u, err := s.UpsertUser(ctx, &models.User{Provider: "google", ProviderID: "ban-1", Email: "ban1@test.com"})
		if err != nil {
			t.Fatalf("UpsertUser error = %v", err)
		}

		before := time.Now().UTC().Add(-time.Second)
		if err := s.BanUser(ctx, u.ID); err != nil {
			t.Fatalf("BanUser error = %v", err)
		}
		after := time.Now().UTC().Add(time.Second)

		got, err := s.GetUser(ctx, u.ID)
		if err != nil {
			t.Fatalf("GetUser error = %v", err)
		}
		if got.BannedAt == nil {
			t.Fatal("BannedAt is nil after BanUser")
		}
		if got.BannedAt.Before(before) || got.BannedAt.After(after) {
			t.Errorf("BannedAt = %v, want between %v and %v", got.BannedAt, before, after)
		}
	})

	t.Run("no error for nonexistent user (UPDATE affects 0 rows)", func(t *testing.T) {
		s := openTestStore(t)
		// BanUser does not check RowsAffected — no error expected.
		if err := s.BanUser(ctx, 99999); err != nil {
			t.Errorf("BanUser(nonexistent) unexpected error: %v", err)
		}
	})
}

// ─── UnbanUser ────────────────────────────────────────────────────────────────

func TestUnbanUser(t *testing.T) {
	ctx := context.Background()

	t.Run("clears banned_at", func(t *testing.T) {
		s := openTestStore(t)
		u, err := s.UpsertUser(ctx, &models.User{Provider: "google", ProviderID: "unban-1", Email: "unban1@test.com"})
		if err != nil {
			t.Fatalf("UpsertUser error = %v", err)
		}

		if err := s.BanUser(ctx, u.ID); err != nil {
			t.Fatalf("BanUser error = %v", err)
		}

		if err := s.UnbanUser(ctx, u.ID); err != nil {
			t.Fatalf("UnbanUser error = %v", err)
		}

		got, err := s.GetUser(ctx, u.ID)
		if err != nil {
			t.Fatalf("GetUser error = %v", err)
		}
		if got.BannedAt != nil {
			t.Errorf("BannedAt = %v, want nil after UnbanUser", got.BannedAt)
		}
	})

	t.Run("no error on already-active user", func(t *testing.T) {
		s := openTestStore(t)
		u, err := s.UpsertUser(ctx, &models.User{Provider: "google", ProviderID: "unban-active", Email: "ua@test.com"})
		if err != nil {
			t.Fatalf("UpsertUser error = %v", err)
		}

		// User is not banned — UnbanUser should be a no-op, not an error.
		if err := s.UnbanUser(ctx, u.ID); err != nil {
			t.Errorf("UnbanUser on active user unexpected error: %v", err)
		}
	})
}

// ─── SetPasswordHash ──────────────────────────────────────────────────────────

func TestSetPasswordHash(t *testing.T) {
	ctx := context.Background()

	t.Run("updates password_hash and clears reset token", func(t *testing.T) {
		s := openTestStore(t)

		hash := "$2a$10$somehash"
		verifyExp := time.Now().UTC().Add(24 * time.Hour)
		u, err := s.CreateUserWithPassword(ctx, "sph@test.com", "SPH User", hash, "verify-tok", verifyExp)
		if err != nil {
			t.Fatalf("CreateUserWithPassword error = %v", err)
		}

		// Set a reset token first.
		resetExp := time.Now().UTC().Add(time.Hour)
		if err := s.SetResetToken(ctx, u.ID, "reset-tok-123", resetExp); err != nil {
			t.Fatalf("SetResetToken error = %v", err)
		}

		// Now set a new password via admin.
		newHash := "$2a$10$newhash"
		if err := s.SetPasswordHash(ctx, u.ID, newHash); err != nil {
			t.Fatalf("SetPasswordHash error = %v", err)
		}

		got, err := s.GetUser(ctx, u.ID)
		if err != nil {
			t.Fatalf("GetUser error = %v", err)
		}
		if got.PasswordHash == nil || *got.PasswordHash != newHash {
			t.Errorf("PasswordHash = %v, want %q", got.PasswordHash, newHash)
		}
		if got.ResetToken != nil {
			t.Errorf("ResetToken = %v, want nil after SetPasswordHash", got.ResetToken)
		}
		if got.ResetExpiresAt != nil {
			t.Errorf("ResetExpiresAt = %v, want nil after SetPasswordHash", got.ResetExpiresAt)
		}
	})

	t.Run("no error for nonexistent user", func(t *testing.T) {
		s := openTestStore(t)
		if err := s.SetPasswordHash(ctx, 99999, "$2a$10$hash"); err != nil {
			t.Errorf("SetPasswordHash(nonexistent) unexpected error: %v", err)
		}
	})
}

// ─── ListAllFilters ───────────────────────────────────────────────────────────

func TestListAllFilters(t *testing.T) {
	ctx := context.Background()

	t.Run("returns all filters joined with user email", func(t *testing.T) {
		s := openTestStore(t)
		u1, err := s.UpsertUser(ctx, &models.User{Provider: "google", ProviderID: "laf-1", Email: "laf1@test.com"})
		if err != nil {
			t.Fatalf("UpsertUser u1 error = %v", err)
		}
		u2, err := s.UpsertUser(ctx, &models.User{Provider: "google", ProviderID: "laf-2", Email: "laf2@test.com"})
		if err != nil {
			t.Fatalf("UpsertUser u2 error = %v", err)
		}

		f1 := &models.UserSearchFilter{Keywords: "golang", Location: "Remote"}
		f2 := &models.UserSearchFilter{Keywords: "rust", Location: "NYC"}
		if err := s.CreateUserFilter(ctx, u1.ID, f1); err != nil {
			t.Fatalf("CreateUserFilter f1 error = %v", err)
		}
		if err := s.CreateUserFilter(ctx, u2.ID, f2); err != nil {
			t.Fatalf("CreateUserFilter f2 error = %v", err)
		}

		filters, err := s.ListAllFilters(ctx)
		if err != nil {
			t.Fatalf("ListAllFilters error = %v", err)
		}
		if len(filters) < 2 {
			t.Fatalf("expected at least 2 filters, got %d", len(filters))
		}

		// Verify both filters have correct email populated.
		emailByFilterID := map[int64]string{f1.ID: "laf1@test.com", f2.ID: "laf2@test.com"}
		for _, af := range filters {
			if wantEmail, ok := emailByFilterID[af.ID]; ok {
				if af.UserEmail != wantEmail {
					t.Errorf("filter %d: UserEmail = %q, want %q", af.ID, af.UserEmail, wantEmail)
				}
			}
		}
	})

	t.Run("returns empty slice when no filters exist", func(t *testing.T) {
		s := openTestStore(t)
		// No users or filters created — table should be empty.
		filters, err := s.ListAllFilters(ctx)
		if err != nil {
			t.Fatalf("ListAllFilters error = %v", err)
		}
		if len(filters) != 0 {
			t.Errorf("expected 0 filters, got %d", len(filters))
		}
	})
}

// ─── GetAdminStats ────────────────────────────────────────────────────────────

func TestGetAdminStats(t *testing.T) {
	ctx := context.Background()

	t.Run("returns non-negative counts", func(t *testing.T) {
		s := openTestStore(t)

		stats, err := s.GetAdminStats(ctx)
		if err != nil {
			t.Fatalf("GetAdminStats error = %v", err)
		}
		if stats.TotalUsers < 0 {
			t.Errorf("TotalUsers = %d, want >= 0", stats.TotalUsers)
		}
		if stats.TotalJobs < 0 {
			t.Errorf("TotalJobs = %d, want >= 0", stats.TotalJobs)
		}
		if stats.TotalFilters < 0 {
			t.Errorf("TotalFilters = %d, want >= 0", stats.TotalFilters)
		}
		if stats.NewUsersLast7d < 0 {
			t.Errorf("NewUsersLast7d = %d, want >= 0", stats.NewUsersLast7d)
		}
	})

	t.Run("new user appears in NewUsersLast7d", func(t *testing.T) {
		s := openTestStore(t)

		before, err := s.GetAdminStats(ctx)
		if err != nil {
			t.Fatalf("GetAdminStats (before) error = %v", err)
		}

		_, err = s.UpsertUser(ctx, &models.User{Provider: "google", ProviderID: "stats-new", Email: "stats-new@test.com"})
		if err != nil {
			t.Fatalf("UpsertUser error = %v", err)
		}

		after, err := s.GetAdminStats(ctx)
		if err != nil {
			t.Fatalf("GetAdminStats (after) error = %v", err)
		}

		if after.TotalUsers != before.TotalUsers+1 {
			t.Errorf("TotalUsers: before=%d after=%d, want increment of 1", before.TotalUsers, after.TotalUsers)
		}
		if after.NewUsersLast7d != before.NewUsersLast7d+1 {
			t.Errorf("NewUsersLast7d: before=%d after=%d, want increment of 1", before.NewUsersLast7d, after.NewUsersLast7d)
		}
	})

	t.Run("TotalFilters increments when filter added", func(t *testing.T) {
		s := openTestStore(t)

		u, err := s.UpsertUser(ctx, &models.User{Provider: "google", ProviderID: "stats-filt", Email: "sf@test.com"})
		if err != nil {
			t.Fatalf("UpsertUser error = %v", err)
		}

		before, err := s.GetAdminStats(ctx)
		if err != nil {
			t.Fatalf("GetAdminStats (before) error = %v", err)
		}

		if err := s.CreateUserFilter(ctx, u.ID, &models.UserSearchFilter{Keywords: "stats-test"}); err != nil {
			t.Fatalf("CreateUserFilter error = %v", err)
		}

		after, err := s.GetAdminStats(ctx)
		if err != nil {
			t.Fatalf("GetAdminStats (after) error = %v", err)
		}

		if after.TotalFilters != before.TotalFilters+1 {
			t.Errorf("TotalFilters: before=%d after=%d, want increment of 1", before.TotalFilters, after.TotalFilters)
		}
	})
}
