# Architecture

The autonomous agent framework has three layers:

```
┌─────────────────────────────────────────────────┐
│ Layer 3: Configuration (agent.yaml)              │
│ Project-specific: branch names, test commands,   │
│ code standards, backlog file paths               │
├─────────────────────────────────────────────────┤
│ Layer 2: Workflow (CLAUDE.md)                    │
│ The autonomous loop: pre-flight → pick feature → │
│ plan → branch → TDD → merge → repeat            │
├─────────────────────────────────────────────────┤
│ Layer 1: Container (Docker)                      │
│ The sandbox: Ubuntu + Node + Claude Code CLI +   │
│ your toolchain. Nothing escapes.                 │
└─────────────────────────────────────────────────┘
```

## Layer 1: The Container

The Docker container provides:

- **Isolation**: Claude Code runs with `--dangerously-skip-permissions`, meaning it can execute any command without asking. The container boundary is what makes this safe — the agent can only affect files inside `/workspace`.
- **Reproducibility**: Every run starts from the same base image. No "works on my machine" problems.
- **Toolchain**: Your project's language runtime, test framework, and build tools are installed in the image.

### Mount Topology

```
Host                              Container
────                              ─────────
~/my-project/          →          /workspace/
~/.claude/             →          /home/agent/.claude/
~/.claude.json         →          /home/agent/.claude.json
~/.ssh/ (optional)     →          /home/agent/.ssh/ (read-only)
```

The project directory is mounted read-write so the agent's changes (code, commits, plans) persist on the host.

The `.claude` directory and `.claude.json` are mounted so the agent inherits your Claude Code authentication. These are the session credentials from `claude login`.

## Layer 2: The Workflow

`CLAUDE.md` defines a 6-step loop that the agent follows:

1. **Pre-flight**: Read config, check branch, sync with remote
2. **Pick**: Find next `[ ]` item in the backlog
3. **Plan**: Write a step-by-step implementation plan
4. **Branch**: Create a git worktree on a feature branch
5. **Implement**: TDD loop — tests first, implement, run all tests, commit
6. **Merge**: Merge feature branch, cleanup worktree, push, mark done

### Why Worktrees?

Git worktrees let the agent work on a feature branch without switching the main branch. This means:

- If the agent crashes mid-feature, the default branch is untouched
- Multiple agent sessions can't step on each other
- Cleanup is simple: `git worktree remove` and `git branch -d`

### Why TDD?

Test-driven development gives the agent a feedback loop. Without tests, the agent can only verify code by reading it. With tests, it gets a binary pass/fail signal after every change. This catches regressions early and keeps the codebase stable across many autonomous changes.

TDD is optional — set `testing.enabled: false` in `agent.yaml` if your project doesn't have a test framework.

## Layer 3: The Configuration

`agent.yaml` is the single file that adapts the generic workflow to your project. The agent reads it at Step 0 and uses it throughout.

Key design choice: the agent (Claude, the LLM) reads the YAML file contents directly using its file-reading tools. There is no YAML parser or preprocessor — Claude understands YAML natively and extracts the values it needs. This means you can add comments, notes, or even free-form text in the YAML and the agent will understand it.

## Data Flow

```
scripts/agent.sh
  │
  ├── docker compose up -d agent         (start container if needed)
  │
  └── docker compose exec agent claude   (launch Claude Code)
        │
        ├── Reads CLAUDE.md              (the workflow)
        ├── Reads agent.yaml             (the config)
        ├── Reads TODO.md                (the backlog)
        │
        └── Enters autonomous loop:
              │
              ├── git worktree add ...   (create feature branch)
              ├── Write tests            (if testing enabled)
              ├── Write code
              ├── Run tests              (if testing enabled)
              ├── git commit
              ├── ... (repeat per plan step)
              ├── git merge
              ├── git push
              └── Update TODO.md
```

## Agent Types

The framework supports specialized agent types, each with their own instruction
set. Instead of one monolithic `CLAUDE.md`, instructions are split into:

```
agents/
  _base/CLAUDE.md       ← shared by all agents (pre-flight, git, standards)
  manager/CLAUDE.md     ← orchestration workflow
  coder/CLAUDE.md       ← TDD implementation workflow
  designer/CLAUDE.md    ← UI/UX and Figma integration
  automation/CLAUDE.md  ← CI/CD and infrastructure
  qa/CLAUDE.md          ← testing and quality assurance
```

When launched with `scripts/agent.sh -t <type>`, the launcher concatenates the
base and type-specific instructions into a single assembled `CLAUDE.md` and
bind-mounts it into the container. Each container sees only its own role's
instructions.

### Coordination Model

Agents coordinate through **task files** in the `tasks/` directory:

```
Manager reads backlog
  │
  ├── Creates tasks/add-auth.md        (Type: coder, Status: pending)
  ├── Creates tasks/auth-tests.md      (Type: qa, Status: pending)
  └── Creates tasks/auth-ci.md         (Type: automation, Status: pending)

Workers pick up their tasks
  │
  ├── Coder reads tasks/add-auth.md    → creates worktree → implements → done
  ├── QA reads tasks/auth-tests.md     → creates worktree → writes tests → done
  └── Automation reads tasks/auth-ci.md → creates worktree → adds pipeline → done

Manager reviews and merges completed branches
```

Each worker agent runs in its own Docker container and git worktree, providing
full isolation. The task file format is the shared protocol — no direct
inter-agent communication is needed.

See [Agent Types](agent-types.md) for the full reference.

## Context Window

Claude Code has a finite context window. The autonomous loop is designed to survive context exhaustion:

- Each plan step produces an independent commit
- If the agent runs out of context mid-feature, the work done so far is committed
- The plan file records which steps are complete
- The next agent session can resume from where the previous one stopped

This is why the plan step says "each step should be small, testable, and independently committable" — it's not just good engineering practice, it's a survival mechanism for long-running agent sessions.
