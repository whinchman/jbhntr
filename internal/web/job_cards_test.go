package web_test

// job_cards_test.go — tests for the tinder-style mobile card deck backend:
//   - GET /partials/job-cards handler
//   - respondJobAction with HX-Target: job-card-deck

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/whinchman/jobhuntr/internal/models"
	"github.com/whinchman/jobhuntr/internal/web"
)

// ─── helpers ─────────────────────────────────────────────────────────────────

// newCardDeckServer creates a test server with a spy store pre-loaded with jobs.
func newCardDeckServer(t *testing.T, jobs ...*models.Job) (*httptest.Server, *spyJobStore) {
	t.Helper()
	spy := newSpyJobStore(jobs...)
	srv := web.NewServer(spy)
	ts := httptest.NewServer(srv.Handler())
	return ts, spy
}

// doHTMXWithTarget issues a POST with HX-Request and an explicit HX-Target header.
func doHTMXWithTarget(t *testing.T, client *http.Client, url, hxTarget string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("HX-Request", "true")
	req.Header.Set("HX-Target", hxTarget)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	return resp
}

// ─── GET /partials/job-cards ──────────────────────────────────────────────────

// TestHandleJobCardsPartial_Unauthenticated verifies that an unauthenticated
// request to /partials/job-cards returns 200 with an empty body (no redirect).
func TestHandleJobCardsPartial_Unauthenticated(t *testing.T) {
	ts, _ := newCardDeckServer(t,
		htmxJob(1, models.StatusDiscovered),
		htmxJob(2, models.StatusNotified),
	)
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/partials/job-cards")
	if err != nil {
		t.Fatalf("GET /partials/job-cards: %v", err)
	}
	body := readBody(t, resp)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", resp.StatusCode, body)
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}
	// Unauthenticated response must be empty (no job cards rendered).
	if strings.TrimSpace(body) != "" {
		t.Errorf("expected empty body for unauthenticated request, got: %.200s", body)
	}
}

// TestHandleJobCardsPartial_RouteRegistered verifies that the route
// /partials/job-cards is registered and returns 200, not 404.
func TestHandleJobCardsPartial_RouteRegistered(t *testing.T) {
	ts, _ := newCardDeckServer(t)
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/partials/job-cards")
	if err != nil {
		t.Fatalf("GET /partials/job-cards: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		t.Errorf("GET /partials/job-cards returned 404 — route not registered")
	}
}

// ─── respondJobAction with HX-Target: job-card-deck ─────────────────────────

// TestRespondJobAction_CardDeck_Approve_ReturnsHTML verifies that approving a
// job with HX-Target: job-card-deck returns 200 HTML (not JSON).
func TestRespondJobAction_CardDeck_Approve_ReturnsHTML(t *testing.T) {
	ts, _ := newCardDeckServer(t,
		htmxJob(1, models.StatusDiscovered),
		htmxJob(2, models.StatusNotified),
	)
	defer ts.Close()

	resp := doHTMXWithTarget(t, ts.Client(), ts.URL+"/api/jobs/1/approve", "job-card-deck")
	body := readBody(t, resp)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", resp.StatusCode, body)
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}
}

// TestRespondJobAction_CardDeck_Reject_ReturnsHTML verifies that rejecting a
// job with HX-Target: job-card-deck returns 200 HTML (not JSON).
func TestRespondJobAction_CardDeck_Reject_ReturnsHTML(t *testing.T) {
	ts, _ := newCardDeckServer(t,
		htmxJob(1, models.StatusDiscovered),
	)
	defer ts.Close()

	resp := doHTMXWithTarget(t, ts.Client(), ts.URL+"/api/jobs/1/reject", "job-card-deck")
	body := readBody(t, resp)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", resp.StatusCode, body)
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}
}

// TestRespondJobAction_CardDeck_ExcludeStatusesSetInFilter verifies that when
// HX-Target is "job-card-deck", the filter passed to ListJobs includes
// ExcludeStatuses = [rejected].
func TestRespondJobAction_CardDeck_ExcludeStatusesSetInFilter(t *testing.T) {
	spy := newSpyJobStore(
		htmxJob(1, models.StatusDiscovered),
		htmxJob(2, models.StatusNotified),
	)
	ts := newSpyServer(t, spy)
	defer ts.Close()

	resp := doHTMXWithTarget(t, ts.Client(), ts.URL+"/api/jobs/1/approve", "job-card-deck")
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	f := spy.LastFilter()
	if len(f.ExcludeStatuses) == 0 {
		t.Error("ExcludeStatuses is empty — expected [rejected] for card-deck HX-Target")
		return
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

// TestRespondJobAction_JobTable_NotAffected_Regression guards against regression:
// approving a job with HX-Target: job-table-body must still render "job_rows"
// (not "job_cards") and must not include ExcludeStatuses in the filter.
func TestRespondJobAction_JobTable_NotAffected_Regression(t *testing.T) {
	spy := newSpyJobStore(
		htmxJob(1, models.StatusDiscovered),
		htmxJob(2, models.StatusNotified),
	)
	ts := newSpyServer(t, spy)
	defer ts.Close()

	resp := doHTMXWithTarget(t, ts.Client(), ts.URL+"/api/jobs/1/approve", "job-table-body")
	body := readBody(t, resp)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", resp.StatusCode, body)
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}

	// The job_rows template renders <tr> elements; job_cards renders <div class="job-card">.
	// For the table target, we must NOT see job-card divs.
	if strings.Contains(body, `class="job-card"`) {
		t.Error("job-table-body target incorrectly rendered job_cards template — regression")
	}

	// Filter should NOT have ExcludeStatuses set for non-card-deck targets.
	f := spy.LastFilter()
	if len(f.ExcludeStatuses) > 0 {
		t.Errorf("ExcludeStatuses = %v for job-table-body target, want empty (no exclusion)", f.ExcludeStatuses)
	}
}

// TestRespondJobAction_CardDeck_RendersJobCardsTemplate verifies that the
// job_cards template (not job_rows) is rendered when HX-Target is job-card-deck.
// The job_cards template renders elements with class "job-card".
func TestRespondJobAction_CardDeck_RendersJobCardsTemplate(t *testing.T) {
	ts, _ := newCardDeckServer(t,
		htmxJob(1, models.StatusDiscovered),
	)
	defer ts.Close()

	resp := doHTMXWithTarget(t, ts.Client(), ts.URL+"/api/jobs/1/approve", "job-card-deck")
	body := readBody(t, resp)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", resp.StatusCode, body)
	}

	// The job_cards template renders <div class="job-card"> for each job.
	// The job_rows template renders <tr> elements.
	// Verify we see job-card markup (not table rows), confirming the right template ran.
	if !strings.Contains(body, "job-card") {
		t.Errorf("response does not contain 'job-card' markup — job_cards template may not have been executed; body: %.400s", body)
	}
	if strings.Contains(body, "<tr>") {
		t.Error("response contains <tr> — job_rows template was rendered instead of job_cards")
	}
}

// TestRespondJobAction_CardDeck_Reject_RendersJobCardsTemplate verifies that
// rejecting a job with HX-Target: job-card-deck renders the job_cards template
// (not job_rows). This is the parallel of the approve test above for the reject
// action, covering Acceptance Criterion 5 from tinder-mobile-qa.md.
func TestRespondJobAction_CardDeck_Reject_RendersJobCardsTemplate(t *testing.T) {
	ts, _ := newCardDeckServer(t,
		htmxJob(1, models.StatusDiscovered),
		htmxJob(2, models.StatusNotified),
	)
	defer ts.Close()

	resp := doHTMXWithTarget(t, ts.Client(), ts.URL+"/api/jobs/1/reject", "job-card-deck")
	body := readBody(t, resp)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", resp.StatusCode, body)
	}

	// job_cards template must be rendered (not job_rows).
	if !strings.Contains(body, "job-card") {
		t.Errorf("response does not contain 'job-card' markup after reject — job_cards template not executed; body: %.400s", body)
	}
	if strings.Contains(body, "<tr>") {
		t.Error("response contains <tr> after reject with card-deck target — job_rows rendered instead of job_cards")
	}
}

// TestHandleJobCardsPartial_ContentType verifies that the Content-Type header
// for GET /partials/job-cards is text/html (unauthenticated path). This
// supplements TestHandleJobCardsPartial_Unauthenticated which already checks
// the body; this test explicitly asserts the Content-Type header is set
// regardless of authentication state.
func TestHandleJobCardsPartial_ContentType(t *testing.T) {
	ts, _ := newCardDeckServer(t)
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/partials/job-cards")
	if err != nil {
		t.Fatalf("GET /partials/job-cards: %v", err)
	}
	resp.Body.Close()

	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}
}
