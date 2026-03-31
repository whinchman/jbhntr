# Autonomous Agent Workflow

When launched autonomously, follow this workflow exactly. All project-specific
configuration lives in `agent.yaml` — read it first.

## Step 0: Pre-flight Checks

Before doing any work:

A) **Read `agent.yaml`** and internalize the project configuration. Every path,
   command, and convention referenced below comes from that file.

B) Ensure you are on the default branch (the `git.default_branch` value from
   `agent.yaml`). If not, switch to it.

C) Ensure the working directory is clean. If there are uncommitted changes,
   stash them and warn.

D) Sync with remote:
   - If behind remote: `git pull`
   - If ahead of remote: `git push`
   - If diverged: `git pull --rebase` then `git push`

## Step 1: Pick a Feature

Open the backlog file (`workflow.backlog_file` from `agent.yaml`) and find the
next unchecked `[ ]` item. This is your feature.

## Step 2: Plan

Create a plan for the feature with concrete implementation steps. Write the
plan to a markdown file at `<workflow.plans_dir>/<feature-name>.md`.

Each step should be:
- Small enough to implement in one sitting
- Independently testable (if testing is enabled)
- Independently committable

The plan should include:
- Overview of what will be built
- Step-by-step breakdown with specific files to create or modify
- Test cases for each step (if `testing.enabled` is true)
- Any dependencies or prerequisites between steps

## Step 3: Create a Worktree

Create a new git worktree for this feature:
```
git worktree add <workflow.worktrees_dir>/<feature-name> -b <git.feature_prefix><feature-name>
```
All implementation work happens in the worktree, not on the default branch.

## Step 4: Implement

For each step in the plan:

1. **If `testing.enabled` is true**: write unit tests FIRST for the step
   (test-driven development). Place tests according to `testing.test_dir`
   and `testing.test_pattern` from `agent.yaml`.

2. Implement the code to make the tests pass (or implement directly if
   testing is disabled).

3. **If `testing.enabled` is true**: run ALL tests (not just new ones)
   using `testing.command` from `agent.yaml`. Every test must pass.

4. Commit with a clear message describing what was implemented.
   If `git.commit_style` is `conventional`:
   ```
   feat(<feature-name>): <what this step accomplished>
   ```

**If testing is disabled**: do NOT write tests. Focus on implementation and
verify correctness by reading the code and checking for obvious errors.

## Step 5: Feature Complete

Once all plan steps are done:

1. **If testing is enabled**: run the full test suite one final time. ALL tests
   must pass.

2. **If a build command is configured**: run `build.command` from `agent.yaml`
   and verify success.

3. Switch back to the default branch:
   ```
   cd /workspace
   git checkout <git.default_branch>
   ```

4. Merge the feature branch:
   ```
   git merge <git.feature_prefix><feature-name>
   ```

5. Clean up the worktree:
   ```
   git worktree remove <workflow.worktrees_dir>/<feature-name>
   git branch -d <git.feature_prefix><feature-name>
   ```

6. Push the default branch to remote:
   ```
   git push
   ```

7. Mark the feature as `[x]` in the backlog file and commit that change.

## Step 6: Next Feature

Go back to Step 1 and pick the next feature. Repeat until the backlog is empty
or you run out of context.

## Context Window Management

If you are running low on context mid-feature:
1. Complete the current plan step
2. Commit your work
3. Note in the plan file which step you stopped at
4. The next agent session will resume from that point

## Code Standards

Follow the rules in the `code_standards` section of `agent.yaml`. Read them
during Step 0 and apply them to every file you create or modify.

Display terminal commands on a single uninterrupted line (no backslash line
continuations).

---

# JobHuntr Architecture Reference

This project is **jobhuntr** — a headless Go application that automates job searching, notifications, and tailored resume generation. Use this reference when implementing features.

## System Overview

```
Scheduler (hourly) → SerpAPI Google Jobs → SQLite Store
                                              ↓ new jobs
                                         ntfy.sh → Phone notification
                                              ↓ user opens link
                                         Web Dashboard (approve/reject)
                                              ↓ approved
                                    Claude API → Resume + Cover Letter HTML
                                              ↓
                                    go-rod → PDF files
                                              ↓
                                    Web Dashboard (view + download)
```

## Project Structure

```
cmd/jobhuntr/main.go              — Entry point, wires all subsystems
internal/config/config.go          — YAML config with ${ENV_VAR} substitution
internal/models/models.go          — Job, JobStatus, SearchFilter types
internal/store/store.go            — SQLite via modernc.org/sqlite (pure Go)
internal/scraper/source.go         — Source interface
internal/scraper/serpapi.go        — SerpAPI Google Jobs implementation
internal/scraper/scheduler.go      — Background scrape loop
internal/notifier/notifier.go      — ntfy.sh push notifications
internal/generator/generator.go    — Claude API resume/cover letter generation
internal/generator/prompts.go      — Prompt templates
internal/generator/worker.go       — Background generation worker
internal/pdf/pdf.go                — HTML→PDF via go-rod headless Chromium
internal/web/server.go             — chi router + handlers
internal/web/templates/            — Go html/template + HTMX partials
```

## Job State Machine

```
discovered → notified → approved → generating → complete
                      → rejected                → failed
```

Status transitions are enforced in the store layer. Only valid transitions are allowed.

## Database (SQLite + WAL mode)

Two tables:
- **jobs**: all listing data + status + generated HTML + PDF paths + error. UNIQUE(external_id, source) for dedup.
- **scrape_runs**: observability log of each scraping execution.

## Key Dependencies

| Package | Import Path | Purpose |
|---------|-------------|---------|
| SQLite | modernc.org/sqlite | Pure-Go SQLite (no CGO) |
| YAML | gopkg.in/yaml.v3 | Config parsing |
| chi | github.com/go-chi/chi/v5 | HTTP routing |
| go-rod | github.com/go-rod/rod | Headless Chrome for PDF |
| go-anthropic | github.com/liushuangls/go-anthropic/v2 | Claude API |
| SerpAPI | github.com/serpapi/serpapi-golang | Job search API |
| rate | golang.org/x/time/rate | Rate limiting |

## Config Format (config.yaml)

```yaml
server:
  port: 8080
  base_url: "http://localhost:8080"
scraper:
  interval: "1h"
  serpapi_key: "${SERPAPI_KEY}"
search_filters:
  - keywords: "senior software engineer golang"
    location: "Remote"
    min_salary: 150000
ntfy:
  topic: "jobhuntr"
  server: "https://ntfy.sh"
claude:
  api_key: "${ANTHROPIC_API_KEY}"
  model: "claude-sonnet-4-20250514"
resume:
  path: "./resume.md"
output:
  dir: "./output"
```

## Web Dashboard

- Go html/template + HTMX + Pico CSS (CDN)
- No JavaScript framework, no build step
- Templates embedded with //go:embed
- HTMX partials for approve/reject, filtering, search
- Auto-refresh table every 30s

## Notification Flow

1. Scheduler finds new jobs via SerpAPI
2. New jobs saved to DB with status "discovered"
3. Notifier sends ntfy.sh push with link to web dashboard
4. Status updated to "notified"
5. User opens dashboard from notification, clicks approve/reject
6. Approved jobs picked up by background worker for generation
