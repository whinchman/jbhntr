# Task: resume-export-3-routes

- **Type**: coder
- **Status**: pending
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

<!-- Implementing agent fills in when complete -->
