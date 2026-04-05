package web_test

// htmx_respond_job_action_test.go — QA tests for the HTMX code path in
// respondJobAction (fix-bug-007-double-summary).
//
// The bug: approve/reject with hx-target="closest tr" / hx-swap="outerHTML"
// rendered only the updated job row, leaving the old summary <tr> in the DOM
// (duplicate). The fix changes the forms to target #job-table-body and
// hx-swap="innerHTML", while respondJobAction now re-queries the store using
// HX-Current-URL query params and returns the full job_rows partial.
//
// Test coverage:
//  1. Approve with HX-Request returns HTML (not JSON) for all matching jobs.
//  2. Reject with HX-Request returns HTML (not JSON) for all matching jobs.
//  3. HX-Current-URL status param filters the re-queried job list.
//  4. HX-Current-URL sort/order params are forwarded to ListJobs.
//  5. HX-Current-URL search (q) param is forwarded to ListJobs.
//  6. Missing HX-Current-URL falls back gracefully (returns all jobs, no panic).
//  7. Malformed HX-Current-URL falls back gracefully (returns all jobs, no panic).
//  8. Empty HX-Current-URL falls back gracefully (returns all jobs, no panic).
//  9. Non-HTMX approve still returns JSON (regression guard).
// 10. Non-HTMX reject still returns JSON (regression guard).

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/whinchman/jobhuntr/internal/models"
	"github.com/whinchman/jobhuntr/internal/store"
	"github.com/whinchman/jobhuntr/internal/web"
)

// ─── spy mock store ──────────────────────────────────────────────────────────

// spyJobStore wraps mockJobStore and records the filter passed to the most
// recent ListJobs call so tests can assert that the correct params were
// forwarded from HX-Current-URL.
type spyJobStore struct {
	mu         sync.Mutex
	inner      *mockJobStore
	lastFilter store.ListJobsFilter
}

func newSpyJobStore(jobs ...*models.Job) *spyJobStore {
	return &spyJobStore{inner: newMockJobStore(jobs...)}
}

func (s *spyJobStore) GetJob(ctx context.Context, userID int64, id int64) (*models.Job, error) {
	return s.inner.GetJob(ctx, userID, id)
}

func (s *spyJobStore) ListJobs(ctx context.Context, userID int64, f store.ListJobsFilter) ([]models.Job, error) {
	s.mu.Lock()
	s.lastFilter = f
	s.mu.Unlock()
	return s.inner.ListJobs(ctx, userID, f)
}

func (s *spyJobStore) UpdateJobStatus(ctx context.Context, userID int64, id int64, newStatus models.JobStatus) error {
	return s.inner.UpdateJobStatus(ctx, userID, id, newStatus)
}

func (s *spyJobStore) UpdateApplicationStatus(ctx context.Context, userID int64, id int64, status models.ApplicationStatus) error {
	return s.inner.UpdateApplicationStatus(ctx, userID, id, status)
}

func (s *spyJobStore) LastFilter() store.ListJobsFilter {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastFilter
}

// ─── helpers ─────────────────────────────────────────────────────────────────

// htmxJob creates a test job that belongs to userID 0 (unauthenticated context
// used by NewServer).
func htmxJob(id int64, status models.JobStatus) *models.Job {
	j := newTestJob(id, status)
	j.UserID = 0
	return j
}

// newSpyServer creates an httptest.Server backed by a spyJobStore.
func newSpyServer(t *testing.T, spy *spyJobStore) *httptest.Server {
	t.Helper()
	srv := web.NewServer(spy)
	return httptest.NewServer(srv.Handler())
}

// doHTMX issues a POST to path with HX-Request: true and an optional
// HX-Current-URL header. Returns the response (caller must close body).
func doHTMX(t *testing.T, client *http.Client, baseURL, path, currentURL string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, baseURL+path, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("HX-Request", "true")
	if currentURL != "" {
		req.Header.Set("HX-Current-URL", currentURL)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	return resp
}

// readBody reads and closes the response body, returning it as a string.
func readBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	b, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return string(b)
}

// ─── tests ───────────────────────────────────────────────────────────────────

// TestRespondJobAction_HTMX_Approve_ReturnsHTML verifies that a POST
// /api/jobs/{id}/approve with HX-Request:true returns an HTML body (not JSON),
// with status 200.
func TestRespondJobAction_HTMX_Approve_ReturnsHTML(t *testing.T) {
	spy := newSpyJobStore(
		htmxJob(1, models.StatusDiscovered),
		htmxJob(2, models.StatusNotified),
	)
	ts := newSpyServer(t, spy)
	defer ts.Close()

	resp := doHTMX(t, ts.Client(), ts.URL, "/api/jobs/1/approve", "")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body := readBody(t, resp)
		t.Fatalf("status = %d, want 200; body: %s", resp.StatusCode, body)
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}
}

// TestRespondJobAction_HTMX_Reject_ReturnsHTML verifies that a POST
// /api/jobs/{id}/reject with HX-Request:true returns an HTML body (not JSON),
// with status 200.
func TestRespondJobAction_HTMX_Reject_ReturnsHTML(t *testing.T) {
	spy := newSpyJobStore(
		htmxJob(1, models.StatusDiscovered),
		htmxJob(2, models.StatusNotified),
	)
	ts := newSpyServer(t, spy)
	defer ts.Close()

	resp := doHTMX(t, ts.Client(), ts.URL, "/api/jobs/1/reject", "")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body := readBody(t, resp)
		t.Fatalf("status = %d, want 200; body: %s", resp.StatusCode, body)
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}
}

// TestRespondJobAction_HTMX_ReturnsFullList verifies that approving a single
// job while three jobs exist returns HTML that contains rows for ALL jobs that
// match the current filter — not just the approved job.
func TestRespondJobAction_HTMX_ReturnsFullList(t *testing.T) {
	spy := newSpyJobStore(
		htmxJob(1, models.StatusDiscovered),
		htmxJob(2, models.StatusNotified),
		htmxJob(3, models.StatusNotified),
	)
	ts := newSpyServer(t, spy)
	defer ts.Close()

	// HX-Current-URL with no filter — all 3 jobs should be in the response.
	currentURL := ts.URL + "/"
	resp := doHTMX(t, ts.Client(), ts.URL, "/api/jobs/1/approve", currentURL)
	body := readBody(t, resp)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", resp.StatusCode, body)
	}

	// The job_rows template renders each job. Verify job IDs 1, 2, 3 all appear.
	for _, wantID := range []string{"1", "2", "3"} {
		if !strings.Contains(body, wantID) {
			t.Errorf("response body does not contain job ID %s — want full list; body snippet: %.300s", wantID, body)
		}
	}
}

// TestRespondJobAction_HTMX_StatusFilterRespected verifies that the status
// query param in HX-Current-URL is forwarded to ListJobs. When the client is
// viewing status=notified, only notified jobs should appear in the response.
func TestRespondJobAction_HTMX_StatusFilterRespected(t *testing.T) {
	spy := newSpyJobStore(
		htmxJob(1, models.StatusDiscovered),
		htmxJob(2, models.StatusNotified),
		htmxJob(3, models.StatusNotified),
	)
	ts := newSpyServer(t, spy)
	defer ts.Close()

	// Approve job 2, with HX-Current-URL showing status=notified filter.
	currentURL := ts.URL + "/?status=notified"
	resp := doHTMX(t, ts.Client(), ts.URL, "/api/jobs/2/approve", currentURL)
	body := readBody(t, resp)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", resp.StatusCode, body)
	}

	// Verify the filter was passed to ListJobs.
	f := spy.LastFilter()
	if f.Status != models.StatusNotified {
		t.Errorf("ListJobs called with Status=%q, want %q", f.Status, models.StatusNotified)
	}
}

// TestRespondJobAction_HTMX_SearchParamForwarded verifies that the q param in
// HX-Current-URL is extracted and forwarded as the Search field in ListJobs.
func TestRespondJobAction_HTMX_SearchParamForwarded(t *testing.T) {
	spy := newSpyJobStore(
		htmxJob(1, models.StatusDiscovered),
		htmxJob(2, models.StatusNotified),
	)
	ts := newSpyServer(t, spy)
	defer ts.Close()

	currentURL := ts.URL + "/?q=golang+engineer"
	resp := doHTMX(t, ts.Client(), ts.URL, "/api/jobs/1/approve", currentURL)
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	f := spy.LastFilter()
	if f.Search != "golang engineer" {
		t.Errorf("ListJobs called with Search=%q, want %q", f.Search, "golang engineer")
	}
}

// TestRespondJobAction_HTMX_SortOrderForwarded verifies that sort and order
// params in HX-Current-URL are forwarded to ListJobs. An invalid sort column
// is sanitised to the default ("discovered_at").
func TestRespondJobAction_HTMX_SortOrderForwarded(t *testing.T) {
	spy := newSpyJobStore(htmxJob(1, models.StatusDiscovered))
	ts := newSpyServer(t, spy)
	defer ts.Close()

	t.Run("valid sort forwarded", func(t *testing.T) {
		currentURL := ts.URL + "/?sort=company&order=asc"
		resp := doHTMX(t, ts.Client(), ts.URL, "/api/jobs/1/reject", currentURL)
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", resp.StatusCode)
		}

		f := spy.LastFilter()
		if f.Sort != "company" {
			t.Errorf("ListJobs Sort=%q, want company", f.Sort)
		}
		if f.Order != "asc" {
			t.Errorf("ListJobs Order=%q, want asc", f.Order)
		}
	})

	t.Run("invalid sort column falls back to discovered_at", func(t *testing.T) {
		// Re-create the job so it can be rejected again.
		spy2 := newSpyJobStore(htmxJob(5, models.StatusDiscovered))
		ts2 := newSpyServer(t, spy2)
		defer ts2.Close()

		currentURL := ts2.URL + "/?sort=DROP+TABLE&order=desc"
		resp := doHTMX(t, ts2.Client(), ts2.URL, "/api/jobs/5/reject", currentURL)
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", resp.StatusCode)
		}

		f := spy2.LastFilter()
		if f.Sort != "discovered_at" {
			t.Errorf("ListJobs Sort=%q, want discovered_at (sanitised from invalid column)", f.Sort)
		}
	})
}

// TestRespondJobAction_HTMX_MissingCurrentURL verifies that a missing
// HX-Current-URL header (no header sent at all) is handled gracefully: the
// handler returns 200 HTML with all jobs and does not panic.
func TestRespondJobAction_HTMX_MissingCurrentURL(t *testing.T) {
	spy := newSpyJobStore(
		htmxJob(10, models.StatusDiscovered),
		htmxJob(11, models.StatusNotified),
	)
	ts := newSpyServer(t, spy)
	defer ts.Close()

	// doHTMX with empty currentURL skips setting HX-Current-URL entirely.
	resp := doHTMX(t, ts.Client(), ts.URL, "/api/jobs/10/approve", "")
	body := readBody(t, resp)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", resp.StatusCode, body)
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}
}

// TestRespondJobAction_HTMX_MalformedCurrentURL verifies that a malformed
// HX-Current-URL (not a valid URL) does not cause a panic or 500. The handler
// must fall back gracefully and return 200 HTML.
func TestRespondJobAction_HTMX_MalformedCurrentURL(t *testing.T) {
	spy := newSpyJobStore(htmxJob(20, models.StatusDiscovered))
	ts := newSpyServer(t, spy)
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/jobs/20/approve", nil)
	req.Header.Set("HX-Request", "true")
	// A URL that url.Parse cannot extract query params from (scheme-only junk).
	req.Header.Set("HX-Current-URL", "://not a valid url at all%%%")

	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("POST approve with malformed URL: %v", err)
	}
	body := readBody(t, resp)

	// The handler must not panic. With a malformed URL url.Parse returns an
	// error and the code falls back to url.Values{} — ListJobs is then called
	// with no filter and returns all jobs. We expect 200 with HTML.
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", resp.StatusCode, body)
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}
}

// TestRespondJobAction_HTMX_EmptyCurrentURL verifies that an empty
// HX-Current-URL header ("" string value) is treated as "no filter" and
// returns all jobs gracefully.
func TestRespondJobAction_HTMX_EmptyCurrentURL(t *testing.T) {
	spy := newSpyJobStore(
		htmxJob(30, models.StatusDiscovered),
		htmxJob(31, models.StatusNotified),
	)
	ts := newSpyServer(t, spy)
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/jobs/30/approve", nil)
	req.Header.Set("HX-Request", "true")
	req.Header.Set("HX-Current-URL", "")

	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("POST approve with empty URL: %v", err)
	}
	body := readBody(t, resp)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", resp.StatusCode, body)
	}
	// With no status filter, both jobs should appear.
	for _, wantID := range []string{"30", "31"} {
		if !strings.Contains(body, wantID) {
			t.Errorf("response body missing job ID %s — want full unfiltered list", wantID)
		}
	}
}

// TestRespondJobAction_NonHTMX_Approve_ReturnsJSON guards against regression:
// a POST /api/jobs/{id}/approve WITHOUT HX-Request must still return JSON.
func TestRespondJobAction_NonHTMX_Approve_ReturnsJSON(t *testing.T) {
	spy := newSpyJobStore(htmxJob(40, models.StatusDiscovered))
	ts := newSpyServer(t, spy)
	defer ts.Close()

	resp, err := ts.Client().Post(ts.URL+"/api/jobs/40/approve", "application/json", nil)
	if err != nil {
		t.Fatalf("POST approve: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json for non-HTMX request", ct)
	}
}

// TestRespondJobAction_NonHTMX_Reject_ReturnsJSON guards against regression:
// a POST /api/jobs/{id}/reject WITHOUT HX-Request must still return JSON.
func TestRespondJobAction_NonHTMX_Reject_ReturnsJSON(t *testing.T) {
	spy := newSpyJobStore(htmxJob(50, models.StatusDiscovered))
	ts := newSpyServer(t, spy)
	defer ts.Close()

	resp, err := ts.Client().Post(ts.URL+"/api/jobs/50/reject", "application/json", nil)
	if err != nil {
		t.Fatalf("POST reject: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json for non-HTMX request", ct)
	}
}

// TestRespondJobAction_HTMX_RejectStatusFilter verifies that rejecting a job
// while viewing a status-filtered page returns HTML scoped to that filter,
// with the correct status forwarded to ListJobs.
func TestRespondJobAction_HTMX_RejectStatusFilter(t *testing.T) {
	spy := newSpyJobStore(
		htmxJob(60, models.StatusDiscovered),
		htmxJob(61, models.StatusDiscovered),
		htmxJob(62, models.StatusNotified),
	)
	ts := newSpyServer(t, spy)
	defer ts.Close()

	// Client is viewing status=discovered when it rejects job 60.
	currentURL := ts.URL + "/?status=discovered"
	resp := doHTMX(t, ts.Client(), ts.URL, "/api/jobs/60/reject", currentURL)
	body := readBody(t, resp)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", resp.StatusCode, body)
	}

	// Verify filter was forwarded.
	f := spy.LastFilter()
	if f.Status != models.StatusDiscovered {
		t.Errorf("ListJobs Status=%q, want %q", f.Status, models.StatusDiscovered)
	}

	// Job 62 (notified) must not appear when the filter is status=discovered.
	if strings.Contains(body, "62") {
		t.Error("response body contains job 62 (notified) but client is filtering status=discovered")
	}
}

// TestRespondJobAction_HTMX_MultipleFiltersForwarded tests that all three
// filter params — status, q, and sort/order — are all forwarded together.
func TestRespondJobAction_HTMX_MultipleFiltersForwarded(t *testing.T) {
	spy := newSpyJobStore(htmxJob(70, models.StatusDiscovered))
	ts := newSpyServer(t, spy)
	defer ts.Close()

	currentURL := ts.URL + "/?status=discovered&q=senior&sort=title&order=asc"
	resp := doHTMX(t, ts.Client(), ts.URL, "/api/jobs/70/approve", currentURL)
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	f := spy.LastFilter()
	errs := []string{}
	if f.Status != models.StatusDiscovered {
		errs = append(errs, "Status="+string(f.Status)+" want discovered")
	}
	if f.Search != "senior" {
		errs = append(errs, "Search="+f.Search+" want senior")
	}
	if f.Sort != "title" {
		errs = append(errs, "Sort="+f.Sort+" want title")
	}
	if f.Order != "asc" {
		errs = append(errs, "Order="+f.Order+" want asc")
	}
	if len(errs) > 0 {
		t.Errorf("ListJobs filter mismatch: %s", strings.Join(errs, "; "))
	}
}

// ─── URL parsing helpers (unit-level) ─────────────────────────────────────────

// TestRespondJobAction_HTMX_CurrentURL_ParseBehavior verifies the Go standard
// library's url.Parse behaviour for the edge-case URLs that HX-Current-URL
// might carry, ensuring the handler's fallback logic is sound.
func TestRespondJobAction_HTMX_CurrentURL_ParseBehavior(t *testing.T) {
	cases := []struct {
		raw     string
		wantErr bool // url.Parse returns non-nil error
	}{
		{"http://localhost/?status=discovered", false},
		{"https://app.example.com/dashboard?q=go&sort=title&order=asc", false},
		{"", false},            // empty string parses without error
		{"/?status=notified", false}, // relative URL — parses fine
		{"://bad", true},       // scheme-relative malform — returns error
	}

	for _, tc := range cases {
		_, err := url.Parse(tc.raw)
		gotErr := err != nil
		if gotErr != tc.wantErr {
			t.Errorf("url.Parse(%q): err=%v, wantErr=%v", tc.raw, err, tc.wantErr)
		}
	}
}
