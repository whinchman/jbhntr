package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/whinchman/jobhuntr/internal/models"
)

// ListActiveUserIDs returns the IDs of all users that have at least one
// search filter configured. These are the users the scheduler should scrape
// for.
func (s *Store) ListActiveUserIDs(ctx context.Context) ([]int64, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT user_id
		FROM user_search_filters
		ORDER BY user_id`)
	if err != nil {
		return nil, fmt.Errorf("store: list active user IDs: %w", err)
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("store: list active user IDs scan: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// UpsertUser inserts a new user or updates the last_login_at if the user
// already exists (matched on provider + provider_id). It returns the user
// with its database ID populated. The resume_markdown and onboarding_complete
// fields are not overwritten on conflict — login should not erase a user's
// resume or reset their onboarding status.
func (s *Store) UpsertUser(ctx context.Context, user *models.User) (*models.User, error) {
	now := time.Now().UTC().Format(time.RFC3339)

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO users (provider, provider_id, email, display_name, avatar_url, resume_markdown, onboarding_complete, last_login_at)
		VALUES ($1, $2, $3, $4, $5, $6, 0, $7)
		ON CONFLICT(provider, provider_id) DO UPDATE SET
			email = excluded.email,
			display_name = excluded.display_name,
			avatar_url = excluded.avatar_url,
			last_login_at = excluded.last_login_at`,
		user.Provider, user.ProviderID, user.Email,
		user.DisplayName, user.AvatarURL, user.ResumeMarkdown, now,
	)
	if err != nil {
		return nil, fmt.Errorf("store: upsert user: %w", err)
	}

	return s.GetUserByProvider(ctx, user.Provider, user.ProviderID)
}

// GetUser retrieves a user by primary key. Returns an error if not found.
func (s *Store) GetUser(ctx context.Context, id int64) (*models.User, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, provider, provider_id, email, display_name, avatar_url,
		       resume_markdown, onboarding_complete, created_at, last_login_at
		FROM users WHERE id = $1`, id)

	u, err := scanUser(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("store: user %d not found", id)
		}
		return nil, fmt.Errorf("store: get user: %w", err)
	}
	return u, nil
}

// GetUserByProvider retrieves a user by provider name and provider-specific
// ID. Returns an error if not found.
func (s *Store) GetUserByProvider(ctx context.Context, provider, providerID string) (*models.User, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, provider, provider_id, email, display_name, avatar_url,
		       resume_markdown, onboarding_complete, created_at, last_login_at
		FROM users WHERE provider = $1 AND provider_id = $2`, provider, providerID)

	u, err := scanUser(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("store: user %s/%s not found", provider, providerID)
		}
		return nil, fmt.Errorf("store: get user by provider: %w", err)
	}
	return u, nil
}

// CreateUserFilter inserts a new search filter for the given user.
// The filter's ID and CreatedAt fields are populated on return.
func (s *Store) CreateUserFilter(ctx context.Context, userID int64, filter *models.UserSearchFilter) error {
	var id int64
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO user_search_filters (user_id, keywords, location, min_salary, max_salary, title)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id`,
		userID, filter.Keywords, filter.Location,
		filter.MinSalary, filter.MaxSalary, filter.Title,
	).Scan(&id)
	if err != nil {
		return fmt.Errorf("store: create user filter: %w", err)
	}
	filter.ID = id
	filter.UserID = userID
	filter.CreatedAt = time.Now().UTC()
	return nil
}

// ListUserFilters returns all search filters belonging to the given user,
// ordered by created_at DESC.
func (s *Store) ListUserFilters(ctx context.Context, userID int64) ([]models.UserSearchFilter, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, keywords, location, min_salary, max_salary, title, created_at
		FROM user_search_filters
		WHERE user_id = $1
		ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("store: list user filters: %w", err)
	}
	defer rows.Close()

	var filters []models.UserSearchFilter
	for rows.Next() {
		f, err := scanUserFilter(rows)
		if err != nil {
			return nil, fmt.Errorf("store: list user filters scan: %w", err)
		}
		filters = append(filters, *f)
	}
	return filters, rows.Err()
}

// DeleteUserFilter deletes a search filter by ID, scoped to the given user.
// Returns an error if the filter does not exist or does not belong to the user.
func (s *Store) DeleteUserFilter(ctx context.Context, userID int64, filterID int64) error {
	res, err := s.db.ExecContext(ctx,
		"DELETE FROM user_search_filters WHERE id = $1 AND user_id = $2",
		filterID, userID,
	)
	if err != nil {
		return fmt.Errorf("store: delete user filter: %w", err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("store: delete user filter rows affected: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("store: filter %d not found for user %d", filterID, userID)
	}
	return nil
}

// UpdateUserResume updates the resume_markdown column for the given user.
func (s *Store) UpdateUserResume(ctx context.Context, userID int64, markdown string) error {
	res, err := s.db.ExecContext(ctx,
		"UPDATE users SET resume_markdown = $1 WHERE id = $2",
		markdown, userID,
	)
	if err != nil {
		return fmt.Errorf("store: update user resume: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("store: update user resume rows affected: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("store: user %d not found", userID)
	}
	return nil
}

// UpdateUserOnboarding sets the display_name, resume_markdown, and
// onboarding_complete = 1 for the given user. This is called when the user
// completes the onboarding flow.
func (s *Store) UpdateUserOnboarding(ctx context.Context, userID int64, displayName string, resume string) error {
	res, err := s.db.ExecContext(ctx,
		"UPDATE users SET display_name = $1, resume_markdown = $2, onboarding_complete = 1 WHERE id = $3",
		displayName, resume, userID,
	)
	if err != nil {
		return fmt.Errorf("store: update user onboarding: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("store: update user onboarding rows affected: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("store: user %d not found", userID)
	}
	return nil
}

// UpdateUserDisplayName updates the display_name column for the given user.
func (s *Store) UpdateUserDisplayName(ctx context.Context, userID int64, displayName string) error {
	res, err := s.db.ExecContext(ctx,
		"UPDATE users SET display_name = $1 WHERE id = $2",
		displayName, userID,
	)
	if err != nil {
		return fmt.Errorf("store: update user display name: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("store: update user display name rows affected: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("store: user %d not found", userID)
	}
	return nil
}

// scanUser scans a single user row into a models.User.
func scanUser(s scanner) (*models.User, error) {
	var u models.User
	var createdAt, lastLoginAt string
	var onboardingComplete int
	err := s.Scan(
		&u.ID, &u.Provider, &u.ProviderID, &u.Email,
		&u.DisplayName, &u.AvatarURL, &u.ResumeMarkdown,
		&onboardingComplete, &createdAt, &lastLoginAt,
	)
	if err != nil {
		return nil, err
	}
	u.OnboardingComplete = onboardingComplete != 0
	if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
		u.CreatedAt = t
	}
	if t, err := time.Parse(time.RFC3339, lastLoginAt); err == nil {
		u.LastLoginAt = t
	}
	return &u, nil
}

// scanUserFilter scans a single user_search_filters row.
func scanUserFilter(s scanner) (*models.UserSearchFilter, error) {
	var f models.UserSearchFilter
	var createdAt string
	err := s.Scan(
		&f.ID, &f.UserID, &f.Keywords, &f.Location,
		&f.MinSalary, &f.MaxSalary, &f.Title, &createdAt,
	)
	if err != nil {
		return nil, err
	}
	if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
		f.CreatedAt = t
	}
	return &f, nil
}
