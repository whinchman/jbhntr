# Plan: 3B — Web Server & API Endpoints

## Overview
Set up an HTTP server using chi with JSON API endpoints for job management.
Includes request logging middleware, graceful shutdown support, and full
handler tests.

## Steps

### Step 1: Tests for internal/web/server.go handlers
Write handler tests using httptest.NewRecorder covering all endpoints.

### Step 2: Implement internal/web/server.go
Server struct with chi router. Endpoints:
- GET  /health                     → {"status":"ok"}
- GET  /api/jobs                   → JSON array (status/q/page/limit params)
- GET  /api/jobs/{id}              → JSON job or 404
- POST /api/jobs/{id}/approve      → discovered/notified → approved
- POST /api/jobs/{id}/reject       → → rejected

Middleware: slog request logger.
Wire into main.go with graceful shutdown (http.Server.Shutdown).

## Files Created/Modified
- internal/web/server.go      (new)
- internal/web/server_test.go (new)
- cmd/jobhuntr/main.go        (start HTTP server + graceful shutdown)
