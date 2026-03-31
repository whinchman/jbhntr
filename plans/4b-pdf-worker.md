# Plan: 4B — PDF Generation & Background Worker

## Overview
PDF converter using go-rod headless Chromium, and background worker that polls
for approved jobs, generates HTML via Claude, converts to PDF, and marks complete.

## Steps

### Step 1: Install go-rod
- go get github.com/go-rod/rod

### Step 2: internal/pdf/pdf.go
Converter interface: PDFFromHTML(ctx, html string, outputPath string) error
RodConverter struct: wraps go-rod Browser launched once at construction.
- NewRodConverter() — connects to browser (rod.New().MustConnect())
- PDFFromHTML: navigate page to blank, set HTML, call Page.PDF(A4, margins), write file

### Step 3: internal/generator/worker.go
Worker struct: store (WorkerStore interface), generator Generator, pdf Converter, outputDir, pollInterval, logger

WorkerStore interface:
- ListJobs(ctx, filter) ([]Job, error)
- UpdateJobStatus(ctx, id, status) error
- UpdateJobGenerated(ctx, id, resumeHTML, coverHTML, resumePDF, coverPDF) error
- GetJob(ctx, id) (*Job, error)

Worker.Start(ctx): ticker loop every 30s
Worker.processApproved(ctx): query approved jobs, for each: generating→generate→PDF→complete/failed

### Step 4: Tests
- Worker tests with mocks (mock Generator, mock Converter, in-memory store)
- Test: happy path → status goes to complete, generated fields set
- Test: generator error → status fails, error saved
- PDF unit test skipped unless INTEGRATION=1 (headless browser required)

## Files Created/Modified
- internal/pdf/pdf.go (replaces doc.go)
- internal/generator/worker.go (new)
- internal/generator/worker_test.go (new)
- cmd/jobhuntr/main.go (wire worker)
