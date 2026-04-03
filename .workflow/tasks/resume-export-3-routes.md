# Task: resume-export-3-routes

- **Type**: code-reviewer
- **Status**: done
- **Repo**: .
- **Parallel Group**: 3
- **Branch**: feature/resume-export-3-routes
- **Source Item**: resume-export (multi-format export: Markdown, DOCX, PDF)
- **Dependencies**: resume-export-1-foundation, resume-export-2-exporter

## Description

Add four new download routes to the web server and update the job detail UI template to show download buttons for all available formats (Markdown, DOCX, PDF). Depends on the `Job` struct having `ResumeMarkdown`/`CoverMarkdown` fields (from foundation task) and the `exporter.ToDocx` function (from exporter task).

Files to modify:
- `internal/web/server.go` — register 4 new routes, implement 4 new handlers
- `internal/web/templates/job_detail.html` — replace single PDF button with per-format download groups

## Acceptance Criteria

- [ ] `GET /output/{id}/resume.md` returns `Content-Type: text/markdown`, `Content-Disposition: attachment; filename=resume.md`, and the job's `ResumeMarkdown` text; returns 404 if `ResumeMarkdown == ""`
- [ ] `GET /output/{id}/cover_letter.md` same pattern for `CoverMarkdown`
- [ ] `GET /output/{id}/resume.docx` calls `exporter.ToDocx(job.ResumeMarkdown)`, returns `Content-Type: application/vnd.openxmlformats-officedocument.wordprocessingml.document`, `Content-Disposition: attachment; filename=resume.docx`; returns 404 if `ResumeMarkdown == ""`; returns 500 on ToDocx error
- [ ] `GET /output/{id}/cover_letter.docx` same pattern for `CoverMarkdown`
- [ ] Existing `GET /output/{id}/resume.pdf` and `GET /output/{id}/cover_letter.pdf` endpoints are unchanged and still pass their existing tests
- [ ] `job_detail.html`: resume section shows `[Download Markdown]` and `[Download DOCX]` buttons when `Job.ResumeMarkdown != ""`; shows `[Download PDF]` button only when `Job.ResumePDF != ""`
- [ ] `job_detail.html`: cover letter section follows the same pattern with `Job.CoverMarkdown` and `Job.CoverPDF`
- [ ] All existing web tests pass (`go test ./internal/web/...`)

## Interface Contracts

### New handlers to add in internal/web/server.go
```go
func (s *Server) handleDownloadResumeMarkdown(w http.ResponseWriter, r *http.Request)
func (s *Server) handleDownloadCoverMarkdown(w http.ResponseWriter, r *http.Request)
func (s *Server) handleDownloadResumeDocx(w http.ResponseWriter, r *http.Request)
func (s *Server) handleDownloadCoverDocx(w http.ResponseWriter, r *http.Request)
```

### Routes to register (in the Handler() route block, alongside existing PDF routes)
```go
r.Get("/output/{id}/resume.md",         s.handleDownloadResumeMarkdown)
r.Get("/output/{id}/cover_letter.md",   s.handleDownloadCoverMarkdown)
r.Get("/output/{id}/resume.docx",       s.handleDownloadResumeDocx)
r.Get("/output/{id}/cover_letter.docx", s.handleDownloadCoverDocx)
```

### exporter package import
```go
import "github.com/whinchman/jobhuntr/internal/exporter"

// Usage in DOCX handler:
docxBytes, err := exporter.ToDocx(job.ResumeMarkdown)
```

### Template data structure (internal/web/server.go — jobDetailData)
The `jobDetailData` struct is already defined; it embeds `*models.Job` via the `Job` field, which after the foundation task will have `ResumeMarkdown`, `CoverMarkdown`, `ResumePDF`, and `CoverPDF` fields. The template can reference these directly.

### Template download group pattern (job_detail.html)
```html
{{if .Job.ResumeMarkdown}}
<p>
  <a href="/output/{{.Job.ID}}/resume.md" download="resume.md" role="button" class="outline secondary">Download Markdown</a>
  <a href="/output/{{.Job.ID}}/resume.docx" download="resume.docx" role="button" class="outline secondary">Download DOCX</a>
  {{if .Job.ResumePDF}}
  <a href="/output/{{.Job.ID}}/resume.pdf" download="resume.pdf" role="button" class="outline">Download PDF</a>
  {{end}}
</p>
{{end}}
```
Apply the same pattern for the cover letter section using `CoverMarkdown` and `CoverPDF`.

### Job struct fields available after foundation task
```go
// models.Job — fields available for template and handlers
ResumeMarkdown string  // non-empty when generation succeeded
CoverMarkdown  string  // non-empty when generation succeeded
ResumePDF      string  // non-empty only if PDF conversion succeeded
CoverPDF       string  // non-empty only if PDF conversion succeeded
```

## Context

- Plan: `.workflow/plans/resume-export.md` — see sections 3.5, 3.6, 5 (Steps 7–8)
- `internal/web/server.go`: the existing PDF download handlers (`handleDownloadResume`, `handleDownloadCover`) use `chi.URLParam(r, "id")` to extract the job ID, then call `s.store.GetJob(...)`. Follow the same pattern for the new handlers.
- Auth context: look at how existing handlers retrieve the logged-in user (likely via `r.Context()` or a session helper) and apply the same user scoping when calling `s.store.GetJob(ctx, userID, id)`.
- The existing `JobStore` interface in server.go only needs `GetJob` and `ListJobs` — no change to the interface is required for these handlers.
- Return 404 (not 500) when content is empty string — this signals the document hasn't been generated yet, which is a normal state.

## Notes

### Implementation Summary

**Branch**: `feature/resume-export-3-routes` (worktree at `/workspace/worktrees/resume-export-3-routes`)
**Based on**: `feature/resume-export-2-exporter`
**Commit**: `5572059`

**Changes made**:

1. `internal/web/server.go`:
   - Added import for `github.com/whinchman/jobhuntr/internal/exporter`
   - Registered 4 new routes in `Handler()`: `/output/{id}/resume.md`, `/output/{id}/cover_letter.md`, `/output/{id}/resume.docx`, `/output/{id}/cover_letter.docx`
   - Implemented `handleDownloadResumeMarkdown` — returns `text/markdown` with `Content-Disposition: attachment; filename=resume.md`; 404 if `ResumeMarkdown == ""`
   - Implemented `handleDownloadCoverMarkdown` — same pattern for CoverMarkdown
   - Implemented `handleDownloadResumeDocx` — calls `exporter.ToDocx(job.ResumeMarkdown)`, returns DOCX content type; 404 if empty, 500 on ToDocx error
   - Implemented `handleDownloadCoverDocx` — same pattern for CoverMarkdown

2. `internal/web/templates/job_detail.html`:
   - Resume section: replaced single PDF button with conditional multi-format group (MD + DOCX shown when `ResumeMarkdown != ""`; PDF only when `ResumePDF != ""`)
   - Cover letter section: same pattern with `CoverMarkdown` and `CoverPDF`

3. `internal/web/server_test.go`:
   - Added `newJobWithMarkdown()` helper
   - Added `TestDownloadResumeMarkdown` (3 subtests: success, empty markdown → 404, unknown job → 404)
   - Added `TestDownloadCoverMarkdown` (3 subtests)
   - Added `TestDownloadResumeDocx` (3 subtests: success verifies PK zip magic bytes, empty → 404, unknown → 404)
   - Added `TestDownloadCoverDocx` (3 subtests)

**Test results**: All 12 new tests pass. Pre-existing auth test failures (TestRequireAuth_Unauthenticated, TestRequireAuth_DeletedUser, TestIntegration_*) are present on the upstream branch and unrelated to this task.

### Code Review Findings

**Verdict: approve** — 0 critical, 1 warning, 2 info findings

#### [WARNING] internal/web/server_test.go lines 1225, 1287 — potential panic in error message format

In `TestDownloadResumeDocx` and `TestDownloadCoverDocx`, the check guards `len(body) < 2` but the `t.Errorf` format string accesses `body[:4]`. If `len(body)` is 0, 1, 2, or 3 (e.g. because the server returned an error response with a short body), the slice expression panics at test time instead of reporting a clean test failure. The check and the error message are inconsistent.

Fix: change the guard to `len(body) < 4` or change the error format to `body` (full slice) and guard `len(body) < 2` separately from the format verb.

Logged as BUG-013.

#### [INFO] No test for exporter.ToDocx 500 error path

The acceptance criterion "returns 500 on ToDocx error" is not covered by the test suite. The mock store cannot inject a `ToDocx` failure because the exporter is called directly (not via an interface). This is a known constraint of the current architecture (exporter is a pure function, not injectable). Coverage of this path would require either wrapping `exporter.ToDocx` behind an interface or using a build-tag approach.

Not blocking — the happy-path and 404 paths are well tested and the 500 handler is a straightforward 2-line block. Noting for future improvement.

#### [INFO] No user-isolation tests for new download endpoints

The existing PDF download handlers (`TestDownloadResumePDF`, `TestDownloadCoverPDF`) also lack user-isolation test cases, so this is consistent with the existing pattern. However, now that four additional authenticated download routes exist, a future test pass should cover the scenario where user A tries to download a job belonging to user B (expects 404).

Not blocking — the implementation correctly passes `userID` from context to `GetJob`, which enforces user scoping at the store layer. The pattern is verified by other endpoint isolation tests.
