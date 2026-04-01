package store

import (
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// Migrate applies all pending SQL migrations in order.
// It creates the schema_migrations tracking table if it does not exist,
// then runs each migration file whose name has not been recorded yet.
// Each migration runs inside its own transaction.
func (s *Store) Migrate() error {
	if err := s.ensureMigrationsTable(); err != nil {
		return fmt.Errorf("store: ensure migrations table: %w", err)
	}

	applied, err := s.appliedMigrations()
	if err != nil {
		return fmt.Errorf("store: read applied migrations: %w", err)
	}

	files, err := s.pendingMigrations(applied)
	if err != nil {
		return fmt.Errorf("store: list pending migrations: %w", err)
	}

	for _, name := range files {
		if err := s.runMigration(name); err != nil {
			return fmt.Errorf("store: run migration %s: %w", name, err)
		}
	}
	return nil
}

// ensureMigrationsTable creates the schema_migrations table if it does
// not exist.
func (s *Store) ensureMigrationsTable() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			name       TEXT PRIMARY KEY,
			applied_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
		)
	`)
	return err
}

// appliedMigrations returns the set of migration names already applied.
func (s *Store) appliedMigrations() (map[string]bool, error) {
	rows, err := s.db.Query("SELECT name FROM schema_migrations")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	applied := make(map[string]bool)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		applied[name] = true
	}
	return applied, rows.Err()
}

// pendingMigrations reads the embedded migrations directory and returns
// file names not yet in the applied set, sorted lexicographically.
func (s *Store) pendingMigrations(applied map[string]bool) ([]string, error) {
	entries, err := fs.ReadDir(migrationFS, "migrations")
	if err != nil {
		return nil, err
	}

	var pending []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		if !applied[e.Name()] {
			pending = append(pending, e.Name())
		}
	}
	sort.Strings(pending)
	return pending, nil
}

// runMigration reads a single SQL file and executes it within a
// transaction, then records it in schema_migrations.
func (s *Store) runMigration(name string) error {
	content, err := migrationFS.ReadFile("migrations/" + name)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	if _, err := tx.Exec(string(content)); err != nil {
		return fmt.Errorf("exec sql: %w", err)
	}

	if _, err := tx.Exec(
		"INSERT INTO schema_migrations (name) VALUES (?)", name,
	); err != nil {
		return fmt.Errorf("record migration: %w", err)
	}

	return tx.Commit()
}
