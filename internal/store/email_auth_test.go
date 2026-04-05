package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/whinchman/jobhuntr/internal/models"
)

// ─── CreateUserWithPassword ──────────────────────────────────────────────────

func TestCreateUserWithPassword(t *testing.T) {
	ctx := context.Background()

	t.Run("inserts new email/password user", func(t *testing.T) {
		s := openTestStore(t)
		expiry := time.Now().UTC().Add(24 * time.Hour)
		u, err := s.CreateUserWithPassword(ctx, "new@example.com", "New User", "$2a$10$hash", "tok123", expiry)
		if err != nil {
			t.Fatalf("CreateUserWithPassword error = %v", err)
		}
		if u.ID == 0 {
			t.Error("ID not set after insert")
		}
		if u.Email != "new@example.com" {
			t.Errorf("Email = %q, want new@example.com", u.Email)
		}
		if u.DisplayName != "New User" {
			t.Errorf("DisplayName = %q, want New User", u.DisplayName)
		}
		if u.EmailVerified {
			t.Error("EmailVerified should be false for new password user")
		}
		if u.PasswordHash == nil || *u.PasswordHash != "$2a$10$hash" {
			t.Errorf("PasswordHash = %v, want $2a$10$hash", u.PasswordHash)
		}
		if u.EmailVerifyToken == nil || *u.EmailVerifyToken != "tok123" {
			t.Errorf("EmailVerifyToken = %v, want tok123", u.EmailVerifyToken)
		}
		if u.EmailVerifyExpiresAt == nil {
			t.Error("EmailVerifyExpiresAt should not be nil")
		}
	})

	t.Run("returns ErrEmailTaken on duplicate email", func(t *testing.T) {
		s := openTestStore(t)
		expiry := time.Now().UTC().Add(time.Hour)
		_, err := s.CreateUserWithPassword(ctx, "dup@example.com", "First", "hash1", "tok-a", expiry)
		if err != nil {
			t.Fatalf("first CreateUserWithPassword error = %v", err)
		}
		_, err = s.CreateUserWithPassword(ctx, "dup@example.com", "Second", "hash2", "tok-b", expiry)
		if !errors.Is(err, ErrEmailTaken) {
			t.Errorf("expected ErrEmailTaken, got %v", err)
		}
	})
}

// ─── GetUserByEmail ──────────────────────────────────────────────────────────

func TestGetUserByEmail(t *testing.T) {
	ctx := context.Background()

	t.Run("returns user by email", func(t *testing.T) {
		s := openTestStore(t)
		expiry := time.Now().UTC().Add(time.Hour)
		created, err := s.CreateUserWithPassword(ctx, "byemail@example.com", "Email User", "hash", "tok", expiry)
		if err != nil {
			t.Fatalf("CreateUserWithPassword error = %v", err)
		}

		got, err := s.GetUserByEmail(ctx, "byemail@example.com")
		if err != nil {
			t.Fatalf("GetUserByEmail error = %v", err)
		}
		if got == nil {
			t.Fatal("GetUserByEmail returned nil, want user")
		}
		if got.ID != created.ID {
			t.Errorf("ID = %d, want %d", got.ID, created.ID)
		}
	})

	t.Run("returns nil for unknown email", func(t *testing.T) {
		s := openTestStore(t)
		got, err := s.GetUserByEmail(ctx, "nobody@example.com")
		if err != nil {
			t.Fatalf("GetUserByEmail error = %v", err)
		}
		if got != nil {
			t.Errorf("expected nil for unknown email, got user %d", got.ID)
		}
	})
}

// ─── SetResetToken / ConsumeResetToken ──────────────────────────────────────

func TestResetToken(t *testing.T) {
	ctx := context.Background()

	t.Run("set and consume reset token", func(t *testing.T) {
		s := openTestStore(t)
		expiry := time.Now().UTC().Add(time.Hour)
		u, err := s.CreateUserWithPassword(ctx, "reset@example.com", "Reset User", "oldhash", "verifytok", expiry)
		if err != nil {
			t.Fatalf("CreateUserWithPassword error = %v", err)
		}

		resetExpiry := time.Now().UTC().Add(time.Hour)
		if err := s.SetResetToken(ctx, u.ID, "resettoken123", resetExpiry); err != nil {
			t.Fatalf("SetResetToken error = %v", err)
		}

		got, err := s.ConsumeResetToken(ctx, "resettoken123", "newhash")
		if err != nil {
			t.Fatalf("ConsumeResetToken error = %v", err)
		}
		if got == nil {
			t.Fatal("ConsumeResetToken returned nil, want user")
		}
		if got.ID != u.ID {
			t.Errorf("ID = %d, want %d", got.ID, u.ID)
		}
		if got.PasswordHash == nil || *got.PasswordHash != "newhash" {
			t.Errorf("PasswordHash = %v, want newhash", got.PasswordHash)
		}
		if got.ResetToken != nil {
			t.Errorf("ResetToken should be nil after consume, got %v", *got.ResetToken)
		}
		if got.ResetExpiresAt != nil {
			t.Error("ResetExpiresAt should be nil after consume")
		}
	})

	t.Run("consume returns nil for unknown token", func(t *testing.T) {
		s := openTestStore(t)
		got, err := s.ConsumeResetToken(ctx, "nonexistent-token", "hash")
		if err != nil {
			t.Fatalf("ConsumeResetToken error = %v", err)
		}
		if got != nil {
			t.Errorf("expected nil for unknown token, got user %d", got.ID)
		}
	})

	t.Run("consume returns nil for expired token", func(t *testing.T) {
		s := openTestStore(t)
		expiry := time.Now().UTC().Add(time.Hour)
		u, err := s.CreateUserWithPassword(ctx, "expired@example.com", "Expired", "hash", "tok", expiry)
		if err != nil {
			t.Fatalf("CreateUserWithPassword error = %v", err)
		}

		// Set token that is already expired.
		pastExpiry := time.Now().UTC().Add(-time.Hour)
		if err := s.SetResetToken(ctx, u.ID, "expiredtok", pastExpiry); err != nil {
			t.Fatalf("SetResetToken error = %v", err)
		}

		got, err := s.ConsumeResetToken(ctx, "expiredtok", "newhash")
		if err != nil {
			t.Fatalf("ConsumeResetToken error = %v", err)
		}
		if got != nil {
			t.Errorf("expected nil for expired token, got user %d", got.ID)
		}
	})
}

// ─── SetEmailVerifyToken / ConsumeVerifyToken ────────────────────────────────

func TestEmailVerifyToken(t *testing.T) {
	ctx := context.Background()

	t.Run("set and consume verify token", func(t *testing.T) {
		s := openTestStore(t)
		expiry := time.Now().UTC().Add(time.Hour)
		u, err := s.CreateUserWithPassword(ctx, "verify@example.com", "Verify User", "hash", "verifytok", expiry)
		if err != nil {
			t.Fatalf("CreateUserWithPassword error = %v", err)
		}
		if u.EmailVerified {
			t.Fatal("new user should not be email verified")
		}

		// Override with a known token for the consume test.
		newExpiry := time.Now().UTC().Add(time.Hour)
		if err := s.SetEmailVerifyToken(ctx, u.ID, "knowntok", newExpiry); err != nil {
			t.Fatalf("SetEmailVerifyToken error = %v", err)
		}

		got, err := s.ConsumeVerifyToken(ctx, "knowntok")
		if err != nil {
			t.Fatalf("ConsumeVerifyToken error = %v", err)
		}
		if got == nil {
			t.Fatal("ConsumeVerifyToken returned nil, want user")
		}
		if !got.EmailVerified {
			t.Error("EmailVerified should be true after consume")
		}
		if got.EmailVerifyToken != nil {
			t.Errorf("EmailVerifyToken should be nil after consume, got %v", *got.EmailVerifyToken)
		}
	})

	t.Run("consume returns nil for unknown token", func(t *testing.T) {
		s := openTestStore(t)
		got, err := s.ConsumeVerifyToken(ctx, "nonexistent")
		if err != nil {
			t.Fatalf("ConsumeVerifyToken error = %v", err)
		}
		if got != nil {
			t.Errorf("expected nil for unknown token, got user %d", got.ID)
		}
	})

	t.Run("consume returns nil for expired token", func(t *testing.T) {
		s := openTestStore(t)
		expiry := time.Now().UTC().Add(time.Hour)
		u, err := s.CreateUserWithPassword(ctx, "expvfy@example.com", "Exp Vfy", "hash", "tok", expiry)
		if err != nil {
			t.Fatalf("CreateUserWithPassword error = %v", err)
		}

		pastExpiry := time.Now().UTC().Add(-time.Hour)
		if err := s.SetEmailVerifyToken(ctx, u.ID, "expiredvfy", pastExpiry); err != nil {
			t.Fatalf("SetEmailVerifyToken error = %v", err)
		}

		got, err := s.ConsumeVerifyToken(ctx, "expiredvfy")
		if err != nil {
			t.Fatalf("ConsumeVerifyToken error = %v", err)
		}
		if got != nil {
			t.Errorf("expected nil for expired token, got user %d", got.ID)
		}
	})
}

// ─── OAuth users default EmailVerified = true ────────────────────────────────

func TestOAuthUser_EmailVerifiedDefault(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)

	u, err := s.UpsertUser(ctx, &models.User{
		Provider:   "google",
		ProviderID: "oauth-vfy",
		Email:      "oauthvfy@example.com",
	})
	if err != nil {
		t.Fatalf("UpsertUser error = %v", err)
	}

	got, err := s.GetUser(ctx, u.ID)
	if err != nil {
		t.Fatalf("GetUser error = %v", err)
	}
	if !got.EmailVerified {
		t.Error("OAuth user should have EmailVerified = true by default")
	}
}
