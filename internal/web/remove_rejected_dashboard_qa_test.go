// NOTE: Go is not installed in this container. These tests require a Docker
// build (or a local Go toolchain) to execute:
//
//	docker build -t jobhuntr . && docker run --rm jobhuntr go test ./internal/web/...
//
// Run from the repo root. All tests should pass with no regressions.
//
// QA tests for GH #1 — "remove rejected jobs from main dashboard".
//
// Acceptance criteria covered here:
//   - GET /partials/job-table passes ExcludeStatuses: [rejected] to ListJobs
//   - GET /?status=discovered regression: status filter still wired after ExcludeStatuses addition
//   - GET /jobs/rejected does NOT pass ExcludeStatuses to ListJobs (that page must show rejected jobs)
//   - GET /jobs/rejected returns 200 HTML (smoke check)

package web_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/whinchman/jobhuntr/internal/models"
	"github.com/whinchman/jobhuntr/internal/web"
)

// ─── GET /partials/job-table ──────────────────────────────────────────────────

// TestJobTablePartial_ExcludeStatusesWired verifies that handleJobTablePartial
// passes ExcludeStatuses: [rejected] in the filter forwarded to ListJobs.
// This confirms the partial handler is wired identically to handleDashboard.
func TestJobTablePartial_ExcludeStatusesWired(t *testing.T) {
	spy := newSpyJobStore(
		htmxJob(1, models.StatusDiscovered),
		htmxJob(2, models.StatusRejected),
	)
	ts := newSpyServer(t, spy)
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/partials/job-table")
	if err != nil {
		t.Fatalf("GET /partials/job-table: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("GET /partials/job-table status = %d, want 200; body: %s", resp.StatusCode, body)
	}

	f := spy.LastFilter()
	if len(f.ExcludeStatuses) == 0 {
		t.Fatal("handleJobTablePartial did not set ExcludeStatuses — rejected jobs may appear in the partial")
	}
	found := false
	for _, s := range f.ExcludeStatuses {
		if s == models.StatusRejected {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ExcludeStatuses = %v, want to contain %q", f.ExcludeStatuses, models.StatusRejected)
	}
}

// ─── GET /?status=discovered regression ──────────────────────────────────────

// TestDashboard_DiscoveredFilter_Regression verifies that GET /?status=discovered
// still forwards Status = "discovered" to ListJobs after the ExcludeStatuses
// addition. This is the regression guard: adding ExcludeStatuses must not
// silently drop the existing Status filter.
func TestDashboard_DiscoveredFilter_Regression(t *testing.T) {
	spy := newSpyJobStore(
		htmxJob(1, models.StatusDiscovered),
		htmxJob(2, models.StatusNotified),
	)
	ts := newSpyServer(t, spy)
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/?status=discovered")
	if err != nil {
		t.Fatalf("GET /?status=discovered: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("GET /?status=discovered status = %d, want 200; body: %s", resp.StatusCode, body)
	}

	f := spy.LastFilter()
	if f.Status != models.StatusDiscovered {
		t.Errorf("ListJobs called with Status = %q, want %q", f.Status, models.StatusDiscovered)
	}
	// ExcludeStatuses must still be set even when an explicit status filter is given.
	if len(f.ExcludeStatuses) == 0 {
		t.Error("ExcludeStatuses was not set when ?status=discovered — dashboard may have lost the rejected-exclusion filter")
	}
}

// TestJobTablePartial_DiscoveredStatusParam verifies that GET
// /partials/job-table?status=discovered forwards Status = "discovered" to
// ListJobs and still sets ExcludeStatuses.
func TestJobTablePartial_DiscoveredStatusParam(t *testing.T) {
	spy := newSpyJobStore(
		htmxJob(1, models.StatusDiscovered),
		htmxJob(2, models.StatusNotified),
	)
	ts := newSpyServer(t, spy)
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/partials/job-table?status=discovered")
	if err != nil {
		t.Fatalf("GET /partials/job-table?status=discovered: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, want 200; body: %s", resp.StatusCode, body)
	}

	f := spy.LastFilter()
	if f.Status != models.StatusDiscovered {
		t.Errorf("ListJobs called with Status = %q, want %q", f.Status, models.StatusDiscovered)
	}
	if len(f.ExcludeStatuses) == 0 {
		t.Error("ExcludeStatuses was not set on partial with ?status=discovered")
	}
}

// ─── GET /jobs/rejected ──────────────────────────────────────────────────────

// TestRejectedJobsPage_NoExcludeStatuses verifies that handleRejectedJobs does
// NOT pass ExcludeStatuses to ListJobs. The /jobs/rejected page must show
// rejected jobs, so applying ExcludeStatuses:[rejected] would break it.
func TestRejectedJobsPage_NoExcludeStatuses(t *testing.T) {
	spy := newSpyJobStore(
		htmxJob(1, models.StatusRejected),
		htmxJob(2, models.StatusDiscovered),
	)
	ts := newSpyServer(t, spy)
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/jobs/rejected")
	if err != nil {
		t.Fatalf("GET /jobs/rejected: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("GET /jobs/rejected status = %d, want 200; body: %s", resp.StatusCode, body)
	}

	f := spy.LastFilter()
	if len(f.ExcludeStatuses) > 0 {
		t.Errorf("handleRejectedJobs forwarded ExcludeStatuses = %v; the rejected page must not exclude rejected jobs", f.ExcludeStatuses)
	}
	// Confirm it does use Status = rejected to scope results.
	if f.Status != models.StatusRejected {
		t.Errorf("handleRejectedJobs forwarded Status = %q, want %q", f.Status, models.StatusRejected)
	}
}

// TestRejectedJobsPage_Returns200HTML verifies that GET /jobs/rejected returns
// 200 with a text/html content type (route is wired up, template renders).
func TestRejectedJobsPage_Returns200HTML(t *testing.T) {
	ms := newMockJobStore(newTestJob(5, models.StatusRejected))
	srv := web.NewServer(ms)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/jobs/rejected")
	if err != nil {
		t.Fatalf("GET /jobs/rejected: %v", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET /jobs/rejected status = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}
}
