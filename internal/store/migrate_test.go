package store

import (
	"testing"
)

func TestMigrate(t *testing.T) {
	t.Run("creates schema_migrations table", func(t *testing.T) {
		s := openTestStore(t)
		var count int
		err := s.db.QueryRow(
			"SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'schema_migrations'",
		).Scan(&count)
		if err != nil {
			t.Fatalf("query error = %v", err)
		}
		if count != 1 {
			t.Errorf("schema_migrations table count = %d, want 1", count)
		}
	})

	t.Run("applies all migrations", func(t *testing.T) {
		s := openTestStore(t)
		rows, err := s.db.Query("SELECT name FROM schema_migrations ORDER BY name")
		if err != nil {
			t.Fatalf("query error = %v", err)
		}
		defer rows.Close()

		var names []string
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil {
				t.Fatalf("scan error = %v", err)
			}
			names = append(names, name)
		}
		if err := rows.Err(); err != nil {
			t.Fatalf("rows error = %v", err)
		}

		expected := []string{
			"001_create_users.sql",
			"002_create_user_search_filters.sql",
			"003_add_user_id_to_jobs.sql",
			"004_rebuild_jobs_unique_constraint.sql",
			"005_add_onboarding_complete.sql",
			"006_rebuild_jobs_drop_legacy_unique.sql",
			"007_add_ntfy_topic_to_users.sql",
			"008_add_markdown_columns.sql",
			"009_add_email_auth.sql",
			"010_add_banned_at_to_users.sql",
			"011_add_application_status.sql",
			"012_add_user_banned_terms.sql",
			"013_add_dedup_hash.sql",
			"014_add_google_drive_tokens.sql",
		}
		if len(names) != len(expected) {
			t.Fatalf("migrations applied = %d, want %d: %v", len(names), len(expected), names)
		}
		for i, name := range names {
			if name != expected[i] {
				t.Errorf("migration[%d] = %q, want %q", i, name, expected[i])
			}
		}
	})

	t.Run("is idempotent", func(t *testing.T) {
		s := openTestStore(t)
		// Migrate was already called by Open. Call it again.
		if err := s.Migrate(); err != nil {
			t.Fatalf("second Migrate() error = %v", err)
		}
	})

	t.Run("creates users table", func(t *testing.T) {
		s := openTestStore(t)
		_, err := s.db.Exec(
			"INSERT INTO users (provider, provider_id, email) VALUES ('test', '123', 'a@b.com') ON CONFLICT DO NOTHING",
		)
		if err != nil {
			t.Fatalf("INSERT into users error = %v", err)
		}
	})

	t.Run("creates user_search_filters table", func(t *testing.T) {
		s := openTestStore(t)
		// Need a user first for the FK.
		s.db.Exec("INSERT INTO users (provider, provider_id) VALUES ('test', '456') ON CONFLICT DO NOTHING")
		var userID int64
		s.db.QueryRow("SELECT id FROM users WHERE provider='test' AND provider_id='456'").Scan(&userID)
		_, err := s.db.Exec(
			"INSERT INTO user_search_filters (user_id, keywords) VALUES ($1, 'golang')", userID,
		)
		if err != nil {
			t.Fatalf("INSERT into user_search_filters error = %v", err)
		}
	})

	t.Run("adds user_id to jobs", func(t *testing.T) {
		s := openTestStore(t)
		_, err := s.db.Exec(
			"INSERT INTO jobs (user_id, external_id, source, status) VALUES (0, 'mig-test', 'test', 'discovered')",
		)
		if err != nil {
			t.Fatalf("INSERT into jobs with user_id error = %v", err)
		}
	})

	t.Run("adds application_status columns to jobs", func(t *testing.T) {
		s := openTestStore(t)
		// Verify columns exist and accept valid values.
		_, err := s.db.Exec(`
			INSERT INTO jobs (user_id, external_id, source, status, application_status, applied_at)
			VALUES (0, 'mig-appstatus', 'test', 'discovered', 'applied', NOW())
		`)
		if err != nil {
			t.Fatalf("INSERT with application_status=applied error = %v", err)
		}
	})

	t.Run("application_status CHECK constraint rejects invalid values", func(t *testing.T) {
		s := openTestStore(t)
		_, err := s.db.Exec(`
			INSERT INTO jobs (user_id, external_id, source, status, application_status)
			VALUES (0, 'mig-badstatus', 'test', 'discovered', 'invalid_value')
		`)
		if err == nil {
			t.Fatal("INSERT with invalid application_status should have failed, got nil")
		}
	})

	t.Run("application_status columns are nullable", func(t *testing.T) {
		s := openTestStore(t)
		_, err := s.db.Exec(`
			INSERT INTO jobs (user_id, external_id, source, status)
			VALUES (0, 'mig-nullstatus', 'test', 'discovered')
		`)
		if err != nil {
			t.Fatalf("INSERT with NULL application_status columns error = %v", err)
		}
	})

	t.Run("creates user_banned_terms table", func(t *testing.T) {
		s := openTestStore(t)
		// Insert a user and a banned term to verify the table and FK exist.
		s.db.Exec("INSERT INTO users (provider, provider_id, email) VALUES ('test', 'banned-mig', 'banned@mig.test') ON CONFLICT DO NOTHING")
		var userID int64
		s.db.QueryRow("SELECT id FROM users WHERE provider='test' AND provider_id='banned-mig'").Scan(&userID)
		_, err := s.db.Exec(
			"INSERT INTO user_banned_terms (user_id, term) VALUES ($1, 'spam') ON CONFLICT DO NOTHING", userID,
		)
		if err != nil {
			t.Fatalf("INSERT into user_banned_terms error = %v", err)
		}
	})

	t.Run("user_banned_terms unique constraint prevents duplicate terms per user", func(t *testing.T) {
		s := openTestStore(t)
		s.db.Exec("INSERT INTO users (provider, provider_id, email) VALUES ('test', 'banned-mig2', 'banned2@mig.test') ON CONFLICT DO NOTHING")
		var userID int64
		s.db.QueryRow("SELECT id FROM users WHERE provider='test' AND provider_id='banned-mig2'").Scan(&userID)
		s.db.Exec("INSERT INTO user_banned_terms (user_id, term) VALUES ($1, 'dup-term') ON CONFLICT DO NOTHING", userID)
		_, err := s.db.Exec("INSERT INTO user_banned_terms (user_id, term) VALUES ($1, 'dup-term')", userID)
		if err == nil {
			t.Fatal("duplicate banned term insert should have failed, got nil")
		}
	})

	t.Run("creates user_google_tokens table", func(t *testing.T) {
		s := openTestStore(t)
		s.db.Exec("INSERT INTO users (provider, provider_id, email) VALUES ('test', 'gdrive-mig', 'gdrive@mig.test') ON CONFLICT DO NOTHING")
		var userID int64
		s.db.QueryRow("SELECT id FROM users WHERE provider='test' AND provider_id='gdrive-mig'").Scan(&userID)
		_, err := s.db.Exec(
			"INSERT INTO user_google_tokens (user_id, token_json) VALUES ($1, $2)",
			userID, `{"access_token":"tok","token_type":"Bearer"}`,
		)
		if err != nil {
			t.Fatalf("INSERT into user_google_tokens error = %v", err)
		}
	})

	t.Run("user_google_tokens primary key prevents duplicate user rows", func(t *testing.T) {
		s := openTestStore(t)
		s.db.Exec("INSERT INTO users (provider, provider_id, email) VALUES ('test', 'gdrive-mig3', 'gdrive3@mig.test') ON CONFLICT DO NOTHING")
		var userID int64
		s.db.QueryRow("SELECT id FROM users WHERE provider='test' AND provider_id='gdrive-mig3'").Scan(&userID)
		s.db.Exec("INSERT INTO user_google_tokens (user_id, token_json) VALUES ($1, '{}') ON CONFLICT DO NOTHING", userID)
		_, err := s.db.Exec("INSERT INTO user_google_tokens (user_id, token_json) VALUES ($1, '{}')", userID)
		if err == nil {
			t.Fatal("duplicate user_google_tokens insert should have failed, got nil")
		}
	})
}
