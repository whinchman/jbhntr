# Plan: 5B — Job Detail Page & PDF Downloads

## Overview
Add a job detail page and PDF download endpoints. Users navigate here from the
dashboard to see the full description, approve/reject, and (when complete)
view/download the generated resume and cover letter PDFs.

## Steps

### Step 1: Tests (server_test.go additions)
- GET /jobs/{id} returns 200 with text/html for a known job
- GET /jobs/{id} returns 404 for unknown job
- GET /output/{id}/resume.pdf returns 200 + file data for a complete job whose
  ResumePDF path points to a real temp file; 404 if path is empty; 404 if job
  not found
- GET /output/{id}/cover_letter.pdf same as above for CoverPDF

### Step 2: Create internal/web/templates/job_detail.html
Template blocks: "title" and "content".
Content sections:
- Job header: title, company, location, salary, apply link (external, _blank)
- Status badge (reuse .status-badge CSS class)
- Approve/reject buttons (hx-post → /api/jobs/{id}/approve|reject,
  hx-target="#status-section", hx-swap="outerHTML") only if
  status == discovered or notified
- Full description in a <pre> or block quote
- If status == complete: resume HTML preview in an iframe or div;
  download links for resume.pdf and cover_letter.pdf
- Back link to /

### Step 3: Add routes to server.go
- Register templates/job_detail.html in NewServer (ParseFS call)
- GET /jobs/{id}  → handleJobDetail (render job_detail.html)
- GET /output/{id}/resume.pdf → handleDownloadResume
- GET /output/{id}/cover_letter.pdf → handleDownloadCover
  Both download handlers: get job, check path non-empty, http.ServeFile.
  Set Content-Disposition: attachment; filename=resume.pdf (or cover_letter.pdf)

## Files Created/Modified
- internal/web/templates/job_detail.html  (new)
- internal/web/server.go                  (add 3 routes + handlers)
- internal/web/server_test.go             (add test cases)
