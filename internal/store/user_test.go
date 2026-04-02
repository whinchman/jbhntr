package store

import (
	"context"
	"testing"
	"time"

	"github.com/whinchman/jobhuntr/internal/models"
)

// ─── ListActiveUserIDs ──────────────────────────────────────────────────────

func TestListActiveUserIDs(t *testing.T) {
	ctx := context.Background()

	t.Run("returns distinct user IDs with filters", func(t *testing.T) {
		s := openTestStore(t)
		u1, _ := s.UpsertUser(ctx, &models.User{Provider: "google", ProviderID: "active-1", Email: "a1@test.com"})
		u2, _ := s.UpsertUser(ctx, &models.User{Provider: "google", ProviderID: "active-2", Email: "a2@test.com"})

		s.CreateUserFilter(ctx, u1.ID, &models.UserSearchFilter{Keywords: "golang"})
		s.CreateUserFilter(ctx, u1.ID, &models.UserSearchFilter{Keywords: "rust"})
		s.CreateUserFilter(ctx, u2.ID, &models.UserSearchFilter{Keywords: "python"})

		ids, err := s.ListActiveUserIDs(ctx)
		if err != nil {
			t.Fatalf("ListActiveUserIDs error = %v", err)
		}
		if len(ids) != 2 {
			t.Fatalf("len = %d, want 2", len(ids))
		}
		// IDs should be in ascending order.
		if ids[0] != u1.ID || ids[1] != u2.ID {
			t.Errorf("ids = %v, want [%d, %d]", ids, u1.ID, u2.ID)
		}
	})

	t.Run("returns empty when no filters exist", func(t *testing.T) {
		s := openTestStore(t)
		// Create users but no filters.
		s.UpsertUser(ctx, &models.User{Provider: "google", ProviderID: "nofilt-1", Email: "nf1@test.com"})

		ids, err := s.ListActiveUserIDs(ctx)
		if err != nil {
			t.Fatalf("ListActiveUserIDs error = %v", err)
		}
		if len(ids) != 0 {
			t.Errorf("len = %d, want 0", len(ids))
		}
	})

	t.Run("does not return users with no filters", func(t *testing.T) {
		s := openTestStore(t)
		u1, _ := s.UpsertUser(ctx, &models.User{Provider: "google", ProviderID: "sel-1", Email: "s1@test.com"})
		s.UpsertUser(ctx, &models.User{Provider: "google", ProviderID: "sel-2", Email: "s2@test.com"}) // no filters
		u3, _ := s.UpsertUser(ctx, &models.User{Provider: "google", ProviderID: "sel-3", Email: "s3@test.com"})

		s.CreateUserFilter(ctx, u1.ID, &models.UserSearchFilter{Keywords: "go"})
		s.CreateUserFilter(ctx, u3.ID, &models.UserSearchFilter{Keywords: "java"})

		ids, err := s.ListActiveUserIDs(ctx)
		if err != nil {
			t.Fatalf("ListActiveUserIDs error = %v", err)
		}
		if len(ids) != 2 {
			t.Fatalf("len = %d, want 2", len(ids))
		}
		if ids[0] != u1.ID || ids[1] != u3.ID {
			t.Errorf("ids = %v, want [%d, %d]", ids, u1.ID, u3.ID)
		}
	})
}

// ─── UpsertUser ──────────────────────────────────────────────────────────────

func TestUpsertUser(t *testing.T) {
	ctx := context.Background()

	t.Run("inserts new user", func(t *testing.T) {
		s := openTestStore(t)
		user := &models.User{
			Provider:       "google",
			ProviderID:     "goog-1",
			Email:          "alice@example.com",
			DisplayName:    "Alice",
			AvatarURL:      "https://example.com/avatar.png",
			ResumeMarkdown: "# My Resume",
		}
		got, err := s.UpsertUser(ctx, user)
		if err != nil {
			t.Fatalf("UpsertUser error = %v", err)
		}
		if got.ID == 0 {
			t.Error("ID not set after upsert")
		}
		if got.Email != "alice@example.com" {
			t.Errorf("Email = %q, want alice@example.com", got.Email)
		}
		if got.CreatedAt.IsZero() {
			t.Error("CreatedAt is zero")
		}
	})

	t.Run("updates last_login_at on conflict", func(t *testing.T) {
		s := openTestStore(t)
		user := &models.User{
			Provider:   "github",
			ProviderID: "gh-1",
			Email:      "bob@example.com",
		}
		first, err := s.UpsertUser(ctx, user)
		if err != nil {
			t.Fatalf("first UpsertUser error = %v", err)
		}

		user.Email = "bob-updated@example.com"
		second, err := s.UpsertUser(ctx, user)
		if err != nil {
			t.Fatalf("second UpsertUser error = %v", err)
		}
		if second.ID != first.ID {
			t.Errorf("ID changed: first=%d, second=%d", first.ID, second.ID)
		}
		if second.Email != "bob-updated@example.com" {
			t.Errorf("Email not updated: %q", second.Email)
		}
	})

	t.Run("does not overwrite resume_markdown on conflict", func(t *testing.T) {
		s := openTestStore(t)
		user := &models.User{
			Provider:       "google",
			ProviderID:     "goog-resume",
			Email:          "carol@example.com",
			ResumeMarkdown: "# Original Resume",
		}
		_, err := s.UpsertUser(ctx, user)
		if err != nil {
			t.Fatalf("first UpsertUser error = %v", err)
		}

		user.ResumeMarkdown = "# Overwritten Resume"
		got, err := s.UpsertUser(ctx, user)
		if err != nil {
			t.Fatalf("second UpsertUser error = %v", err)
		}
		if got.ResumeMarkdown != "# Original Resume" {
			t.Errorf("ResumeMarkdown was overwritten: %q", got.ResumeMarkdown)
		}
	})

	t.Run("returns populated ID", func(t *testing.T) {
		s := openTestStore(t)
		user := &models.User{
			Provider:   "google",
			ProviderID: "goog-id-test",
			Email:      "id@example.com",
		}
		got, err := s.UpsertUser(ctx, user)
		if err != nil {
			t.Fatalf("UpsertUser error = %v", err)
		}
		if got.ID <= 0 {
			t.Errorf("ID = %d, want > 0", got.ID)
		}
	})
}

// ─── GetUser ─────────────────────────────────────────────────────────────────

func TestGetUser(t *testing.T) {
	ctx := context.Background()

	t.Run("returns inserted user", func(t *testing.T) {
		s := openTestStore(t)
		user := &models.User{
			Provider:    "google",
			ProviderID:  "goog-get",
			Email:       "get@example.com",
			DisplayName: "Get User",
		}
		created, _ := s.UpsertUser(ctx, user)

		got, err := s.GetUser(ctx, created.ID)
		if err != nil {
			t.Fatalf("GetUser error = %v", err)
		}
		if got.Email != "get@example.com" {
			t.Errorf("Email = %q, want get@example.com", got.Email)
		}
		if got.DisplayName != "Get User" {
			t.Errorf("DisplayName = %q, want Get User", got.DisplayName)
		}
	})

	t.Run("returns error for unknown ID", func(t *testing.T) {
		s := openTestStore(t)
		_, err := s.GetUser(ctx, 99999)
		if err == nil {
			t.Error("GetUser(nonexistent) expected error, got nil")
		}
	})
}

// ─── GetUserByProvider ──────────────────────────────────────────────────────

func TestGetUserByProvider(t *testing.T) {
	ctx := context.Background()

	t.Run("returns user by provider and providerID", func(t *testing.T) {
		s := openTestStore(t)
		user := &models.User{
			Provider:   "github",
			ProviderID: "gh-provider-test",
			Email:      "provider@example.com",
		}
		s.UpsertUser(ctx, user)

		got, err := s.GetUserByProvider(ctx, "github", "gh-provider-test")
		if err != nil {
			t.Fatalf("GetUserByProvider error = %v", err)
		}
		if got.Email != "provider@example.com" {
			t.Errorf("Email = %q, want provider@example.com", got.Email)
		}
	})

	t.Run("returns error for unknown provider combo", func(t *testing.T) {
		s := openTestStore(t)
		_, err := s.GetUserByProvider(ctx, "unknown", "none")
		if err == nil {
			t.Error("GetUserByProvider(unknown) expected error, got nil")
		}
	})
}

// ─── CreateUserFilter ───────────────────────────────────────────────────────

func TestCreateUserFilter(t *testing.T) {
	ctx := context.Background()

	t.Run("inserts filter with correct user_id", func(t *testing.T) {
		s := openTestStore(t)
		user, _ := s.UpsertUser(ctx, &models.User{Provider: "google", ProviderID: "filter-user", Email: "f@test.com"})

		filter := &models.UserSearchFilter{
			Keywords:  "golang",
			Location:  "Remote",
			MinSalary: 100000,
			MaxSalary: 200000,
			Title:     "Senior Engineer",
		}
		err := s.CreateUserFilter(ctx, user.ID, filter)
		if err != nil {
			t.Fatalf("CreateUserFilter error = %v", err)
		}
		if filter.UserID != user.ID {
			t.Errorf("UserID = %d, want %d", filter.UserID, user.ID)
		}
	})

	t.Run("sets ID on returned filter", func(t *testing.T) {
		s := openTestStore(t)
		user, _ := s.UpsertUser(ctx, &models.User{Provider: "google", ProviderID: "filter-id", Email: "fid@test.com"})

		filter := &models.UserSearchFilter{Keywords: "rust"}
		err := s.CreateUserFilter(ctx, user.ID, filter)
		if err != nil {
			t.Fatalf("CreateUserFilter error = %v", err)
		}
		if filter.ID <= 0 {
			t.Errorf("filter.ID = %d, want > 0", filter.ID)
		}
	})
}

// ─── ListUserFilters ────────────────────────────────────────────────────────

func TestListUserFilters(t *testing.T) {
	ctx := context.Background()

	t.Run("returns filters for user", func(t *testing.T) {
		s := openTestStore(t)
		user, _ := s.UpsertUser(ctx, &models.User{Provider: "google", ProviderID: "list-user", Email: "l@test.com"})

		f1 := &models.UserSearchFilter{Keywords: "first"}
		f2 := &models.UserSearchFilter{Keywords: "second"}
		s.CreateUserFilter(ctx, user.ID, f1)
		s.CreateUserFilter(ctx, user.ID, f2)

		filters, err := s.ListUserFilters(ctx, user.ID)
		if err != nil {
			t.Fatalf("ListUserFilters error = %v", err)
		}
		if len(filters) != 2 {
			t.Fatalf("len = %d, want 2", len(filters))
		}
		// Both filters should be present.
		keywords := map[string]bool{filters[0].Keywords: true, filters[1].Keywords: true}
		if !keywords["first"] || !keywords["second"] {
			t.Errorf("expected both filters, got %v", keywords)
		}
	})

	t.Run("returns empty slice for user with no filters", func(t *testing.T) {
		s := openTestStore(t)
		user, _ := s.UpsertUser(ctx, &models.User{Provider: "google", ProviderID: "no-filters", Email: "nf@test.com"})

		filters, err := s.ListUserFilters(ctx, user.ID)
		if err != nil {
			t.Fatalf("ListUserFilters error = %v", err)
		}
		if filters != nil && len(filters) != 0 {
			t.Errorf("expected empty slice, got %d filters", len(filters))
		}
	})

	t.Run("does not return other users filters", func(t *testing.T) {
		s := openTestStore(t)
		u1, _ := s.UpsertUser(ctx, &models.User{Provider: "google", ProviderID: "iso-a", Email: "a@test.com"})
		u2, _ := s.UpsertUser(ctx, &models.User{Provider: "google", ProviderID: "iso-b", Email: "b@test.com"})

		s.CreateUserFilter(ctx, u1.ID, &models.UserSearchFilter{Keywords: "user1-filter"})
		s.CreateUserFilter(ctx, u2.ID, &models.UserSearchFilter{Keywords: "user2-filter"})

		filters, err := s.ListUserFilters(ctx, u1.ID)
		if err != nil {
			t.Fatalf("ListUserFilters error = %v", err)
		}
		if len(filters) != 1 {
			t.Fatalf("len = %d, want 1", len(filters))
		}
		if filters[0].Keywords != "user1-filter" {
			t.Errorf("Keywords = %q, want user1-filter", filters[0].Keywords)
		}
	})
}

// ─── DeleteUserFilter ───────────────────────────────────────────────────────

func TestDeleteUserFilter(t *testing.T) {
	ctx := context.Background()

	t.Run("deletes existing filter", func(t *testing.T) {
		s := openTestStore(t)
		user, _ := s.UpsertUser(ctx, &models.User{Provider: "google", ProviderID: "del-user", Email: "d@test.com"})
		filter := &models.UserSearchFilter{Keywords: "to-delete"}
		s.CreateUserFilter(ctx, user.ID, filter)

		err := s.DeleteUserFilter(ctx, user.ID, filter.ID)
		if err != nil {
			t.Fatalf("DeleteUserFilter error = %v", err)
		}

		filters, _ := s.ListUserFilters(ctx, user.ID)
		if len(filters) != 0 {
			t.Errorf("expected 0 filters after delete, got %d", len(filters))
		}
	})

	t.Run("returns error for non-existent filter", func(t *testing.T) {
		s := openTestStore(t)
		user, _ := s.UpsertUser(ctx, &models.User{Provider: "google", ProviderID: "del-nope", Email: "dn@test.com"})

		err := s.DeleteUserFilter(ctx, user.ID, 99999)
		if err == nil {
			t.Error("DeleteUserFilter(nonexistent) expected error, got nil")
		}
	})

	t.Run("returns error when filter belongs to different user", func(t *testing.T) {
		s := openTestStore(t)
		u1, _ := s.UpsertUser(ctx, &models.User{Provider: "google", ProviderID: "del-a", Email: "da@test.com"})
		u2, _ := s.UpsertUser(ctx, &models.User{Provider: "google", ProviderID: "del-b", Email: "db@test.com"})

		filter := &models.UserSearchFilter{Keywords: "owned-by-u1"}
		s.CreateUserFilter(ctx, u1.ID, filter)

		err := s.DeleteUserFilter(ctx, u2.ID, filter.ID)
		if err == nil {
			t.Error("DeleteUserFilter(wrong user) expected error, got nil")
		}
	})
}

// ─── UpdateUserResume ──────────────────────────────────────────────────────

func TestUpdateUserResume(t *testing.T) {
	ctx := context.Background()

	t.Run("updates resume for existing user", func(t *testing.T) {
		s := openTestStore(t)
		user, err := s.UpsertUser(ctx, &models.User{
			Provider:       "google",
			ProviderID:     "resume-update",
			Email:          "resume@example.com",
			ResumeMarkdown: "# Old Resume",
		})
		if err != nil {
			t.Fatalf("UpsertUser error = %v", err)
		}

		err = s.UpdateUserResume(ctx, user.ID, "# New Resume\n\nUpdated content.")
		if err != nil {
			t.Fatalf("UpdateUserResume error = %v", err)
		}

		got, err := s.GetUser(ctx, user.ID)
		if err != nil {
			t.Fatalf("GetUser error = %v", err)
		}
		if got.ResumeMarkdown != "# New Resume\n\nUpdated content." {
			t.Errorf("ResumeMarkdown = %q, want updated content", got.ResumeMarkdown)
		}
	})

	t.Run("returns error for non-existent user", func(t *testing.T) {
		s := openTestStore(t)
		err := s.UpdateUserResume(ctx, 99999, "# Should Fail")
		if err == nil {
			t.Error("UpdateUserResume(nonexistent) expected error, got nil")
		}
	})
}

// ─── Provider Uniqueness ───────────────────────────────────────────────────

func TestUpsertUser_ProviderUniqueness(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)

	t.Run("same provider different ID creates separate users", func(t *testing.T) {
		u1, err := s.UpsertUser(ctx, &models.User{Provider: "google", ProviderID: "prov-a", Email: "a@test.com"})
		if err != nil {
			t.Fatalf("UpsertUser u1 error = %v", err)
		}
		u2, err := s.UpsertUser(ctx, &models.User{Provider: "google", ProviderID: "prov-b", Email: "b@test.com"})
		if err != nil {
			t.Fatalf("UpsertUser u2 error = %v", err)
		}
		if u1.ID == u2.ID {
			t.Error("same provider, different ID should create separate users")
		}
	})

	t.Run("different provider same ID creates separate users", func(t *testing.T) {
		u1, err := s.UpsertUser(ctx, &models.User{Provider: "google", ProviderID: "shared-id", Email: "g@test.com"})
		if err != nil {
			t.Fatalf("UpsertUser u1 error = %v", err)
		}
		u2, err := s.UpsertUser(ctx, &models.User{Provider: "github", ProviderID: "shared-id", Email: "gh@test.com"})
		if err != nil {
			t.Fatalf("UpsertUser u2 error = %v", err)
		}
		if u1.ID == u2.ID {
			t.Error("different provider, same ID should create separate users")
		}
	})
}

// ─── ListUserFilters for nonexistent user ──────────────────────────────────

func TestListUserFilters_NonexistentUser(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)

	filters, err := s.ListUserFilters(ctx, 99999)
	if err != nil {
		t.Fatalf("ListUserFilters error = %v", err)
	}
	if filters != nil && len(filters) != 0 {
		t.Errorf("expected empty result for nonexistent user, got %d", len(filters))
	}
}

// ─── DeleteUserFilter for nonexistent user+filter ──────────────────────────

func TestDeleteUserFilter_NonexistentUser(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)

	err := s.DeleteUserFilter(ctx, 99999, 99999)
	if err == nil {
		t.Error("DeleteUserFilter for nonexistent user+filter should return error")
	}
}

// ─── GetUser with negative ID ──────────────────────────────────────────────

func TestGetUser_NegativeID(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)

	_, err := s.GetUser(ctx, -1)
	if err == nil {
		t.Error("GetUser(-1) expected error, got nil")
	}
}

// ─── CreateUserFilter sets CreatedAt ────────────────────────────────────────

func TestCreateUserFilter_SetsCreatedAt(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)

	user, _ := s.UpsertUser(ctx, &models.User{Provider: "google", ProviderID: "ts-user", Email: "ts@test.com"})
	filter := &models.UserSearchFilter{Keywords: "timestamp-test"}

	before := time.Now().UTC().Add(-time.Second)
	err := s.CreateUserFilter(ctx, user.ID, filter)
	if err != nil {
		t.Fatalf("CreateUserFilter error = %v", err)
	}
	after := time.Now().UTC().Add(time.Second)

	if filter.CreatedAt.IsZero() {
		t.Fatal("CreatedAt is zero")
	}
	if filter.CreatedAt.Before(before) || filter.CreatedAt.After(after) {
		t.Errorf("CreatedAt = %v, want between %v and %v", filter.CreatedAt, before, after)
	}
}

// ─── UpdateUserNtfyTopic ───────────────────────────────────────────────────

func TestUpdateUserNtfyTopic(t *testing.T) {
	ctx := context.Background()

	t.Run("sets ntfy_topic for existing user", func(t *testing.T) {
		s := openTestStore(t)
		user, err := s.UpsertUser(ctx, &models.User{
			Provider:   "google",
			ProviderID: "ntfy-update",
			Email:      "ntfy@example.com",
		})
		if err != nil {
			t.Fatalf("UpsertUser error = %v", err)
		}

		if err := s.UpdateUserNtfyTopic(ctx, user.ID, "my-jobs-topic"); err != nil {
			t.Fatalf("UpdateUserNtfyTopic error = %v", err)
		}

		got, err := s.GetUser(ctx, user.ID)
		if err != nil {
			t.Fatalf("GetUser error = %v", err)
		}
		if got.NtfyTopic != "my-jobs-topic" {
			t.Errorf("NtfyTopic = %q, want my-jobs-topic", got.NtfyTopic)
		}
	})

	t.Run("clears ntfy_topic when set to empty string", func(t *testing.T) {
		s := openTestStore(t)
		user, _ := s.UpsertUser(ctx, &models.User{Provider: "google", ProviderID: "ntfy-clear", Email: "nc@example.com"})

		s.UpdateUserNtfyTopic(ctx, user.ID, "some-topic")
		if err := s.UpdateUserNtfyTopic(ctx, user.ID, ""); err != nil {
			t.Fatalf("UpdateUserNtfyTopic(empty) error = %v", err)
		}

		got, err := s.GetUser(ctx, user.ID)
		if err != nil {
			t.Fatalf("GetUser error = %v", err)
		}
		if got.NtfyTopic != "" {
			t.Errorf("NtfyTopic = %q, want empty string after clear", got.NtfyTopic)
		}
	})

	t.Run("returns error for non-existent user", func(t *testing.T) {
		s := openTestStore(t)
		err := s.UpdateUserNtfyTopic(ctx, 99999, "topic")
		if err == nil {
			t.Error("UpdateUserNtfyTopic(nonexistent) expected error, got nil")
		}
	})

	t.Run("new user has empty ntfy_topic by default", func(t *testing.T) {
		s := openTestStore(t)
		user, _ := s.UpsertUser(ctx, &models.User{Provider: "google", ProviderID: "ntfy-default", Email: "nd@example.com"})

		got, err := s.GetUser(ctx, user.ID)
		if err != nil {
			t.Fatalf("GetUser error = %v", err)
		}
		if got.NtfyTopic != "" {
			t.Errorf("NtfyTopic = %q, want empty string for new user", got.NtfyTopic)
		}
	})
}
