# TODO

Stakeholder-approved work ready for worker agents. Items here have been
researched, planned, and approved — they are ready to implement.

Worker agents (coder, designer, automation, qa, code-reviewer) pick up
`[ ]` items from this file.

---

## oauth-google — Google OAuth (ALREADY IMPLEMENTED)

**Note:** Architect confirmed Google OAuth is fully implemented in `internal/web/auth.go`.
Routes, session handling, CSRF, DB upsert, and tests all exist.
Only gap: operator setup docs in README (Google Console steps, env vars).
No implementation tasks needed — close this item.


---


## analytics — Stats Dashboard

**Plan:** plans/analytics.md
**Tasks:** analytics-1-store, analytics-2-handlers, analytics-3-code-review, analytics-4-qa
**Depends on:** job-pipeline-pages (needs application_status column from migration 011)

Dedicated `/stats` route. Single conditional-aggregation SQL query for 7 counters. Stat cards + CSS bar chart for weekly job discovery trend. Per-user only.

**Task order:**
1. analytics-1-store (coder) — after job-pipeline-1-migration
2. analytics-2-handlers (coder)
3. analytics-3-code-review + analytics-4-qa (parallel)
