# Task: resume-export-5-qa

- **Type**: qa
- **Status**: pending
- **Repo**: .
- **Parallel Group**: 4
- **Branch**: feature/resume-export-3-routes
- **Source Item**: resume-export (multi-format export: Markdown, DOCX, PDF)
- **Dependencies**: resume-export-1-foundation, resume-export-2-exporter, resume-export-3-routes

## Description

Write and run tests for the resume-export feature. The coder tasks (1–3) already include some tests; this task adds missing coverage and validates the full test suite.

Focus areas:
1. `internal/exporter/docx_test.go` — verify full coverage of the supported Markdown subset
2. `internal/store/store_test.go` — verify `UpdateJobGenerated` persists `resume_markdown` and `cover_markdown` correctly
3. `internal/generator/worker_test.go` — add tests for nil-converter path (no PDF generated, job still completes)
4. `internal/generator/generator_test.go` — add tests for the new four-section parser (happy path, missing separator, partial response)
5. `internal/web/server_test.go` — add tests for the four new download handlers (content present, content absent → 404, DOCX content-type header)

## Acceptance Criteria

- [ ] `exporter.ToDocx` unit tests cover: empty string, H1/H2/H3 headings, bold run, italic run, unordered list item, plain paragraph, mixed content (heading + list + paragraph)
- [ ] Store test verifies `resume_markdown` and `cover_markdown` are read back correctly after `UpdateJobGenerated`
- [ ] Worker test verifies that when `converter == nil`, the job is still marked complete with empty PDF paths (not failed)
- [ ] Generator test verifies the four-section parse: happy path returns all four strings trimmed; missing any separator returns an error
- [ ] Web handler tests verify: 200 + correct Content-Type for MD download when content non-empty; 200 + DOCX content-type when content non-empty; 404 when content is empty
- [ ] `go test ./...` passes with no failures

## Interface Contracts

### Functions under test

```go
// internal/exporter/docx.go
func ToDocx(md string) ([]byte, error)

// internal/generator/generator.go
func (g *AnthropicGenerator) Generate(ctx, job, baseResume) (resumeMD, resumeHTML, coverMD, coverHTML string, err error)

// internal/generator/worker.go — nil-converter behaviour
// When w.converter == nil: processJob must complete successfully with empty resumePDF and coverPDF

// internal/store/store.go
func (s *Store) UpdateJobGenerated(ctx, userID, id int64, resumeHTML, coverHTML, resumeMarkdown, coverMarkdown, resumePDF, coverPDF string) error

// internal/web/server.go — new handlers
// GET /output/{id}/resume.md        → 200 text/markdown or 404
// GET /output/{id}/cover_letter.md  → 200 text/markdown or 404
// GET /output/{id}/resume.docx      → 200 application/vnd.openxmlformats... or 404
// GET /output/{id}/cover_letter.docx → 200 application/vnd.openxmlformats... or 404
```

## Context

- Plan: `.workflow/plans/resume-export.md` — section 6 (acceptance criteria) defines the full expected behaviour
- Existing tests use `database/sql` with a real PostgreSQL connection in `internal/store/store_test.go`; follow the same pattern for new store tests
- Worker tests in `internal/generator/worker_test.go` likely use a mock `WorkerStore`; add a nil-converter test case using the existing test infrastructure
- Web server tests in `internal/web/server_test.go` use `httptest`; add test cases for the four new routes

## Notes

<!-- QA agent fills in coverage summary and results here -->
