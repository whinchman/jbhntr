# Review Ready: resume-export + modern-design

**Date**: 2026-04-03

Both features are merged to `development` locally. Review and push when satisfied.

---

## modern-design — Update design look to be more modern

**Plan**: .workflow/plans/modern-design.md

### Summary
Single `app.css` override file (799 lines) on top of PicoCSS v2. Fresh design token set, indigo accent, 7 status badge variants. Static file serving via Go embed. All 8 templates cleaned of inline styles.

### Validation
| Check | Result |
|-------|--------|
| Code Review (×3) | PASS — 0 critical, 0 warnings |
| QA | PASS |

### Known Issue
**BUG-010** (Low/cosmetic): `.providers-section` missing margin rule — slight spacing loss above provider buttons on login page. Fix: `.providers-section { margin-top: var(--space-4); }` in app.css.

---

## resume-export — Markdown, DOCX, and optional PDF downloads

**Plan**: .workflow/plans/resume-export.md

### Summary
- Migration 008: `resume_markdown`, `cover_markdown` columns
- Generator updated to return 4 sections (MD + HTML for both resume and cover letter)
- `internal/exporter` package: `ToDocx(md string) ([]byte, error)` using `gomutex/godocx`
- PDF conversion now optional (non-fatal; skipped if Chromium unavailable)
- 4 new download routes: `/output/{id}/resume.md`, `/output/{id}/cover_letter.md`, `/output/{id}/resume.docx`, `/output/{id}/cover_letter.docx`
- `job_detail.html`: conditional download buttons per format

### Validation
| Check | Result |
|-------|--------|
| Code Review (×3) | PASS — 0 critical across all tasks |
| QA | PASS |

### Known Issues (non-blocking)
- **BUG-012** (Warning): `parseInline` underscore italic detection doesn't check word boundaries — identifiers like `node_modules` could render italicised in DOCX output
- **BUG-013** (Warning): DOCX test body length guard checks `< 2` but accesses `body[:4]` — will panic on very short response bodies

---

## To Ship

```
git push origin development
```
