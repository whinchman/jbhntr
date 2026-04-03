# Task: resume-export-4-review

- **Type**: code-reviewer
- **Status**: pending
- **Repo**: .
- **Parallel Group**: 4
- **Branch**: feature/resume-export-3-routes
- **Source Item**: resume-export (multi-format export: Markdown, DOCX, PDF)
- **Dependencies**: resume-export-1-foundation, resume-export-2-exporter, resume-export-3-routes

## Description

Review all code changes across the three implementation tasks for the resume-export feature. The final merged state is on the branch produced by resume-export-3-routes (which was rebased on the earlier tasks). Check for correctness, security, and code standard compliance.

Key areas to review:
1. `internal/store/migrations/008_add_markdown_columns.sql` — correctness and idempotency
2. `internal/models/models.go` — new fields placed correctly
3. `internal/store/store.go` — `UpdateJobGenerated` SQL correctness, `scanJob` column order matches SELECT
4. `internal/generator/prompts.go` — prompt clarity, separator constants
5. `internal/generator/generator.go` — four-section parse correctness, error paths
6. `internal/generator/worker.go` — nil-safe PDF path, error handling non-fatal
7. `cmd/jobhuntr/main.go` — non-fatal PDF init pattern
8. `internal/exporter/docx.go` — Markdown parsing correctness, panic safety for unknown elements
9. `internal/web/server.go` — new route registration, handler correctness, auth scoping, correct HTTP status codes
10. `internal/web/templates/job_detail.html` — conditional rendering logic

## Acceptance Criteria

- [ ] No critical security issues (e.g. path traversal in file serving, missing auth checks)
- [ ] `scanJob` column order exactly matches the SELECT column list
- [ ] PDF nil-check is present in worker.go before every call to `w.converter.*`
- [ ] All four new download handlers return 404 (not 200/500) when content is empty
- [ ] DOCX handler propagates `exporter.ToDocx` errors as HTTP 500
- [ ] Template conditional blocks are logically correct (MD and DOCX button gated on `ResumeMarkdown != ""`, PDF button gated on `ResumePDF != ""`)
- [ ] No panics possible in `internal/exporter/docx.go` for any valid string input
- [ ] Verdict written to task Notes section: `approve` or `request-changes`
- [ ] Any bugs of severity warning or above written to `.workflow/BUGS.md`

## Interface Contracts

No new contracts — this task reviews compliance with the contracts defined in tasks 1–3.

## Context

- Plan: `.workflow/plans/resume-export.md` — overall acceptance criteria in section 6
- Branches to review: all changes are on the branch produced by resume-export-3-routes (the coder for task 3 will have merged or rebased the earlier branches)
- Alternatively, review each feature branch individually: `feature/resume-export-1-foundation`, `feature/resume-export-2-exporter`, `feature/resume-export-3-routes`

## Notes

<!-- Code reviewer fills in verdict and findings here -->
