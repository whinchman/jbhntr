# Architecture Plan: Resume/Cover Letter Export Formats

**Date**: 2026-04-03
**Feature**: Multi-format export (Markdown, DOCX, PDF) for generated resumes and cover letters
**Status**: Draft

---

## 1. Current State

### What exists today

The generator pipeline works as follows:

1. **Background worker** (`internal/generator/worker.go`): polls for jobs with status `approved`, calls the Anthropic Claude API via `generator.Generate()`, receives back two raw HTML strings (resume + cover letter), converts both to PDF via headless Chromium (`internal/pdf/pdf.go`), then stores the file paths in the database and marks the job `complete`.

2. **Storage**: The `jobs` table holds:
   - `resume_html TEXT` — raw Claude-generated HTML
   - `cover_html TEXT` — raw Claude-generated HTML
   - `resume_pdf TEXT` — filesystem path to the PDF
   - `cover_pdf TEXT` — filesystem path to the PDF

3. **Download endpoints** (`internal/web/server.go`):
   - `GET /output/{id}/resume.pdf` → `handleDownloadResume` — serves the file at `job.ResumePDF`
   - `GET /output/{id}/cover_letter.pdf` → `handleDownloadCover` — serves the file at `job.CoverPDF`

4. **UI** (`internal/web/templates/job_detail.html`): shows two `<a>` download buttons, one per document, both hardcoded to `.pdf`.

5. **Claude prompt** (`internal/generator/prompts.go`): instructs Claude to output two self-contained HTML documents with inline CSS, separated by `---SEPARATOR---`. The HTML is designed for PDF conversion; it does not contain Markdown-friendly content.

### What is missing

- No Markdown download
- No DOCX download
- PDF generation is mandatory (if Chromium is unavailable or errors, the whole job fails)
- No way for users to choose which formats they want

---

## 2. Scope of Changes

The three backlog items are tightly coupled and best addressed together:

| Item | Summary |
|------|---------|
| Markdown export | Download resume/cover letter as `.md` |
| DOCX export | Download resume/cover letter as `.docx` |
| Optional PDF | PDF generation can fail gracefully or be disabled; job still succeeds |

---

## 3. Architecture Overview

### 3.1 Dual-output Claude prompt

Claude currently produces HTML designed for PDF rendering (inline CSS, `<html>` wrappers). To support Markdown download without re-calling the API, we need Claude to also produce Markdown.

**Decision: change the prompt to return Markdown source + HTML simultaneously.**

The new separator protocol:
```
---RESUME_MD---
[resume markdown]
---RESUME_HTML---
[resume html]
---COVER_MD---
[cover letter markdown]
---COVER_HTML---
[cover letter html]
```

This is a one-shot change. The Markdown content is the canonical human-readable form; the HTML is derived from it for PDF/DOCX conversion purposes.

Alternative considered: call Claude twice (once for MD, once for HTML). Rejected — doubles API cost and latency.

Alternative considered: convert HTML→Markdown server-side (e.g., html-to-markdown library). Viable fallback if the prompt approach turns out messy, but adds a dependency and the conversion quality for richly styled HTML is unpredictable.

### 3.2 DOCX generation approach

Rather than converting HTML→DOCX (which requires an external tool like LibreOffice or a heavyweight library), we generate DOCX **directly from Markdown** using a Go library to construct the OOXML structure.

**Recommended library: `github.com/gomutex/godocx`** (MIT license, pure Go, no CGO, actively maintained as of 2024–2025).

Trade-off analysis:

| Library | License | CGO | Actively maintained | Notes |
|---------|---------|-----|---------------------|-------|
| `gomutex/godocx` | MIT | No | Yes (2024–2025) | Clean API; read/write .docx; paragraph/run/style support |
| `fumiama/go-docx` | AGPL-3.0 | No | Yes | AGPL is incompatible with closed-source; ruled out |
| `unidoc/unioffice` | Commercial | No | Yes | Requires paid license; ruled out |
| `lukasjarosch/go-docx` | MIT | No | Limited | Template-replacement only, not programmatic generation |
| `mmonterroca/docxgo` | AGPL/MIT | No | New (Oct 2025) | Too new, limited production track record |

**Decision: `gomutex/godocx`** — MIT license, no CGO dependency, actively maintained, supports programmatic paragraph/run creation from parsed Markdown.

DOCX generation approach: parse the Markdown into a simple AST (headings, paragraphs, lists, bold/italic runs) and emit DOCX paragraphs/runs via godocx. We do not need to support every Markdown feature — resumes use a limited set: `#`/`##` headings, `**bold**`, `_italic_`, bullet lists (`-`/`*`), and plain paragraphs.

A minimal Markdown→DOCX converter will live in a new package `internal/exporter/`.

### 3.3 Making PDF optional

Currently `worker.go` fails the job if either PDF conversion fails. The change:

- PDF conversion becomes best-effort: if `pdf.Converter` is nil or the conversion returns an error, the worker logs a warning and continues rather than failing the job.
- `main.go` wraps `pdf.NewRodConverter()` in a recoverable error path: if Chromium is not available, the worker starts without a converter (converter = nil) and PDF download links simply do not appear in the UI.
- The `Job.ResumePDF` / `Job.CoverPDF` fields remain empty strings when PDF is not generated; the UI already checks `job.ResumePDF != ""` before rendering the PDF download link (the download handler returns 404 if the path is empty — this is the correct behaviour).

### 3.4 New database columns

Two new TEXT columns are added to the `jobs` table via a migration:

```sql
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS resume_markdown TEXT NOT NULL DEFAULT '';
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS cover_markdown  TEXT NOT NULL DEFAULT '';
```

DOCX files are generated on-demand (not stored on disk) to avoid bloating storage. The Markdown content is stored in the database alongside the HTML.

Alternative considered: generate DOCX on disk like PDFs. Rejected — DOCX generation is fast (pure Go, no browser), so on-demand generation is simpler and avoids managing additional output files.

### 3.5 New download endpoints

```
GET /output/{id}/resume.md            → handleDownloadResumeMarkdown
GET /output/{id}/cover_letter.md      → handleDownloadCoverMarkdown
GET /output/{id}/resume.docx          → handleDownloadResumeDocx
GET /output/{id}/cover_letter.docx    → handleDownloadCoverDocx
```

Existing PDF endpoints are unchanged:
```
GET /output/{id}/resume.pdf           → handleDownloadResume       (unchanged)
GET /output/{id}/cover_letter.pdf     → handleDownloadCover        (unchanged)
```

All endpoints return 404 if the document has not been generated yet (empty string in DB).

### 3.6 UI changes

The job detail template gains a download button group per document. Buttons are only rendered when the relevant content exists.

```
Generated Resume
[preview panel]
Downloads: [MD] [DOCX] [PDF]   ← shown only when respective content non-empty

Cover Letter
[preview panel]
Downloads: [MD] [DOCX] [PDF]
```

---

## 4. Affected Files

| File | Change |
|------|--------|
| `internal/generator/prompts.go` | New separator constants and updated `systemPrompt` to return MD + HTML |
| `internal/generator/generator.go` | Parse new four-section response; return `resumeMD`, `resumeHTML`, `coverMD`, `coverHTML` |
| `internal/generator/worker.go` | Pass MD strings to store; make PDF conversion optional (nil-safe converter) |
| `internal/models/models.go` | Add `ResumeMarkdown`, `CoverMarkdown` fields to `Job` struct |
| `internal/store/store.go` | Update `UpdateJobGenerated`, `scanJob`, schema constant; update `GetJob`/`ListJobs` SELECT lists |
| `internal/store/migrations/008_add_markdown_columns.sql` | New migration: `ALTER TABLE jobs ADD COLUMN resume_markdown / cover_markdown` |
| `internal/web/server.go` | Add 4 new download handlers; register routes; `WorkerStore.UpdateJobGenerated` signature update |
| `internal/web/templates/job_detail.html` | Replace single PDF buttons with three-button download groups |
| `internal/exporter/` | **New package**: Markdown→DOCX converter |
| `internal/exporter/docx.go` | `func ToDocx(md string) ([]byte, error)` |
| `internal/exporter/docx_test.go` | Unit tests for converter |
| `go.mod` / `go.sum` | Add `github.com/gomutex/godocx` |
| `cmd/jobhuntr/main.go` | Make PDF converter optional (non-fatal if Chromium unavailable) |

---

## 5. Step-by-Step Implementation Plan

### Step 1: Add database migration (no code changes yet)

Create `/workspace/internal/store/migrations/008_add_markdown_columns.sql`:
```sql
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS resume_markdown TEXT NOT NULL DEFAULT '';
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS cover_markdown  TEXT NOT NULL DEFAULT '';
```

### Step 2: Update models

File: `internal/models/models.go`

Add to `Job` struct after `CoverHTML`:
```go
ResumeMarkdown string
CoverMarkdown  string
```

### Step 3: Update store

File: `internal/store/store.go`

3a. Update `scanJob` to scan `resume_markdown` and `cover_markdown`:
```go
// In the Scan call, add after &job.CoverHTML:
&job.ResumeMarkdown, &job.CoverMarkdown,
```

3b. Update all SELECT queries (`GetJob`, `ListJobs`) to include `resume_markdown, cover_markdown` in the column list.

3c. Update `UpdateJobGenerated` signature:
```go
func (s *Store) UpdateJobGenerated(ctx context.Context, userID int64, id int64, resumeHTML, coverHTML, resumeMarkdown, coverMarkdown, resumePDF, coverPDF string) error
```
Update the SQL and args accordingly.

3d. Update the `WorkerStore` interface in `internal/generator/worker.go` to match the new signature.

3e. Update the `JobStore` interface in `internal/web/server.go` if needed (it only uses `GetJob`/`ListJobs` — no change required there, but `WorkerStore` must be updated).

### Step 4: Update prompt and generator

File: `internal/generator/prompts.go`

Replace `separator` constant and `systemPrompt`. New constants:
```go
const (
    sepResumeHTML = "---RESUME_HTML---"
    sepCoverMD    = "---COVER_MD---"
    sepCoverHTML  = "---COVER_HTML---"
)

const systemPrompt = `You are an expert resume writer and career coach.
Given a job listing and a base resume in Markdown, produce four sections in this exact format with no extra text:

---RESUME_MD---
[tailored resume in Markdown]
---RESUME_HTML---
[tailored resume as self-contained HTML with inline CSS for PDF printing]
---COVER_MD---
[professional cover letter in Markdown]
---COVER_HTML---
[cover letter as self-contained HTML with inline CSS for PDF printing]`
```

File: `internal/generator/generator.go`

Update `Generator` interface:
```go
type Generator interface {
    Generate(ctx context.Context, job models.Job, baseResume string) (resumeMD, resumeHTML, coverMD, coverHTML string, err error)
}
```

Update `AnthropicGenerator.Generate` to parse the four-section response.

### Step 5: Update worker

File: `internal/generator/worker.go`

5a. Update `Worker` struct: make `converter pdf.Converter` nilable (it already uses the interface, so nil check suffices).

5b. In `processJob`:
```go
resumeHTML, coverHTML, resumeMD, coverMD, err := w.generator.Generate(...)

resumePDF, coverPDF := "", ""

if w.converter != nil {
    jobDir := filepath.Join(w.outputDir, fmt.Sprintf("%d", job.ID))
    resumePDF = filepath.Join(jobDir, "resume.pdf")
    coverPDF  = filepath.Join(jobDir, "cover_letter.pdf")

    if err := w.converter.PDFFromHTML(ctx, resumeHTML, resumePDF); err != nil {
        log.Warn("resume pdf conversion failed (non-fatal)", "error", err)
        resumePDF = ""
    }
    if err := w.converter.PDFFromHTML(ctx, coverHTML, coverPDF); err != nil {
        log.Warn("cover letter pdf conversion failed (non-fatal)", "error", err)
        coverPDF = ""
    }
}

if err := w.store.UpdateJobGenerated(ctx, 0, job.ID, resumeHTML, coverHTML, resumeMD, coverMD, resumePDF, coverPDF); err != nil { ... }
```

### Step 6: Create exporter package

New file: `internal/exporter/docx.go`

```go
// Package exporter converts Markdown content to office document formats.
package exporter

import (
    "bytes"
    "strings"

    "github.com/gomutex/godocx"
)

// ToDocx converts a Markdown string to a DOCX file and returns the raw bytes.
// It supports: ATX headings (# H1, ## H2, ### H3), bold (**text**),
// italic (_text_ or *text*), unordered lists (- item), and plain paragraphs.
func ToDocx(md string) ([]byte, error) {
    // implementation: parse lines, emit godocx paragraphs/runs
}
```

New file: `internal/exporter/docx_test.go` — unit tests covering each supported Markdown element.

### Step 7: Add download routes and handlers

File: `internal/web/server.go`

7a. Register new routes in `Handler()`:
```go
r.Get("/output/{id}/resume.md",           s.handleDownloadResumeMarkdown)
r.Get("/output/{id}/cover_letter.md",     s.handleDownloadCoverMarkdown)
r.Get("/output/{id}/resume.docx",         s.handleDownloadResumeDocx)
r.Get("/output/{id}/cover_letter.docx",   s.handleDownloadCoverDocx)
```

7b. Handler signatures:

```go
func (s *Server) handleDownloadResumeMarkdown(w http.ResponseWriter, r *http.Request) {
    // fetch job; if ResumeMarkdown == "" return 404
    // Content-Disposition: attachment; filename=resume.md
    // Content-Type: text/markdown
    // Write job.ResumeMarkdown
}

func (s *Server) handleDownloadCoverMarkdown(w http.ResponseWriter, r *http.Request) { ... }

func (s *Server) handleDownloadResumeDocx(w http.ResponseWriter, r *http.Request) {
    // fetch job; if ResumeMarkdown == "" return 404
    // docxBytes, err := exporter.ToDocx(job.ResumeMarkdown)
    // Content-Disposition: attachment; filename=resume.docx
    // Content-Type: application/vnd.openxmlformats-officedocument.wordprocessingml.document
    // Write docxBytes
}

func (s *Server) handleDownloadCoverDocx(w http.ResponseWriter, r *http.Request) { ... }
```

### Step 8: Update UI template

File: `internal/web/templates/job_detail.html`

Replace each single download button with a download group:
```html
{{if .Job.ResumeMarkdown}}
<p>
  {{if .Job.ResumeMarkdown}}
  <a href="/output/{{.Job.ID}}/resume.md" download="resume.md" role="button" class="outline secondary">
    Download Markdown
  </a>
  {{end}}
  <a href="/output/{{.Job.ID}}/resume.docx" download="resume.docx" role="button" class="outline secondary">
    Download DOCX
  </a>
  {{if .Job.ResumePDF}}
  <a href="/output/{{.Job.ID}}/resume.pdf" download="resume.pdf" role="button" class="outline">
    Download PDF
  </a>
  {{end}}
</p>
{{end}}
```
(same pattern for cover letter)

Note: The PDF button is only shown when `job.ResumePDF != ""`, making PDF optional from the UI perspective.

### Step 9: Update main.go (optional PDF)

File: `cmd/jobhuntr/main.go`

```go
var pdfConverter pdf.Converter
if conv, err := pdf.NewRodConverter(); err != nil {
    slog.Warn("PDF converter unavailable; PDF export disabled", "error", err)
} else {
    pdfConverter = conv
    defer conv.Close()
}

worker := generator.NewWorker(db, claudeGen, pdfConverter, ...)
```

### Step 10: Add go.mod dependency

```
go get github.com/gomutex/godocx
```

---

## 6. Acceptance Criteria

- [ ] Claude prompt produces four sections: resume MD, resume HTML, cover MD, cover HTML
- [ ] `Job` struct has `ResumeMarkdown` and `CoverMarkdown` fields
- [ ] Migration `008_add_markdown_columns.sql` applies cleanly on a fresh and existing DB
- [ ] `UpdateJobGenerated` persists all four content fields plus PDF paths
- [ ] Worker stores Markdown even when PDF conversion is skipped or fails
- [ ] `GET /output/{id}/resume.md` returns the Markdown content as a download
- [ ] `GET /output/{id}/cover_letter.md` returns the cover letter Markdown as a download
- [ ] `GET /output/{id}/resume.docx` returns a valid DOCX file readable in LibreOffice/Word
- [ ] `GET /output/{id}/cover_letter.docx` returns a valid DOCX file readable in LibreOffice/Word
- [ ] `GET /output/{id}/resume.pdf` still works for jobs that have a PDF on disk
- [ ] PDF button is hidden in the UI when `job.ResumePDF == ""`
- [ ] `main.go` starts successfully without Chromium; PDF converter initialisation failure is a warning, not a fatal exit
- [ ] All existing tests pass
- [ ] New unit tests for `exporter.ToDocx` cover: headings H1–H3, bold, italic, unordered list, plain paragraph, empty input

---

## 7. Trade-offs and Alternatives

### A. Store DOCX on disk vs. generate on-demand

**Chosen**: Generate on-demand at request time.

Pros: simpler storage management, no migration of existing jobs needed, DOCX generation is fast (<50ms for a typical resume in pure Go).

Cons: small CPU cost per download. Acceptable given low download frequency.

### B. Prompt approach vs. two Claude calls

**Chosen**: Single prompt returning both MD and HTML.

Pros: half the API cost, half the latency, single source of truth per generation.

Cons: prompt is more complex; parsing is slightly more involved.

Fallback: if prompt reliability proves low (separator not found), fall back to a second Claude call for Markdown or use an HTML-to-Markdown library (`github.com/JohannesKaufmann/html-to-markdown` — MIT licensed).

### C. DOCX library choice

**Chosen**: `gomutex/godocx` (MIT, no CGO, active).

The only meaningful alternative is `unidoc/unioffice` which is commercial. `fumiama/go-docx` is AGPL which creates copyleft obligations on the whole application.

If `gomutex/godocx` turns out to lack necessary formatting features, a fallback is to generate DOCX via LibreOffice conversion of the HTML (requires LibreOffice in the Docker image — heavier but fully featured).

### D. Markdown-subset for DOCX conversion

Rather than implementing a full CommonMark parser, we implement a line-by-line state machine supporting the specific Markdown subset Claude produces for resumes:
- ATX headings: `# `, `## `, `### `
- Bold: `**text**`
- Italic: `_text_` and `*text*` (when not bold)
- Unordered lists: `- ` or `* ` at line start
- Blank line = paragraph break

This is intentionally minimal. A full parser (e.g., `github.com/yuin/goldmark`) can be dropped in later if needed.

---

## 8. Dependencies and Prerequisites

| Item | Detail |
|------|--------|
| New Go dependency | `github.com/gomutex/godocx` — MIT license |
| DB migration | `008_add_markdown_columns.sql` — non-destructive ALTER TABLE |
| No new env vars | All changes are code-level |
| Docker image | No change required (DOCX is pure Go, no external binary) |
| Chromium | Still required for PDF if PDF is desired; graceful degradation if absent |

---

## 9. Risks

1. **Claude prompt reliability**: Adding four delimiters increases parsing complexity. The prompt must be very explicit. Risk mitigated by testing with real Claude responses before deploying.

2. **godocx API maturity**: The library is MIT but relatively young. If it lacks a feature (e.g., inline bold/italic within a list item), we may need to drop to lower-level OOXML manipulation or substitute a different library.

3. **Markdown quality from Claude**: The resume Markdown may include tables or other elements not in our supported subset. We should handle unknown elements gracefully (render as plain text rather than panicking).
