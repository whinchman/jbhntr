# Task: local-debug-2-review

- **Type**: code-reviewer
- **Status**: pending
- **Repo**: . (single repo — /workspace)
- **Parallel Group**: 2
- **Branch**: feature/local-debug-1-infra
- **Source Item**: local-debug (plan: .workflow/plans/local-debug.md)
- **Dependencies**: local-debug-1-infra

## Description

Review the changes introduced by task `local-debug-1-infra` on branch
`feature/local-debug-1-infra`. Verify correctness, security posture, and
adherence to the plan.

Focus areas:
1. `.env.example` — no real secrets committed; `DATABASE_URL` correctly commented out as optional for Docker users
2. `Dockerfile.dev` — correct base image (`golang:1.25-bookworm`), `air` installed, `chromium` installed, `WORKDIR /workspace` set, `CMD ["air"]`
3. `docker-compose.yml` — `dev` service is profile-gated (`profiles: [dev]`); existing `app` and `db` services are unchanged; `go-mod-cache` volume defined; `DATABASE_URL` points to `db` host (not `localhost`)
4. `Makefile` — tab-indented (not spaces); all declared targets are in `.PHONY`; `make test` runs `go test ./...`; `make build` targets `./cmd/jobhuntr`
5. `.air.toml` — `include_dir` covers `cmd` and `internal`; `tmp_dir = "tmp"`; sensitive dirs excluded
6. `run.sh` — `.env` sourcing still intact; `DATABASE_URL` and `SESSION_SECRET` added to missing-vars check; existing optional-var checks preserved
7. `.gitignore` — `tmp/` present; `.env.example` NOT in gitignore
8. `agent.yaml` — `testing.command` is `go test ./...`

Write any findings to the Notes section of this task file. Add any critical or
warning severity findings to `/workspace/.workflow/BUGS.md`.

Return verdict: **approve** or **request-changes**.

## Acceptance Criteria

- [ ] All 8 files are present and match the plan specification
- [ ] No secrets or credentials committed
- [ ] `docker-compose.yml` existing services unmodified
- [ ] `Makefile` uses tabs (not spaces) for recipe indentation
- [ ] Verdict recorded in Notes

## Interface Contracts

None.

## Context

Plan: `/workspace/.workflow/plans/local-debug.md`
Implemented on branch: `feature/local-debug-1-infra`

## Notes

<!-- Reviewer fills this in -->
