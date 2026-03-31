# Base Agent Instructions

These instructions apply to all agent types. Your type-specific workflow follows
below this section.

## Pre-flight Checks

Before doing any work:

A) **Read `agent.yaml`** and internalize the project configuration. Every path,
   command, and convention referenced below comes from that file.

B) Ensure you are on the correct branch for your role:
   - **Manager/Architect agents**: work on the default branch (`git.default_branch`)
   - **Worker agents** (coder, code-reviewer, designer, automation, qa): work
     in a dedicated git worktree on a feature branch

C) Ensure the working directory is clean. If there are uncommitted changes,
   stash them and warn.

D) Sync with remote:
   - If behind remote: `git pull`
   - If ahead of remote: `git push`
   - If diverged: `git pull --rebase` then `git push`

## Workflow Pipeline

Work flows through four files in the `workflow/` directory (paths configured
in `agent.yaml` under `workflow.*_file`):

```
BACKLOG.md → UNREADY.md → TODO.md → DONE.md
```

| File | Contains | Who writes | Who reads |
|------|----------|-----------|-----------|
| `BACKLOG.md` | Raw unprocessed features | Stakeholder | Manager, Architect |
| `UNREADY.md` | Researched/planned, awaiting approval | Manager, Architect | Stakeholder |
| `TODO.md` | Approved work, ready for agents | Stakeholder | Worker agents |
| `DONE.md` | Completed and merged | Manager | Stakeholder |
| `BUGS.md` | Bugs found by QA / Code Reviewer | QA, Code Reviewer | Stakeholder, Manager |

**Critical rule**: when moving an item from one file to the next, **remove it
from the source file**. Nothing should exist in two files at once. This
prevents duplicate work.

### Bugs

QA and Code Reviewer agents write bugs to `workflow.bugs_file`
(`workflow/BUGS.md`). Each bug entry should include the file, line, severity,
and reproduction steps. The stakeholder reviews bugs and moves approved fixes
to `TODO.md` (removing them from `BUGS.md`), just like the UNREADY→TODO gate.

### Pipeline stages

1. **BACKLOG → UNREADY**: The Manager reads `BACKLOG.md`, hands items to the
   Architect for research and planning. Once an item has acceptance criteria
   and a plan, the Manager moves it to `UNREADY.md` (removing it from
   `BACKLOG.md`) and commits.

2. **UNREADY → TODO**: The stakeholder (human) reviews items in `UNREADY.md`.
   Approved items are moved to `TODO.md`. This is the only manual gate.

3. **TODO → DONE**: Worker agents pick up `[ ]` items from `TODO.md`,
   implement them, and signal done. The Manager merges the work, moves the
   item to `DONE.md` (removing it from `TODO.md`), and commits.

## Task File Format

Tasks are the coordination protocol between agents. They live in the `tasks/`
directory (or the path specified by `agents.tasks_dir` in `agent.yaml`).

A task file (`tasks/<task-id>.md`) has this structure:

```markdown
# Task: <task-id>

- **Type**: architect | coder | code-reviewer | designer | automation | qa
- **Status**: pending | in-progress | done | failed
- **Branch**: feature/<task-id>
- **Source Item**: <reference to the workflow item>
- **Dependencies**: <task-ids that must complete first>

## Description
<What needs to be done>

## Acceptance Criteria
- [ ] Criterion 1

## Notes
<Feedback from manager, worker notes>
```

**Status transitions**:
- `pending` → `in-progress`: worker picks up the task
- `in-progress` → `done`: worker completed the task successfully
- `in-progress` → `failed`: worker could not complete the task (add reason to Notes)
- `done` → merged by manager (task file archived or deleted)

## Context Window Management

If you are running low on context mid-task:
1. Complete the current step
2. Commit your work
3. Note in the plan file or task file which step you stopped at
4. The next agent session will resume from that point

## Code Standards

Follow the rules in the `code_standards` section of `agent.yaml`. Read them
during pre-flight and apply them to every file you create or modify.

Display terminal commands on a single uninterrupted line (no backslash line
continuations).
