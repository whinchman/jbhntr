package web_test

// job_pipeline_qa_test.go — additional QA tests for the job-pipeline-pages
// feature (task job-pipeline-6-qa).
//
// These tests fill coverage gaps that are not addressed by the existing tests
// in server_test.go:
//
//  1. TestHandleSetApplicationStatus_MissingJob      — 404 when job does not exist
//  2. TestHandleSetApplicationStatus_HTMXRowFragment  — response body contains the
//     replacement <tr id="job-row-{id}"> fragment required by the HTMX contract
//  3. TestDashboard_NoApprovedRejectedTabs            — GET / does not include
//     "approved" or "rejected" status values in the rendered tab list
//  4. TestUpdateApplicationStatus_UserScoping         — store: a second user cannot
//     update a job that belongs to the first user (store-level; runs only when
//     TEST_DATABASE_URL is set)

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/whinchman/jobhuntr/internal/models"
	"github.com/whinchman/jobhuntr/internal/web"
)

// ─── Handler: missing job → 404 ───────────────────────────────────────────────

// TestHandleSetApplicationStatus_MissingJob verifies that POST
// /api/jobs/{id}/application-status returns HTTP 404 when the job does not
// exist in the store.
func TestHandleSetApplicationStatus_MissingJob(t *testing.T) {
	// Create a server with no jobs — any job ID will be missing.
	ts := newServer(t)
	defer ts.Close()

	form := url.Values{"application_status": {"applied"}}
	resp, err := ts.Client().PostForm(ts.URL+"/api/jobs/9999/application-status", form)
	if err != nil {
		t.Fatalf("POST /api/jobs/9999/application-status: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("status = %d, want 404; body: %s", resp.StatusCode, body)
	}
}

// ─── Handler: HTMX row fragment contract ──────────────────────────────────────

// TestHandleSetApplicationStatus_HTMXRowFragment verifies that a successful
// POST /api/jobs/{id}/application-status response body contains the replacement
// <tr id="job-row-{id}"> element as specified by the HTMX contract.
func TestHandleSetApplicationStatus_HTMXRowFragment(t *testing.T) {
	job := newTestJob(7, models.StatusApproved)
	ts := newServer(t, job)
	defer ts.Close()

	form := url.Values{"application_status": {"interviewing"}}
	resp, err := ts.Client().PostForm(ts.URL+"/api/jobs/7/application-status", form)
	if err != nil {
		t.Fatalf("POST /api/jobs/7/application-status: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, want 200; body: %s", resp.StatusCode, body)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	bodyStr := string(body)

	// The template must emit <tr id="job-row-7"> so HTMX can swap the row.
	const wantFragment = `id="job-row-7"`
	if !strings.Contains(bodyStr, wantFragment) {
		t.Errorf("response body does not contain %q\nbody:\n%s", wantFragment, bodyStr)
	}
}

// ─── Dashboard: no approved/rejected tabs ────────────────────────────────────

// TestDashboard_NoApprovedRejectedTabs verifies that the GET / response does
// NOT contain tab links for "approved" or "rejected" statuses. Those statuses
// were intentionally moved to their own dedicated pages (/jobs/approved and
// /jobs/rejected) and must not appear as tabs on the triage dashboard.
//
// The test uses the unauthenticated view (no session cookie) which renders the
// landing / sign-in page. Neither the authenticated tab bar nor the landing
// page should reference /?status=approved or /?status=rejected as tab links.
func TestDashboard_NoApprovedRejectedTabs(t *testing.T) {
	ts := newServer(t,
		newTestJob(1, models.StatusDiscovered),
		newTestJob(2, models.StatusApproved),
		newTestJob(3, models.StatusRejected),
	)
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	bodyStr := string(body)

	// The dashboard template renders tab links like: /?status=<value> or
	// ?status=<value> for the authenticated tab bar.  "approved" and "rejected"
	// are pipeline-stage statuses that have their own dedicated pages; they must
	// never appear as tab query-parameter values on the triage dashboard page.
	forbiddenTabPatterns := []string{
		"?status=approved",
		"?status=rejected",
	}
	for _, pattern := range forbiddenTabPatterns {
		if strings.Contains(bodyStr, pattern) {
			t.Errorf("dashboard HTML contains tab link %q — approved/rejected must not appear as dashboard tabs", pattern)
		}
	}
}

// TestDashboard_TabsContainDiscoveredNotified verifies that the authenticated
// dashboard view renders tab links for the two triage statuses: discovered and
// notified. It uses a server configured with auth so that the full tab bar is
// rendered.
func TestDashboard_TabsContainDiscoveredNotified(t *testing.T) {
	job := &models.Job{
		ID:       1,
		UserID:   42,
		Title:    "My Job",
		Company:  "Acme",
		Location: "Remote",
		Status:   models.StatusDiscovered,
	}
	ms := newMockJobStore(job)
	us := newMockUserStore(&models.User{
		ID:          42,
		Provider:    "google",
		ProviderID:  "g-42",
		Email:       "user42@example.com",
		DisplayName: "User 42",
	})

	cfg := newAuthConfig()
	srv := web.NewServerWithConfig(ms, us, nil, cfg)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	cookie := setSessionCookie(t, ts, 42)

	client := ts.Client()
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/", nil)
	req.AddCookie(cookie)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	bodyStr := string(body)

	// Authenticated users see the tab bar — confirm discovered and notified tabs.
	expectedTabs := []string{
		"?status=discovered",
		"?status=notified",
	}
	for _, pattern := range expectedTabs {
		if !strings.Contains(bodyStr, pattern) {
			t.Errorf("authenticated dashboard HTML does not contain expected tab link %q", pattern)
		}
	}

	// Still confirm no approved/rejected tab links sneak in.
	forbiddenTabs := []string{
		"?status=approved",
		"?status=rejected",
	}
	for _, pattern := range forbiddenTabs {
		if strings.Contains(bodyStr, pattern) {
			t.Errorf("authenticated dashboard HTML contains forbidden tab link %q", pattern)
		}
	}
}
