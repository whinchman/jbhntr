# Configuration Reference

All project-specific configuration lives in `agent.yaml` at the project root. The agent reads this file during Step 0 (pre-flight checks) and uses it throughout the autonomous workflow.

## Full Schema

### `project`

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `name` | string | `"my-project"` | Human-readable project name. Used in commit messages (`feat(<name>): ...`) and plan filenames. |
| `description` | string | `""` | One-line description of the project. Gives the agent context about what it is building. |

### `git`

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `default_branch` | string | `"main"` | The branch the agent merges features into and pushes to remote. Common values: `main`, `develop`, `development`. |
| `feature_prefix` | string | `"feature/"` | Prefix for feature branch names. The agent creates branches like `<prefix><feature-name>`. |
| `commit_style` | string | `"conventional"` | How commit messages are formatted. `conventional` = `feat(<scope>): <message>`. `freeform` = agent decides. |

### `workflow`

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `plans_dir` | string | `"plans"` | Directory where implementation plans are written, one markdown file per feature. |
| `worktrees_dir` | string | `"worktrees"` | Directory where git worktrees are created. Should be in `.gitignore`. |
| `backlog_file` | string | `"TODO.md"` | The file the agent reads to find the next feature. Uses checkbox format: `[ ]` pending, `[x]` done. |
| `done_file` | string | `"DONE.md"` | Archive file for completed work. |

### `testing`

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | boolean | `true` | Whether to use test-driven development. When `false`, the agent skips writing tests and running the test suite. |
| `command` | string | `"npm test"` | Shell command to run the full test suite. Must exit `0` on success, non-zero on failure. |
| `test_dir` | string | `"tests/"` | Directory where test files live. The agent creates new test files here. |
| `test_pattern` | string | `"test_*"` | Naming convention for test files. Examples: `test_*.py`, `*.test.ts`, `*_test.go`. |

### `build`

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `command` | string | `""` | Optional build/compile command. Run after all tests pass during Step 5 (feature complete). Leave empty to skip. |

### `code_standards`

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| (multiline string) | string | `""` | Project-specific coding rules the agent must follow. Write as bullet points. The agent reads these during Step 0 and applies them to every file it creates or modifies. |

## Examples

### Python (pytest)

```yaml
project:
  name: "my-api"
  description: "FastAPI REST API"

git:
  default_branch: "main"

testing:
  enabled: true
  command: "pytest -v --tb=short"
  test_dir: "tests/"
  test_pattern: "test_*.py"

code_standards: |
  - Follow PEP 8
  - Type hints on all function signatures
  - Source in src/, tests in tests/
```

### Node.js (Jest)

```yaml
project:
  name: "my-frontend"
  description: "React + TypeScript SPA"

testing:
  command: "npm test -- --watchAll=false"
  test_dir: "src/__tests__/"
  test_pattern: "*.test.tsx"

build:
  command: "npm run build"

code_standards: |
  - TypeScript strict mode
  - Functional components with hooks
  - CSS Modules for styling
```

### Rust (cargo)

```yaml
project:
  name: "my-cli"
  description: "Command-line tool in Rust"

testing:
  command: "cargo test"
  test_dir: "tests/"
  test_pattern: "*_test.rs"

build:
  command: "cargo build --release"

code_standards: |
  - cargo fmt before commit
  - cargo clippy with zero warnings
  - thiserror for error types
```

### Godot (GUT)

```yaml
project:
  name: "my-game"
  description: "A 2D platformer in Godot 4"

git:
  default_branch: "development"

testing:
  command: "godot --headless -d -s res://addons/gut/gut_cmdln.gd -gdir=res://tests -ginclude_subdirs -gexit --path /workspace/game"
  test_dir: "game/tests/unit/"
  test_pattern: "test_*.gd"

code_standards: |
  - All code under game/
  - Use signals and events bus
  - Commit .gd.uid files
```

### No tests

```yaml
testing:
  enabled: false
```

When testing is disabled, the agent skips Steps 4.1 (write tests) and 4.3 (run tests), and the final test run in Step 5. The workflow becomes: plan, branch, implement, commit, merge.
