# Review Ready: modern-design + resume-export + local-debug

**Date**: 2026-04-03

All three features are merged to `development` locally. Review and push when satisfied.

---

## modern-design — Update design look to be more modern

Single `app.css` override (799 lines) on PicoCSS v2. Fresh design tokens, indigo accent, 7 status badge variants. Static file serving via Go embed. All 8 templates cleaned of inline styles.

**Validation:** Code Review ×3 PASS, QA PASS.
**BUG-010** (Low/cosmetic): `.providers-section` missing margin rule — slight spacing loss above provider buttons on login page.

---

## resume-export — Markdown, DOCX, and optional PDF downloads

Migration 008 adds `resume_markdown`/`cover_markdown`. Generator returns 4 sections. `internal/exporter.ToDocx()` via `gomutex/godocx`. PDF optional (non-fatal). 4 new download routes + conditional UI buttons.

**Validation:** Code Review ×3 PASS, QA PASS.
**BUG-012** (Warning): Italic `_` parser doesn't check word boundaries — `node_modules`-style identifiers may italicise in DOCX.
**BUG-013** (Warning): DOCX test body guard checks `< 2` but accesses `body[:4]` — panic on short body.

---

## local-debug — Local debug deployment for testing

`Makefile` (9 targets: `make dev`, `make test`, etc.), `Dockerfile.dev`, docker-compose `dev` profile, `.air.toml` hot-reload, `.env.example`, `run.sh` required-var warnings, `tmp/` gitignored, `agent.yaml` test command fixed.

**Validation:** Code Review PASS (0 findings), QA PASS (no regressions).

**Usage:**
```
cp .env.example .env   # fill in secrets
make dev               # Docker hot-reload via air
# or
make db-up && make dev-native   # native with hot-reload
```

---

## To Ship

```
git push origin development
```
