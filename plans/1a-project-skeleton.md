# Plan: 1A — Project Skeleton

## Overview
Initialize the Go module, create the full directory structure, write a config.yaml template,
add .gitignore, and produce a main.go that loads config and prints a startup message.

## Steps

### Step 1: Initialize Go module and directory structure
- Run `go mod init github.com/whinchman/jobhuntr`
- Create directories: cmd/jobhuntr/, internal/{config,models,store,scraper,notifier,generator,pdf,web}/
- Create placeholder files (doc.go) in each internal package so the dirs are tracked

### Step 2: Write config.yaml template
- File: config.yaml with all sections (server, scraper, search_filters, ntfy, claude, resume, output)
- Secrets via ${ENV_VAR} placeholders

### Step 3: Write .gitignore
- Entries: bin/, output/, worktrees/, *.db, config.yaml (config.yaml.example is checked in)

### Step 4: Implement internal/config/config.go
- Load YAML from path, substitute ${VAR} → os.Getenv(VAR) in values
- Structs: Config, ServerConfig, ScraperConfig, SearchFilter, NtfyConfig, ClaudeConfig, ResumeConfig, OutputConfig

### Step 5: Implement cmd/jobhuntr/main.go
- Parse --config flag (default: config.yaml)
- Load config via internal/config
- Set up slog with structured output
- Print "jobhuntr starting on :PORT" and block (placeholder — no real server yet)

### Step 6: Write tests
- internal/config/config_test.go: table-driven tests for YAML parsing and ${VAR} substitution
- go test ./... must pass

## Files Created
- go.mod, go.sum
- config.yaml.example
- .gitignore
- cmd/jobhuntr/main.go
- internal/config/config.go
- internal/config/config_test.go
- internal/{models,store,scraper,notifier,generator,pdf,web}/doc.go (package stubs)
