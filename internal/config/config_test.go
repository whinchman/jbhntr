package config

import (
	"os"
	"path/filepath"
	"testing"
)

const sampleYAML = `
server:
  port: 8080
  base_url: "http://localhost:8080"
scraper:
  interval: "1h"
  serpapi_key: "${TEST_SERPAPI_KEY}"
search_filters:
  - keywords: "senior software engineer golang"
    location: "Remote"
    min_salary: 150000
    max_salary: 250000
  - keywords: "staff engineer go"
    location: "New York"
ntfy:
  server: "https://ntfy.sh"
claude:
  api_key: "${TEST_CLAUDE_KEY}"
  model: "claude-sonnet-4-20250514"
resume:
  path: "./resume.md"
output:
  dir: "./output"
`

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "config-*.yaml")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	f.Close()
	return f.Name()
}

func TestLoad(t *testing.T) {
	t.Run("parses all fields correctly", func(t *testing.T) {
		path := writeTemp(t, sampleYAML)

		cfg, err := Load(path)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		if cfg.Server.Port != 8080 {
			t.Errorf("Server.Port = %d, want 8080", cfg.Server.Port)
		}
		if cfg.Server.BaseURL != "http://localhost:8080" {
			t.Errorf("Server.BaseURL = %q, want %q", cfg.Server.BaseURL, "http://localhost:8080")
		}
		if cfg.Scraper.Interval != "1h" {
			t.Errorf("Scraper.Interval = %q, want %q", cfg.Scraper.Interval, "1h")
		}
		if len(cfg.SearchFilters) != 2 {
			t.Fatalf("len(SearchFilters) = %d, want 2", len(cfg.SearchFilters))
		}
		if cfg.SearchFilters[0].Keywords != "senior software engineer golang" {
			t.Errorf("SearchFilters[0].Keywords = %q", cfg.SearchFilters[0].Keywords)
		}
		if cfg.SearchFilters[0].MinSalary != 150000 {
			t.Errorf("SearchFilters[0].MinSalary = %d, want 150000", cfg.SearchFilters[0].MinSalary)
		}
		if cfg.SearchFilters[0].MaxSalary != 250000 {
			t.Errorf("SearchFilters[0].MaxSalary = %d, want 250000", cfg.SearchFilters[0].MaxSalary)
		}
		if cfg.SearchFilters[1].Location != "New York" {
			t.Errorf("SearchFilters[1].Location = %q, want %q", cfg.SearchFilters[1].Location, "New York")
		}
		if cfg.Ntfy.Server != "https://ntfy.sh" {
			t.Errorf("Ntfy.Server = %q", cfg.Ntfy.Server)
		}
		if cfg.Claude.Model != "claude-sonnet-4-20250514" {
			t.Errorf("Claude.Model = %q", cfg.Claude.Model)
		}
		if cfg.Resume.Path != "./resume.md" {
			t.Errorf("Resume.Path = %q", cfg.Resume.Path)
		}
		if cfg.Output.Dir != "./output" {
			t.Errorf("Output.Dir = %q", cfg.Output.Dir)
		}
	})

	t.Run("substitutes env vars", func(t *testing.T) {
		t.Setenv("TEST_SERPAPI_KEY", "serpkey123")
		t.Setenv("TEST_CLAUDE_KEY", "sk-ant-test")

		path := writeTemp(t, sampleYAML)
		cfg, err := Load(path)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		if cfg.Scraper.SerpAPIKey != "serpkey123" {
			t.Errorf("SerpAPIKey = %q, want %q", cfg.Scraper.SerpAPIKey, "serpkey123")
		}
		if cfg.Claude.APIKey != "sk-ant-test" {
			t.Errorf("Claude.APIKey = %q, want %q", cfg.Claude.APIKey, "sk-ant-test")
		}
	})

	t.Run("unset env var becomes empty string", func(t *testing.T) {
		os.Unsetenv("TEST_SERPAPI_KEY")
		path := writeTemp(t, sampleYAML)
		cfg, err := Load(path)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if cfg.Scraper.SerpAPIKey != "" {
			t.Errorf("SerpAPIKey = %q, want empty string for unset env var", cfg.Scraper.SerpAPIKey)
		}
	})

	t.Run("returns error for missing file", func(t *testing.T) {
		_, err := Load(filepath.Join(t.TempDir(), "nonexistent.yaml"))
		if err == nil {
			t.Error("Load() expected error for missing file, got nil")
		}
	})

	t.Run("returns error for invalid yaml", func(t *testing.T) {
		path := writeTemp(t, "server: [invalid: yaml: {")
		_, err := Load(path)
		if err == nil {
			t.Error("Load() expected error for invalid YAML, got nil")
		}
	})
}

func TestSMTPConfig(t *testing.T) {
	t.Run("parses smtp fields correctly", func(t *testing.T) {
		yaml := `
smtp:
  host: "smtp.example.com"
  port: 587
  username: "user@example.com"
  password: "secret"
  from: "noreply@example.com"
`
		path := writeTemp(t, yaml)
		cfg, err := Load(path)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if cfg.SMTP.Host != "smtp.example.com" {
			t.Errorf("SMTP.Host = %q, want %q", cfg.SMTP.Host, "smtp.example.com")
		}
		if cfg.SMTP.Port != 587 {
			t.Errorf("SMTP.Port = %d, want 587", cfg.SMTP.Port)
		}
		if cfg.SMTP.Username != "user@example.com" {
			t.Errorf("SMTP.Username = %q, want %q", cfg.SMTP.Username, "user@example.com")
		}
		if cfg.SMTP.Password != "secret" {
			t.Errorf("SMTP.Password = %q, want %q", cfg.SMTP.Password, "secret")
		}
		if cfg.SMTP.From != "noreply@example.com" {
			t.Errorf("SMTP.From = %q, want %q", cfg.SMTP.From, "noreply@example.com")
		}
	})

	t.Run("smtp block absent yields zero value (no panic)", func(t *testing.T) {
		path := writeTemp(t, "server:\n  port: 8080\n")
		cfg, err := Load(path)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if cfg.SMTP.Host != "" {
			t.Errorf("SMTP.Host = %q, want empty string when smtp block absent", cfg.SMTP.Host)
		}
		if cfg.SMTP.Port != 0 {
			t.Errorf("SMTP.Port = %d, want 0 when smtp block absent", cfg.SMTP.Port)
		}
	})
}

func TestOAuthConfig(t *testing.T) {
	t.Run("oauth.enabled true is parsed", func(t *testing.T) {
		yaml := `
auth:
  session_secret: "mysecret"
  oauth:
    enabled: true
  providers:
    google:
      client_id: "gid"
      client_secret: "gsecret"
`
		path := writeTemp(t, yaml)
		cfg, err := Load(path)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if !cfg.Auth.OAuth.Enabled {
			t.Errorf("Auth.OAuth.Enabled = false, want true")
		}
		// Ensure existing fields are unchanged.
		if cfg.Auth.SessionSecret != "mysecret" {
			t.Errorf("Auth.SessionSecret = %q, want %q", cfg.Auth.SessionSecret, "mysecret")
		}
		if cfg.Auth.Providers.Google.ClientID != "gid" {
			t.Errorf("Auth.Providers.Google.ClientID = %q, want %q", cfg.Auth.Providers.Google.ClientID, "gid")
		}
	})

	t.Run("oauth.enabled defaults to false when absent", func(t *testing.T) {
		yaml := `
auth:
  session_secret: "mysecret"
  providers:
    github:
      client_id: "ghid"
      client_secret: "ghsecret"
`
		path := writeTemp(t, yaml)
		cfg, err := Load(path)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if cfg.Auth.OAuth.Enabled {
			t.Errorf("Auth.OAuth.Enabled = true, want false when oauth block absent")
		}
		// Ensure existing fields are unchanged.
		if cfg.Auth.Providers.GitHub.ClientID != "ghid" {
			t.Errorf("Auth.Providers.GitHub.ClientID = %q, want %q", cfg.Auth.Providers.GitHub.ClientID, "ghid")
		}
	})
}
