# Task: local-debug-3-qa

- **Type**: qa
- **Status**: pending
- **Repo**: . (single repo — /workspace)
- **Parallel Group**: 3
- **Branch**: feature/local-debug-1-infra
- **Source Item**: local-debug (plan: .workflow/plans/local-debug.md)
- **Dependencies**: local-debug-2-review

## Description

QA validation for the `local-debug` feature on branch `feature/local-debug-1-infra`.

The feature adds local dev tooling (no new application logic), so the test
focus is:

1. **`go test ./...` passes** — run the full test suite and confirm no regressions.
2. **`Makefile` target smoke-test** — verify `make test` invokes `go test ./...`
   correctly (inspect the Makefile; run `make test` directly if `make` is available).
3. **File existence checks** — assert all 8 files were created/modified:
   - `.env.example`
   - `.air.toml`
   - `Dockerfile.dev`
   - `Makefile`
   - `docker-compose.yml` (has `dev` service + `go-mod-cache` volume)
   - `run.sh` (has `DATABASE_URL` and `SESSION_SECRET` in warning block)
   - `.gitignore` (contains `tmp/`)
   - `agent.yaml` (`testing.command: go test ./...`)
4. **`.env.example` sanity** — confirm it contains `SESSION_SECRET`, `GITHUB_CLIENT_ID`, and the commented-out `DATABASE_URL` line; confirm `.env` is still in `.gitignore`; confirm `.env.example` is NOT in `.gitignore`.
5. **`docker-compose.yml` YAML validity** — run `docker compose config --quiet` (if Docker is available) or parse the YAML to confirm it is valid and the `dev` service is profile-gated.

If any check fails, log bugs to `/workspace/.workflow/BUGS.md` and set this task
status to `failed` with a description in Notes.

## Acceptance Criteria

- [ ] `go test ./...` exits 0 (no test regressions)
- [ ] All 8 target files exist and contain the expected content
- [ ] `.env.example` is committed and not gitignored
- [ ] `tmp/` is in `.gitignore`
- [ ] `docker-compose.yml` YAML is valid (no parse errors)
- [ ] `agent.yaml` `testing.command` is `go test ./...`
- [ ] No new bugs introduced (or bugs logged if found)

## Interface Contracts

None.

## Context

Plan: `/workspace/.workflow/plans/local-debug.md`
Implemented on branch: `feature/local-debug-1-infra`
Code review task: `local-debug-2-review`

The project is pure Go with no frontend build step. Tests live under the
project root and subdirectories. Run from `/workspace` worktree root.

## Notes

<!-- QA agent fills this in -->
