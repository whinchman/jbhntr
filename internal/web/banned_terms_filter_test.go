package web_test

// banned_terms_filter_test.go — QA tests for the filterStore != nil code path
// in handleJobTablePartial and respondJobAction (fix-bug-009-banned-terms-partial).
//
// The bug: both handlers built a ListJobsFilter with BannedTerms empty, so jobs
// matching the user's banned terms could appear in the table.
//
// The fix (commit 931219a) adds a guarded call to
// s.filterStore.ListUserBannedTerms in both handlers, mirroring handleDashboard.
// On error, both handlers log a warning and continue with an empty slice
// (graceful degradation) instead of returning 500.
//
// Test coverage added here:
//  1. handleJobTablePartial with filterStore: BannedTerms is populated in ListJobs.
//  2. handleJobTablePartial with filterStore error: handler returns 200, no 500.
//  3. respondJobAction HTMX path with filterStore: BannedTerms is populated in ListJobs.
//  4. respondJobAction HTMX path with filterStore error: handler returns 200, no 500.

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/whinchman/jobhuntr/internal/models"
	"github.com/whinchman/jobhuntr/internal/web"
)

// ─── errFilterStore ──────────────────────────────────────────────────────────

// errFilterStore is a FilterStore whose ListUserBannedTerms always returns an
// error.  All other methods are no-ops that succeed so settings routes work
// if ever exercised during test teardown.
type errFilterStore struct{}

func (e *errFilterStore) ListUserBannedTerms(_ context.Context, _ int64) ([]models.UserBannedTerm, error) {
	return nil, fmt.Errorf("db: simulated ListUserBannedTerms failure")
}

func (e *errFilterStore) CreateUserFilter(_ context.Context, _ int64, _ *models.UserSearchFilter) error {
	return nil
}

func (e *errFilterStore) ListUserFilters(_ context.Context, _ int64) ([]models.UserSearchFilter, error) {
	return nil, nil
}

func (e *errFilterStore) DeleteUserFilter(_ context.Context, _ int64, _ int64) error {
	return nil
}

func (e *errFilterStore) UpdateUserResume(_ context.Context, _ int64, _ string) error {
	return nil
}

func (e *errFilterStore) UpdateUserNtfyTopic(_ context.Context, _ int64, _ string) error {
	return nil
}

func (e *errFilterStore) CreateUserBannedTerm(_ context.Context, _ int64, _ string) (*models.UserBannedTerm, error) {
	return nil, nil
}

func (e *errFilterStore) DeleteUserBannedTerm(_ context.Context, _ int64, _ int64) error {
	return nil
}

// ─── helpers ─────────────────────────────────────────────────────────────────

// newFilterSpyServer creates a test server wired with auth (so optionalAuth /
// requireAuth middleware can inject a user into context), a spyJobStore
// (so we can inspect the BannedTerms sent to ListJobs), and a
// mockFilterStore pre-seeded with the given banned terms for userID 42.
//
// Returns:
//   - the httptest.Server (caller must defer ts.Close())
//   - the spy job store (for asserting LastFilter().BannedTerms)
//   - a session cookie for user 42 (inject into every request)
func newFilterSpyServer(t *testing.T, bannedTerms []string, jobs ...*models.Job) (*httptest.Server, *spyJobStore, *http.Cookie) {
	t.Helper()

	spy := newSpyJobStore(jobs...)

	// Build a filter store pre-seeded with the user's banned terms.
	fs := newMockFilterStore()
	for _, term := range bannedTerms {
		if err := fs.CreateUserBannedTerm(context.Background(), 42, term); err != nil {
			t.Fatalf("seed banned term %q: %v", term, err)
		}
	}

	us := newMockUserStore(&models.User{
		ID:                 42,
		Email:              "user42@example.com",
		Provider:           "google",
		ProviderID:         "g-42",
		DisplayName:        "User 42",
		OnboardingComplete: true,
	})

	cfg := newAuthConfig()
	srv := web.NewServerWithConfig(spy, us, fs, cfg)
	ts := httptest.NewServer(srv.Handler())

	cookie := setSessionCookie(t, ts, 42)
	return ts, spy, cookie
}

// newErrFilterSpyServer creates a test server wired with auth and an
// errFilterStore (ListUserBannedTerms always fails).  Returns the server, the
// spy job store, and a session cookie for user 42.
func newErrFilterSpyServer(t *testing.T, jobs ...*models.Job) (*httptest.Server, *spyJobStore, *http.Cookie) {
	t.Helper()

	spy := newSpyJobStore(jobs...)

	us := newMockUserStore(&models.User{
		ID:                 42,
		Email:              "user42@example.com",
		Provider:           "google",
		ProviderID:         "g-42",
		DisplayName:        "User 42",
		OnboardingComplete: true,
	})

	cfg := newAuthConfig()
	srv := web.NewServerWithConfig(spy, us, &errFilterStore{}, cfg)
	ts := httptest.NewServer(srv.Handler())

	cookie := setSessionCookie(t, ts, 42)
	return ts, spy, cookie
}

// job42 creates a test job belonging to userID 42.
func job42(id int64, status models.JobStatus) *models.Job {
	j := newTestJob(id, status)
	j.UserID = 42
	return j
}

// doGetWithCookie issues a GET request to path with the given session cookie.
func doGetWithCookie(t *testing.T, client *http.Client, baseURL, path string, cookie *http.Cookie) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, baseURL+path, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.AddCookie(cookie)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	return resp
}

// doHTMXWithCookie issues a POST with HX-Request:true, an optional
// HX-Current-URL header, and the given session cookie.
func doHTMXWithCookie(t *testing.T, client *http.Client, baseURL, path, currentURL string, cookie *http.Cookie) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, baseURL+path, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("HX-Request", "true")
	if currentURL != "" {
		req.Header.Set("HX-Current-URL", currentURL)
	}
	req.AddCookie(cookie)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	return resp
}

// ─── handleJobTablePartial with filterStore ───────────────────────────────────

// TestHandleJobTablePartial_FilterStore_BannedTermsPopulated verifies that
// when a filterStore is wired and the user has banned terms, those terms are
// forwarded as BannedTerms in the ListJobs filter.
func TestHandleJobTablePartial_FilterStore_BannedTermsPopulated(t *testing.T) {
	ts, spy, cookie := newFilterSpyServer(t,
		[]string{"ContractOnly", "Unpaid"},
		job42(1, models.StatusDiscovered),
		job42(2, models.StatusNotified),
	)
	defer ts.Close()

	resp := doGetWithCookie(t, ts.Client(), ts.URL, "/partials/job-table", cookie)
	body := readBody(t, resp)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /partials/job-table: status = %d, want 200; body: %s", resp.StatusCode, body)
	}

	f := spy.LastFilter()
	if len(f.BannedTerms) != 2 {
		t.Errorf("ListJobs called with len(BannedTerms) = %d, want 2; got %v", len(f.BannedTerms), f.BannedTerms)
	}
	// Verify both terms are present (order may vary).
	termSet := make(map[string]bool, len(f.BannedTerms))
	for _, term := range f.BannedTerms {
		termSet[term] = true
	}
	for _, want := range []string{"ContractOnly", "Unpaid"} {
		if !termSet[want] {
			t.Errorf("BannedTerms does not contain %q; got %v", want, f.BannedTerms)
		}
	}
}

// TestHandleJobTablePartial_FilterStore_SingleTerm verifies that a single
// banned term is correctly forwarded.
func TestHandleJobTablePartial_FilterStore_SingleTerm(t *testing.T) {
	ts, spy, cookie := newFilterSpyServer(t,
		[]string{"Intern"},
		job42(10, models.StatusDiscovered),
	)
	defer ts.Close()

	resp := doGetWithCookie(t, ts.Client(), ts.URL, "/partials/job-table", cookie)
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	f := spy.LastFilter()
	if len(f.BannedTerms) != 1 || f.BannedTerms[0] != "Intern" {
		t.Errorf("BannedTerms = %v, want [Intern]", f.BannedTerms)
	}
}

// TestHandleJobTablePartial_FilterStore_NoBannedTerms verifies that when the
// user has no banned terms, BannedTerms is empty (no panic, no 500).
func TestHandleJobTablePartial_FilterStore_NoBannedTerms(t *testing.T) {
	ts, spy, cookie := newFilterSpyServer(t,
		[]string{}, // no banned terms seeded
		job42(20, models.StatusDiscovered),
	)
	defer ts.Close()

	resp := doGetWithCookie(t, ts.Client(), ts.URL, "/partials/job-table", cookie)
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	f := spy.LastFilter()
	if len(f.BannedTerms) != 0 {
		t.Errorf("BannedTerms = %v, want empty", f.BannedTerms)
	}
}

// TestHandleJobTablePartial_FilterStore_ErrorDegrades verifies that when
// ListUserBannedTerms returns an error, handleJobTablePartial does NOT return
// 500 — it logs the warning and continues, returning 200 HTML with an empty
// BannedTerms slice (graceful degradation).
func TestHandleJobTablePartial_FilterStore_ErrorDegrades(t *testing.T) {
	ts, spy, cookie := newErrFilterSpyServer(t, job42(30, models.StatusDiscovered))
	defer ts.Close()

	resp := doGetWithCookie(t, ts.Client(), ts.URL, "/partials/job-table", cookie)
	body := readBody(t, resp)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /partials/job-table with errFilterStore: status = %d, want 200 (graceful degradation); body: %s", resp.StatusCode, body)
	}

	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}

	// BannedTerms must be empty (error degraded gracefully).
	f := spy.LastFilter()
	if len(f.BannedTerms) != 0 {
		t.Errorf("BannedTerms = %v, want empty on error degradation", f.BannedTerms)
	}
}

// ─── respondJobAction HTMX path with filterStore ─────────────────────────────

// TestRespondJobAction_FilterStore_BannedTermsPopulated verifies that in the
// HTMX branch of respondJobAction, when a filterStore is wired, the user's
// banned terms are forwarded as BannedTerms in the ListJobs re-query.
func TestRespondJobAction_FilterStore_BannedTermsPopulated(t *testing.T) {
	ts, spy, cookie := newFilterSpyServer(t,
		[]string{"Crypto", "NFT"},
		job42(100, models.StatusDiscovered),
		job42(101, models.StatusNotified),
	)
	defer ts.Close()

	currentURL := ts.URL + "/"
	resp := doHTMXWithCookie(t, ts.Client(), ts.URL, "/api/jobs/100/approve", currentURL, cookie)
	body := readBody(t, resp)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/jobs/100/approve (HTMX): status = %d, want 200; body: %s", resp.StatusCode, body)
	}

	f := spy.LastFilter()
	if len(f.BannedTerms) != 2 {
		t.Errorf("ListJobs called with len(BannedTerms) = %d, want 2; got %v", len(f.BannedTerms), f.BannedTerms)
	}
	termSet := make(map[string]bool, len(f.BannedTerms))
	for _, term := range f.BannedTerms {
		termSet[term] = true
	}
	for _, want := range []string{"Crypto", "NFT"} {
		if !termSet[want] {
			t.Errorf("BannedTerms does not contain %q; got %v", want, f.BannedTerms)
		}
	}
}

// TestRespondJobAction_FilterStore_Reject_BannedTermsPopulated verifies the
// same banned-terms population for the reject action.
func TestRespondJobAction_FilterStore_Reject_BannedTermsPopulated(t *testing.T) {
	ts, spy, cookie := newFilterSpyServer(t,
		[]string{"NoRemote"},
		job42(110, models.StatusDiscovered),
	)
	defer ts.Close()

	currentURL := ts.URL + "/?status=discovered"
	resp := doHTMXWithCookie(t, ts.Client(), ts.URL, "/api/jobs/110/reject", currentURL, cookie)
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/jobs/110/reject (HTMX): status = %d, want 200", resp.StatusCode)
	}

	f := spy.LastFilter()
	if len(f.BannedTerms) != 1 || f.BannedTerms[0] != "NoRemote" {
		t.Errorf("BannedTerms = %v, want [NoRemote]", f.BannedTerms)
	}
}

// TestRespondJobAction_FilterStore_ErrorDegrades verifies that when
// ListUserBannedTerms returns an error, respondJobAction does NOT return 500
// in the HTMX path — it logs the warning and continues with an empty
// BannedTerms slice (graceful degradation), returning 200 HTML.
func TestRespondJobAction_FilterStore_ErrorDegrades(t *testing.T) {
	ts, spy, cookie := newErrFilterSpyServer(t, job42(120, models.StatusDiscovered))
	defer ts.Close()

	currentURL := ts.URL + "/"
	resp := doHTMXWithCookie(t, ts.Client(), ts.URL, "/api/jobs/120/approve", currentURL, cookie)
	body := readBody(t, resp)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/jobs/120/approve (HTMX) with errFilterStore: status = %d, want 200 (graceful degradation); body: %s", resp.StatusCode, body)
	}

	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}

	// BannedTerms must be empty — error degraded gracefully, not a 500.
	f := spy.LastFilter()
	if len(f.BannedTerms) != 0 {
		t.Errorf("BannedTerms = %v, want empty on error degradation", f.BannedTerms)
	}
}
