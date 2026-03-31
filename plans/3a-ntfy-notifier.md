# Plan: 3A — Ntfy Notifier

## Overview
Implement the ntfy.sh push notification system and wire it into the Scheduler so
that every newly discovered job triggers a mobile push notification with a link
to the web dashboard.

## Steps

### Step 1: Tests for internal/notifier/notifier.go
Define the Notifier interface and NtfyNotifier struct. Write tests using
httptest.NewServer to verify the HTTP POST payload (title, message, click URL,
priority, tags).

### Step 2: Implement internal/notifier/notifier.go
NtfyNotifier.Notify(ctx, job) POSTs to {server}/{topic} with JSON body:
- title: "New: {job.Title} at {job.Company}"
- message: "{job.Location}" (+ " · {job.Salary}" if non-empty)
- click: "{base_url}/jobs/{job.ID}"
- priority: 3
- tags: ["briefcase"]

Returns wrapped error on non-2xx responses.

### Step 3: Wire Notifier into Scheduler
Update Scheduler to accept an optional Notifier. After RunOnce collects new
jobs, call Notify for each and update their status to "notified" via the store.
Update StoreWriter interface to include UpdateJobStatus (already there in
*store.Store).

### Step 4: Wire into main.go
Instantiate NtfyNotifier and pass to Scheduler in main.go.

## Files Created/Modified
- internal/notifier/notifier.go      (new)
- internal/notifier/notifier_test.go (new)
- internal/scraper/scheduler.go      (add notifier field + notification loop)
- internal/scraper/scheduler_test.go (extend tests)
- cmd/jobhuntr/main.go               (wire notifier)
