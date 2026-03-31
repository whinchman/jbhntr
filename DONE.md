# Completed Work

Features moved here after merge to the default branch.

## Phase 1: Foundation

### 1A: Project Skeleton (merged 2026-03-31)
- Go module initialized: github.com/whinchman/jobhuntr
- Full directory structure: cmd/jobhuntr/, internal/{config,models,store,scraper,notifier,generator,pdf,web}/
- internal/config: YAML loading with ${ENV_VAR} substitution, table-driven tests
- config.yaml.example with all config sections
- .gitignore covering bin/, output/, worktrees/, *.db, config.yaml
- cmd/jobhuntr/main.go: loads config, structured slog output, prints startup message
