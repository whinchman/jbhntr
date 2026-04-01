// Package config loads and parses the jobhuntr YAML configuration file,
// substituting ${ENV_VAR} placeholders with values from the environment.
package config

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config is the root configuration object.
type Config struct {
	Server        ServerConfig   `yaml:"server"`
	Auth          AuthConfig     `yaml:"auth"`
	Scraper       ScraperConfig  `yaml:"scraper"`
	// SearchFilters holds global search filters parsed from the config file.
	// Deprecated: per-user search filters are now stored in the database
	// (user_search_filters table) and managed via the web UI. This field is
	// retained for backward compatibility with config parsing tests.
	SearchFilters []SearchFilter `yaml:"search_filters"`
	Ntfy          NtfyConfig     `yaml:"ntfy"`
	Claude        ClaudeConfig   `yaml:"claude"`
	// Resume is the fallback resume file for the generator worker. Per-user
	// resumes are stored in the database (users.resume_markdown).
	Resume        ResumeConfig   `yaml:"resume"`
	Output        OutputConfig   `yaml:"output"`
}

// AuthConfig holds OAuth and session configuration.
type AuthConfig struct {
	SessionSecret string          `yaml:"session_secret"`
	Providers     ProvidersConfig `yaml:"providers"`
}

// ProvidersConfig holds per-provider OAuth credentials.
type ProvidersConfig struct {
	Google OAuthProviderConfig `yaml:"google"`
	GitHub OAuthProviderConfig `yaml:"github"`
}

// OAuthProviderConfig holds OAuth client credentials for a single provider.
type OAuthProviderConfig struct {
	ClientID     string `yaml:"client_id"`
	ClientSecret string `yaml:"client_secret"`
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Port    int    `yaml:"port"`
	BaseURL string `yaml:"base_url"`
}

// ScraperConfig holds scraping settings.
type ScraperConfig struct {
	Interval   string `yaml:"interval"`
	SerpAPIKey string `yaml:"serpapi_key"`
}

// SearchFilter represents a single job search query.
type SearchFilter struct {
	Keywords  string `yaml:"keywords"`
	Location  string `yaml:"location"`
	MinSalary int    `yaml:"min_salary"`
	MaxSalary int    `yaml:"max_salary"`
	Title     string `yaml:"title"`
}

// NtfyConfig holds ntfy.sh notification settings.
type NtfyConfig struct {
	Topic  string `yaml:"topic"`
	Server string `yaml:"server"`
}

// ClaudeConfig holds Anthropic Claude API settings.
type ClaudeConfig struct {
	APIKey string `yaml:"api_key"`
	Model  string `yaml:"model"`
}

// ResumeConfig points to the base resume file.
type ResumeConfig struct {
	Path string `yaml:"path"`
}

// OutputConfig specifies where generated files are written.
type OutputConfig struct {
	Dir string `yaml:"dir"`
}

var envVarRe = regexp.MustCompile(`\$\{([^}]+)\}`)

// loadDotenv reads a .env file and sets any variables not already in the
// environment. It silently does nothing if the file doesn't exist.
func loadDotenv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		// Only set if not already in the environment (explicit env takes priority).
		if os.Getenv(key) == "" {
			os.Setenv(key, val)
		}
	}
}

// Load reads a YAML config file, substitutes ${ENV_VAR} placeholders,
// and returns the parsed Config. It loads .env from the config file's
// directory before substitution so secrets don't need to be exported.
func Load(path string) (*Config, error) {
	// Try loading .env from the same directory as the config file.
	dir := "."
	if i := strings.LastIndexAny(path, "/\\"); i >= 0 {
		dir = path[:i]
	}
	loadDotenv(dir + "/.env")

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config: read file: %w", err)
	}

	expanded := envVarRe.ReplaceAllStringFunc(string(data), func(match string) string {
		name := envVarRe.FindStringSubmatch(match)[1]
		return os.Getenv(name)
	})

	var cfg Config
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return nil, fmt.Errorf("config: parse yaml: %w", err)
	}

	return &cfg, nil
}
