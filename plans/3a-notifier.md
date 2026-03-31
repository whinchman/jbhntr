# Plan: 3A — Ntfy Notifier

## Overview
Implement the Notifier that sends push notifications via ntfy.sh when new jobs are found.
Wire it into the scheduler so newly discovered jobs trigger a notification.

## Steps

### Step 1: Implement internal/notifier/notifier.go
Notifier struct: topic, server, base_url, http.Client

Notifier interface:
  Notify(ctx context.Context, job models.Job) error

NtfyNotifier.Notify():
- POST to {server}/{topic}
- JSON body: {"title": "New: {Title} at {Company}", "message": "{Location}[ — {Salary}]",
  "click": "{base_url}/jobs/{id}", "priority": 3, "tags": ["briefcase"]}
- Salary included in message only if non-empty

### Step 2: Update StoreWriter in scheduler.go
Add UpdateJobStatus to StoreWriter interface (already on real store).

### Step 3: Wire notifier into scheduler.go
After CreateJob returns inserted=true:
- Call notifier.Notify(ctx, job)
- On success: call store.UpdateJobStatus(ctx, job.ID, StatusNotified)
- On error: log but don't fail the scrape run

Scheduler struct needs a Notifier field.
NewScheduler updated to accept Notifier (may be nil → skip notification).

### Step 4: Tests (internal/notifier/notifier_test.go)
- httptest.NewServer captures POST body
- Verify Content-Type: application/json
- Verify JSON fields: title, message, click, priority, tags
- Salary in message when set, omitted when empty
- HTTP error returns wrapped error

### Step 5: Update scheduler_test.go mock
- Add UpdateJobStatus to mockStore (already present in existing test)
- Add Notifier field / mock notifier

## Files Created/Modified
- internal/notifier/notifier.go (replaces doc.go)
- internal/notifier/notifier_test.go
- internal/scraper/scheduler.go (add Notifier field, wire UpdateJobStatus)
- internal/scraper/scheduler_test.go (extend mock)
