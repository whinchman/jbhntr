# Task: resume-export-2-exporter

- **Type**: coder
- **Status**: done
- **Repo**: .
- **Parallel Group**: 2
- **Branch**: feature/resume-export-2-exporter
- **Source Item**: resume-export (multi-format export: Markdown, DOCX, PDF)
- **Dependencies**: resume-export-1-foundation

## Description

Create the new `internal/exporter` package that converts Markdown to DOCX using the `github.com/gomutex/godocx` library. This is a self-contained new package with no dependencies on other in-progress changes. The `godocx` dependency will have been added to `go.mod` by the foundation task.

Files to create:
- `internal/exporter/docx.go` — public `ToDocx(md string) ([]byte, error)` function
- `internal/exporter/docx_test.go` — unit tests

## Acceptance Criteria

- [x] `internal/exporter/docx.go` exists in package `exporter`
- [x] `ToDocx(md string) ([]byte, error)` is exported and compiles cleanly
- [x] Handles ATX headings: `# H1`, `## H2`, `### H3`
- [x] Handles bold: `**text**`
- [x] Handles italic: `_text_` and `*text*` (when not bold)
- [x] Handles unordered lists: lines starting with `- ` or `* `
- [x] Handles plain paragraphs and blank lines as paragraph breaks
- [x] Empty string input returns a valid (empty) DOCX without error
- [x] Unknown/unsupported Markdown (e.g. tables, code blocks) is rendered as plain text rather than panicking
- [x] `internal/exporter/docx_test.go` has unit tests covering: H1, H2, H3, bold run, italic run, unordered list item, plain paragraph, empty input, mixed content
- [x] All tests pass (`go test ./internal/exporter/...`)

## Interface Contracts

### Package API
```go
// Package exporter converts Markdown content to office document formats.
package exporter

// ToDocx converts a Markdown string to a DOCX file and returns the raw bytes.
// Supported Markdown subset: ATX headings (# H1, ## H2, ### H3), bold (**text**),
// italic (_text_ or *text*), unordered lists (- item or * item), and plain paragraphs.
// Unknown elements are rendered as plain text.
func ToDocx(md string) ([]byte, error)
```

This function is consumed by the web server (task resume-export-3-routes) as:
```go
import "github.com/whinchman/jobhuntr/internal/exporter"

docxBytes, err := exporter.ToDocx(job.ResumeMarkdown)
// write docxBytes to http.ResponseWriter
```

### godocx usage pattern
Use `github.com/gomutex/godocx` to construct the document. Typical pattern:
```go
doc, err := godocx.NewDocument()
// add paragraphs: doc.AddParagraph(...)
// write to buffer: doc.Save(buf)
```
Consult the godocx README/examples at `go doc github.com/gomutex/godocx` for the exact API surface.

## Context

- Plan: `.workflow/plans/resume-export.md` — see sections 3.2, 5 (Step 6), and 7D
- The Markdown subset Claude produces for resumes is intentionally limited. Implement a simple line-by-line state machine rather than a full parser.
- The implementation approach from the plan (section 7D): parse lines, detect ATX headings by prefix (`# `, `## `, `### `), detect list items by `- ` or `* ` prefix, detect bold/italic by `**` / `_` / `*` markers within a line, treat everything else as a plain paragraph.
- Blank lines separate paragraphs; consecutive non-blank non-heading non-list lines belong to the same paragraph.
- The `gomutex/godocx` library was selected specifically because it is MIT-licensed, pure Go (no CGO), and supports programmatic paragraph/run creation. Do not substitute another library.
- If `godocx` does not expose a `Save(w io.Writer) error` method, write to a `bytes.Buffer` using whatever write/encode method it provides and return `buf.Bytes()`.

## Notes

### Code Review (Code Reviewer Agent)

**Verdict**: approve (with one warning filed as BUG-012)

**Findings summary**: 0 critical, 1 warning, 2 info

#### [WARNING] internal/exporter/docx.go:parseInline — underscore treated as italic delimiter even inside words

`parseInline` treats any `_` as the start of an italic span and searches forward for the next `_`. This means prose containing underscores in technical identifiers (e.g. `node_modules`, `role_arn`, `secret_access_key`) will be partially italicised. Example: `"node_modules, my_project"` → "modules, my" rendered italic. This is a real-world hazard for resume content that lists technical skills or environment variable names with underscores.

Affected file/line: `internal/exporter/docx.go:133–141`

Filed as BUG-012.

Fix suggestion: Only treat `_` as an italic delimiter when it appears at a word boundary (preceded by a space, start-of-string, or punctuation, and followed by a non-space character). The simplest approach: check `i == 0 || text[i-1] == ' '` before treating `_` as opening italic.

#### [INFO] internal/exporter/docx.go:51,55,60 — AddHeading errors silently discarded

`_, _ = doc.AddHeading(...)` discards errors. For heading levels 1–3, `AddHeading` never errors (it only errors for level > 9), so this is safe today. If the calling code ever used a variable level, silent discard could hide bugs. Not a current issue.

#### [INFO] internal/exporter/docx.go:69-70 — AddParagraph("") emits an empty run before addInlineRuns adds content runs

`doc.AddParagraph("")` internally calls `p.AddText("")`, adding an empty run to the paragraph before `addInlineRuns` appends the actual content. The empty run is harmless (zero-length text element in the XML) but wastes space. Could use `newParagraph` directly if the API allowed it; not currently possible with the godocx public API.

#### No regressions to task-1 files

The diff on this branch (vs `feature/resume-export-1-foundation`) is limited to `internal/exporter/docx.go`, `internal/exporter/docx_test.go`, and `go.mod`/`go.sum`. No task-1 files (models, store, generator, worker) were modified.

#### BUG-011 resolved

BUG-011 (godocx added as unused indirect dependency) is resolved by this task: `gomutex/godocx` is now a direct dependency actively imported by `internal/exporter/docx.go`. `go mod tidy` produces no changes on this branch.

---

### Implementation Summary (Coder Agent)

**Branch**: `feature/resume-export-2-exporter` (branched from `feature/resume-export-1-foundation`)

**Files created**:
- `internal/exporter/docx.go` — `ToDocx(md string) ([]byte, error)` using `github.com/gomutex/godocx`
- `internal/exporter/docx_test.go` — 18 unit tests, all passing

**Approach**: Line-by-line state machine parser as specified in the plan. Longer heading prefixes (`### `) checked before shorter ones (`# `). Inline spans handled by `parseInline()` which walks the string byte-by-byte detecting `**bold**`, `_italic_`, and `*italic*` delimiters. Unordered lists use godocx style `"List Bullet"`. Unknown Markdown (tables, code blocks, etc.) falls through to plain paragraph accumulation.

**Test coverage**: empty input, H1/H2/H3 headings, bold run, italic run (both `_` and `*`), unordered list (`-` and `*`), plain paragraph, blank-line paragraph separation, mixed content, unknown Markdown (plain text fallback), plus `parseInline` unit tests.

**Test results**: `go test ./internal/exporter/... -v` — 18/18 PASS

**Note**: `go mod tidy` was run to resolve a missing go.sum entry for `github.com/stretchr/testify`. The go.mod was reorganized (direct vs indirect dependencies) as a side effect.
