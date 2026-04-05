package models

import "time"

// User represents an authenticated user.
type User struct {
	ID             int64
	Provider       string // "google" or "github"
	ProviderID     string // OAuth subject / user ID from the provider
	Email          string
	DisplayName    string
	AvatarURL      string
	ResumeMarkdown     string
	OnboardingComplete bool
	NtfyTopic         string
	CreatedAt          time.Time
	LastLoginAt        time.Time

	// Email/password authentication fields.
	// PasswordHash is nil for OAuth-only accounts.
	PasswordHash         *string
	EmailVerified        bool
	EmailVerifyToken     *string
	EmailVerifyExpiresAt *time.Time
	ResetToken           *string
	ResetExpiresAt       *time.Time

	// BannedAt is nil for active users; non-nil means the user has been banned.
	BannedAt *time.Time
}

// UserBannedTerm is one entry in a user's banned-keywords list.
type UserBannedTerm struct {
	ID        int64
	UserID    int64
	Term      string
	CreatedAt time.Time
}

// UserSearchFilter represents a per-user job search query configuration
// stored in the database.
type UserSearchFilter struct {
	ID        int64
	UserID    int64
	Keywords  string
	Location  string
	MinSalary int
	MaxSalary int
	Title     string
	CreatedAt time.Time
}
