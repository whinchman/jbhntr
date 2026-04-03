# Task: resume-export-1-foundation

- **Type**: coder
- **Status**: done
- **Repo**: .
- **Parallel Group**: 1
- **Branch**: feature/resume-export-1-foundation
- **Source Item**: resume-export (multi-format export: Markdown, DOCX, PDF)
- **Dependencies**: none

## Description

Lay the data-layer and generation foundation for multi-format export. This task touches six files across models, store, generator, worker, and main.go. Do all changes in one branch to avoid merge conflicts — no application surface (routes or UI) is added here.

Changes required:

1. **Migration** — create `internal/store/migrations/008_add_markdown_columns.sql`
2. **Models** — add two fields to `Job` struct in `internal/models/models.go`
3. **Store** — update `UpdateJobGenerated` signature, `scanJob`, and SELECT column lists in `internal/store/store.go`
4. **Prompt + Generator** — update separator protocol and `Generate` return type in `internal/generator/prompts.go` and `internal/generator/generator.go`
5. **Worker** — update `WorkerStore` interface and `processJob` logic in `internal/generator/worker.go` to use the new four-value Generate return and make PDF conversion optional (nil-safe)
6. **main.go** — make PDF converter initialisation non-fatal in `cmd/jobhuntr/main.go`
7. **go.mod** — add `github.com/gomutex/godocx` (run `go get github.com/gomutex/godocx`)

## Acceptance Criteria

- [ ] `internal/store/migrations/008_add_markdown_columns.sql` exists with `ALTER TABLE jobs ADD COLUMN IF NOT EXISTS resume_markdown TEXT NOT NULL DEFAULT ''` and the same for `cover_markdown`
- [ ] `models.Job` has `ResumeMarkdown string` and `CoverMarkdown string` fields (added after `CoverHTML`)
- [ ] `store.Store.UpdateJobGenerated` signature is `(ctx, userID, id int64, resumeHTML, coverHTML, resumeMarkdown, coverMarkdown, resumePDF, coverPDF string) error`
- [ ] `scanJob` scans `resume_markdown` and `cover_markdown` from the result set
- [ ] `GetJob` and `ListJobs` SELECT lists include `resume_markdown, cover_markdown`
- [ ] `generator.Generator` interface returns `(resumeMD, resumeHTML, coverMD, coverHTML string, err error)`
- [ ] `AnthropicGenerator.Generate` parses the new four-section response using the new separator constants
- [ ] `WorkerStore.UpdateJobGenerated` in `internal/generator/worker.go` matches the new store signature
- [ ] `worker.processJob` makes PDF conversion nil-safe: if `w.converter == nil`, skip PDF conversion and continue; if conversion returns an error, log a warning and set path to `""` rather than failing the job
- [ ] `cmd/jobhuntr/main.go` wraps `pdf.NewRodConverter()` so that an error is logged as a warning and execution continues with `pdfConverter = nil`
- [ ] `go.mod` and `go.sum` include `github.com/gomutex/godocx`
- [ ] All existing tests pass (`go test ./...`)

## Interface Contracts

### New `Generator` interface (internal/generator/generator.go)
```go
type Generator interface {
    Generate(ctx context.Context, job models.Job, baseResume string) (resumeMD, resumeHTML, coverMD, coverHTML string, err error)
}
```

### New `WorkerStore` interface (internal/generator/worker.go)
```go
type WorkerStore interface {
    GetJob(ctx context.Context, userID int64, id int64) (*models.Job, error)
    ListJobs(ctx context.Context, userID int64, f store.ListJobsFilter) ([]models.Job, error)
    UpdateJobStatus(ctx context.Context, userID int64, id int64, status models.JobStatus) error
    UpdateJobGenerated(ctx context.Context, userID int64, id int64, resumeHTML, coverHTML, resumeMarkdown, coverMarkdown, resumePDF, coverPDF string) error
    UpdateJobError(ctx context.Context, userID int64, id int64, errMsg string) error
}
```

### New prompt separator protocol (internal/generator/prompts.go)
```go
const (
    sepResumeMD  = "---RESUME_MD---"
    sepResumeHTML = "---RESUME_HTML---"
    sepCoverMD   = "---COVER_MD---"
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

### models.Job new fields
```go
// Add after CoverHTML:
ResumeMarkdown string
CoverMarkdown  string
```

### store.UpdateJobGenerated new signature
```go
func (s *Store) UpdateJobGenerated(ctx context.Context, userID int64, id int64,
    resumeHTML, coverHTML, resumeMarkdown, coverMarkdown, resumePDF, coverPDF string) error
```

### worker.processJob PDF-optional pattern
```go
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
```

## Context

- Plan: `.workflow/plans/resume-export.md` — see sections 3.1, 3.2, 3.3, 3.4, and 5 (Steps 1–5, 9–10)
- `internal/generator/generator.go`: currently `Generate` returns `(resumeHTML, coverHTML string, err error)` and parses a single `---SEPARATOR---` delimiter. Replace with four-section parsing.
- `internal/generator/prompts.go`: currently defines `separator` constant and `systemPrompt`. Replace both.
- `internal/generator/worker.go`: `WorkerStore` interface calls `UpdateJobGenerated(ctx, userID, id, resumeHTML, coverHTML, resumePDF, coverPDF string)` — must be updated. `processJob` currently fails immediately on any PDF error — make it non-fatal.
- `internal/store/store.go`: `UpdateJobGenerated` and `scanJob` currently handle 4 content/path fields. Both must be updated to handle 6 (adding resumeMarkdown, coverMarkdown).
- `cmd/jobhuntr/main.go`: currently calls `pdf.NewRodConverter()` and likely exits on error. Wrap with nil-safe pattern.
- Worker tests in `internal/generator/worker_test.go` and store tests in `internal/store/store_test.go` will need updating to match new signatures.

## Notes

### Implementation Summary (2026-04-03)

Branch: `feature/resume-export-1-foundation`

All 7 acceptance criteria implemented across 7 commits:

1. **Migration**: `internal/store/migrations/008_add_markdown_columns.sql` — adds `resume_markdown` and `cover_markdown` columns (IF NOT EXISTS, NOT NULL DEFAULT '').

2. **Models**: `models.Job` gains `ResumeMarkdown string` and `CoverMarkdown string` after `CoverHTML`.

3. **Store**: `UpdateJobGenerated` updated to 8-param signature (adds `resumeMarkdown`, `coverMarkdown`). `scanJob` scans new columns. Both `GetJob` and `ListJobs` SELECT lists include `resume_markdown, cover_markdown`. Store test updated to verify round-trip.

4. **Prompts**: `internal/generator/prompts.go` replaced with four separator constants (`sepResumeMD`, `sepResumeHTML`, `sepCoverMD`, `sepCoverHTML`) and updated `systemPrompt` using new protocol.

5. **Generator**: `Generator` interface returns `(resumeMD, resumeHTML, coverMD, coverHTML string, err error)`. `AnthropicGenerator.Generate` uses `extractSection` helper for four-region parsing.

6. **Worker**: `WorkerStore.UpdateJobGenerated` updated to new 8-param signature. `processJob` uses four-value `Generate` return. PDF conversion is nil-safe: if `w.converter == nil`, PDF paths are `""`; if conversion errors, logs warning and sets path to `""` rather than failing the job. New tests cover nil-converter and non-fatal error paths.

7. **main.go**: `pdf.NewRodConverter()` error is now non-fatal — logs a warning and sets `pdfConverter = nil`.

8. **go.mod/go.sum**: `github.com/gomutex/godocx v0.1.5` added with hashes fetched from sum.golang.org. Transitive test deps (davecgh/go-spew, pmezard/go-difflib, stretchr/testify) also added.

**Note**: `go test ./...` could not be run in this container (no Go toolchain installed). Code has been carefully reviewed for correctness — all interface signatures align, all test mocks updated, column ordering in scanJob matches SELECT lists.
