# Claude Agent

A framework for running Claude Code as an autonomous, sandboxed coding agent on any project.

Claude Code runs inside a Docker container with full permissions (`--dangerously-skip-permissions`). The container **is** the sandbox — Claude can freely edit code, run builds, commit, and push, but cannot touch anything outside the container. You provide a feature backlog, and the agent works through it using test-driven development, git worktrees, and conventional commits.

## How It Works

```
┌─────────────────────────────────────────────────┐
│  scripts/agent.sh                               │
│  Starts the container and launches Claude Code   │
└──────────────────────┬──────────────────────────┘
                       │
                       ▼
┌─────────────────────────────────────────────────┐
│  Docker Container (the sandbox)                  │
│                                                  │
│  Claude Code reads CLAUDE.md + agent.yaml and    │
│  enters the autonomous loop:                     │
│                                                  │
│  Step 0: Pre-flight (sync git, check branch)     │
│  Step 1: Pick next [ ] feature from TODO.md      │
│  Step 2: Write implementation plan               │
│  Step 3: Create git worktree + feature branch    │
│  Step 4: TDD loop (tests → implement → commit)   │
│  Step 5: Merge to default branch, push, cleanup  │
│  Step 6: Loop back to Step 1                     │
└─────────────────────────────────────────────────┘
```

## Quick Start

### Prerequisites

- Docker (or Podman)
- An Anthropic API key or Claude Pro/Team subscription
- Claude Code CLI authenticated on your host (`claude login`)

### 1. Clone this repo (or use it as a template)

```
git clone https://github.com/youruser/claude-agent.git my-project
cd my-project
```

### 2. Add your project code

Either copy your existing code into this directory or adjust the volume mount in `docker-compose.yml` to point at your project.

### 3. Configure the agent

Edit `agent.yaml` with your project's settings:

```yaml
project:
  name: "my-app"
  description: "A web API built with FastAPI"

git:
  default_branch: "main"

testing:
  enabled: true
  command: "pytest -v"

code_standards: |
  - Follow PEP 8
  - Source code in src/
```

### 4. Add features to the backlog

Edit `TODO.md`:

```markdown
## Phase 1: Core Features

- [ ] Add user authentication with JWT
- [ ] Add CRUD endpoints for items
- [ ] Add rate limiting middleware
```

### 5. Launch the agent

```
scripts/agent.sh
```

That's it. The agent will pick the first unchecked feature, plan it, branch, implement with TDD, and merge back. It repeats until the backlog is empty or it runs out of context.

### Custom prompts

Skip the backlog workflow and give the agent a specific task:

```
scripts/agent.sh -p "Fix the race condition in src/worker.py"
```

## Extending the Dockerfile

The base image includes Ubuntu, git, Node.js, and Claude Code. To add your project's toolchain, replace the `Dockerfile` with one from `examples/` or write your own.

See `examples/` for ready-made setups:
- **`examples/godot/`** — Godot 4.6.1 + Android SDK + export templates
- **`examples/python/`** — Python 3.12 + pip + venv
- **`examples/node/`** — Node.js 22 + pnpm
- **`examples/rust/`** — Rust stable via rustup + build-essential

Each example includes a `Dockerfile` and an `agent.yaml`. Copy both to your project root and rebuild:

```
cp examples/python/Dockerfile .
cp examples/python/agent.yaml .
docker compose build
```

See [docs/extending-dockerfile.md](docs/extending-dockerfile.md) for a step-by-step guide.

## Security Model

The `--dangerously-skip-permissions` flag sounds scary, but the Docker container is the security boundary:

- The agent can only see files mounted into `/workspace`
- It cannot access your host filesystem, network services, or other containers (unless you configure it)
- Git credentials are mounted read-only from the host
- Resource limits (memory, CPU) are enforced by Docker

**Do not run `agent.sh` outside of Docker.** The `--dangerously-skip-permissions` flag is only safe because the container isolates the agent.

## Git Authentication

The agent needs to push/pull from your remote. Three options:

### SSH keys (recommended)

Add to `docker-compose.yml`:

```yaml
volumes:
  - ${HOME}/.ssh:/home/agent/.ssh:ro
```

### HTTPS token

Set in your environment before launching:

```
GIT_ASKPASS=echo GIT_TOKEN=ghp_xxx scripts/agent.sh
```

Or configure git credential storage inside the container.

### GitHub CLI

Add `gh` to your Dockerfile and mount `GITHUB_TOKEN`:

```yaml
environment:
  - GITHUB_TOKEN=${GITHUB_TOKEN}
```

## Configuration Reference

See [docs/configuration.md](docs/configuration.md) for the full `agent.yaml` field reference.

| Field | Default | Description |
|-------|---------|-------------|
| `project.name` | `"my-project"` | Name used in commits and plans |
| `git.default_branch` | `"main"` | Branch the agent works from |
| `git.feature_prefix` | `"feature/"` | Prefix for feature branches |
| `git.commit_style` | `"conventional"` | `conventional` or `freeform` |
| `testing.enabled` | `true` | Whether to use TDD |
| `testing.command` | `"npm test"` | Command to run all tests |
| `build.command` | `""` | Optional build command |
| `code_standards` | `""` | Project-specific coding rules |

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `AGENT_SERVICE` | `agent` | Docker Compose service name |
| `AGENT_MEMORY` | `8G` | Container memory limit |
| `AGENT_CPUS` | `4` | Container CPU limit |
| `UID` | `1000` | Container user UID (match your host) |
| `GID` | `1000` | Container user GID (match your host) |

## Documentation

- [Architecture](docs/architecture.md) — How the system works end-to-end
- [Configuration](docs/configuration.md) — Full `agent.yaml` reference
- [Extending the Dockerfile](docs/extending-dockerfile.md) — Adding your toolchain
- [Troubleshooting](docs/troubleshooting.md) — Common issues and fixes

## License

MIT
