# Plan: 5A — Dashboard Layout & Job List

## Overview
HTML dashboard with HTMX dynamic updates, Pico CSS styling, embedded Go templates.

## Steps

### Step 1: Create template files in internal/web/templates/
- layout.html: base layout with Pico CSS CDN, HTMX CDN, nav bar
- dashboard.html: extends layout; status filter tabs, search input, job table
- partials/job_rows.html: table body for HTMX swap + 30s auto-refresh

### Step 2: Add template rendering to server.go
- Embed templates with //go:embed
- GET / → render dashboard.html with all jobs
- GET /partials/job-table → render job_rows.html (supports ?status=, ?q= filters)

### Step 3: Extend Server struct
- Add templates *template.Template field
- Pass config (baseURL) to templates via template data struct

## Files Created/Modified
- internal/web/templates/layout.html (new)
- internal/web/templates/dashboard.html (new)
- internal/web/templates/partials/job_rows.html (new)
- internal/web/server.go (add template rendering, GET /, GET /partials/job-table)
