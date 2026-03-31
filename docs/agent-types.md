# Agent Types

The agent type system lets you run specialized agents, each with their own
instruction set and role. Instead of one agent that does everything, you can
launch focused agents that excel at specific kinds of work.

## How It Works

Each agent type is a directory under `agents/` containing a `CLAUDE.md` file
with role-specific workflow instructions. When you launch an agent with
`--type`, the launcher:

1. Reads `agents/_base/CLAUDE.md` (shared instructions for all agents)
2. Reads `agents/<type>/CLAUDE.md` (type-specific workflow)
3. Concatenates them into a single assembled `CLAUDE.md`
4. Bind-mounts the assembled file into the Docker container at `/workspace/CLAUDE.md`

The agent sees a single `CLAUDE.md` with both the shared base and its
type-specific instructions.

## Available Types

### Manager

Orchestrates work across other agents. The Manager reads the project backlog,
breaks features down into discrete tasks, assigns each task to the appropriate
agent type, reviews completed work, and merges it back to the default branch.

The Manager **never writes application code**. Its commits are limited to task
files, backlog updates, and merge commits.

```
scripts/agent.sh -t manager
```

### Architect

Researches technologies, analyzes requirements, and produces detailed
implementation plans for approval. The Architect reads the codebase, evaluates
trade-offs between approaches, and writes step-by-step plans specific enough
for a Coder agent to execute without ambiguity.

The Architect **never writes application code**. It produces design documents,
research findings, and implementation plans.

```
scripts/agent.sh -t architect
```

### Coder

The default agent type. Implements features and fixes using test-driven
development. Picks up tasks (or reads the backlog directly), creates a git
worktree, writes tests first, implements code, and commits.

```
scripts/agent.sh -t coder
scripts/agent.sh              # same thing (coder is the default)
```

### Designer

Handles UI/UX implementation: design tokens, component styling, semantic HTML,
accessibility, and responsive layouts. Integrates with Figma via MCP tools to
read designs and generate code from Figma files.

The Designer **does not write business logic**. It focuses on presentation,
design systems, and the visual layer.

```
scripts/agent.sh -t designer
```

### Automation Engineer

Builds and maintains CI/CD pipelines, Dockerfiles, build scripts, GitHub
Actions, deployment configurations, and monitoring setup.

The Automation Engineer **never modifies application logic**. It works
exclusively on infrastructure and build tooling.

```
scripts/agent.sh -t automation
```

### Code Reviewer

Reviews completed code for bugs, security vulnerabilities, logic errors, and
correctness issues. Examines every changed line in a branch, documents findings
by severity (critical/warning/info), and creates follow-up fix tasks for any
bugs discovered.

The Code Reviewer **never modifies application code**. It produces review
reports and follow-up task files.

```
scripts/agent.sh -t code-reviewer
```

### QA (Quality Assurance)

Writes comprehensive test suites, reviews code for quality issues, creates
integration and end-to-end tests, and analyzes test coverage.

The QA agent **never modifies application code**. It writes only test code,
test fixtures, and quality reports.

```
scripts/agent.sh -t qa
```

## Task Files

Agents coordinate through task files in the `tasks/` directory. The Manager
creates task files; worker agents read and update them.

### Format

```markdown
# Task: add-user-auth

- **Type**: coder
- **Status**: pending
- **Branch**: feature/add-user-auth
- **Backlog Item**: Add user authentication with JWT
- **Dependencies**: none

## Description
Implement JWT-based authentication with login/logout endpoints.

## Acceptance Criteria
- [ ] POST /auth/login returns a JWT token
- [ ] POST /auth/logout invalidates the token
- [ ] Protected routes return 401 without a valid token

## Notes
```

### Status Flow

```
pending → in-progress → done → (merged by Manager)
                      → failed → (Manager reviews and reassigns)
```

## Creating Custom Agent Types

Add a directory under `agents/` with a `CLAUDE.md` file:

```
agents/
  security-auditor/
    CLAUDE.md
```

The `CLAUDE.md` should define:
1. The agent's role and constraints (what it does and does NOT do)
2. A step-by-step workflow (find task → audit → plan → implement → signal done)
3. Commit message conventions for its work

The type is automatically available via `--type`:

```
scripts/agent.sh -t security-auditor
```

Use `--list-types` to see all available types:

```
scripts/agent.sh --list-types
```

## Backwards Compatibility

If the `agents/` directory does not exist, `agent.sh` falls back to the legacy
single-agent behavior using the root `CLAUDE.md`. Existing projects that do not
use the type system are unaffected.
