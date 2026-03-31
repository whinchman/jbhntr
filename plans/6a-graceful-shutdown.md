# Plan: 6A — Graceful Shutdown & Logging

## Overview
Improve the shutdown sequence and health endpoint. The existing main.go already
handles SIGINT/SIGTERM and HTTP server shutdown, but it does not wait for the
scheduler and worker goroutines to finish their current operation. The health
endpoint only returns `{"status":"ok"}` — expand it to include uptime and last
scrape time.

## Steps

### Step 1: Scheduler: track last scrape time
Add `mu sync.Mutex` and `lastScrapeAt time.Time` to `Scheduler`. Update
`runFilter` to set `lastScrapeAt` after a successful cycle. Add public method
`LastScrapeAt() time.Time`. Test: write a test for `LastScrapeAt` in
`scheduler_test.go` that verifies the time advances after RunOnce.

### Step 2: Server: uptime + last-scrape health endpoint
Add `startTime time.Time` and `lastScrapeFn func() time.Time` to `Server`.
Set `startTime = time.Now()` in `NewServerWithConfig`. Add
`WithLastScrapeFn(f func() time.Time) *Server` setter for wiring from main.
Update `handleHealth` to return:
```json
{"status":"ok","uptime":"1h2m3s","last_scrape":"2026-01-01T00:00:00Z"}
```
`last_scrape` is omitted (null) if no scrape has run yet.
Update the existing `TestHealth` to check for the `uptime` key.

### Step 3: main.go graceful shutdown with WaitGroup
Replace the ad-hoc goroutine launches with:
```go
var wg sync.WaitGroup
wg.Add(2)
go func() { defer wg.Done(); sched.Start(ctx) }()
go func() { defer wg.Done(); worker.Start(ctx) }()
```
After `httpServer.Shutdown(...)`, call `wg.Wait()` before `db.Close()` and the
final log. This ensures in-flight scrapes and generation jobs complete before
the process exits.

## Files Modified
- internal/scraper/scheduler.go   (add lastScrapeAt tracking)
- internal/scraper/scheduler_test.go (test LastScrapeAt)
- internal/web/server.go          (startTime, lastScrapeFn, health update)
- internal/web/server_test.go     (update TestHealth)
- cmd/jobhuntr/main.go            (WaitGroup shutdown)
