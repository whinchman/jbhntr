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
}
