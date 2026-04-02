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
