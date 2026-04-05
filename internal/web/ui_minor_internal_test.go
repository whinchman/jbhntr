// NOTE: Go is not installed in this container. These tests require a Docker
// build (or a local Go toolchain) to execute:
//
//	docker build -t jobhuntr . && docker run --rm jobhuntr go test ./internal/web/...
//
// Run from the repo root. All tests should pass with no regressions.
//
// This file is in package web (internal) so it can access unexported symbols
// (userContextKey) needed to inject an authenticated user into the request
// context for dashboard template tests.

package web

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/whinchman/jobhuntr/internal/models"
	"github.com/whinchman/jobhuntr/internal/store"
)

// ─── minimal mock JobStore ────────────────────────────────────────────────────

type uiMinorJobStore struct {
	mu   sync.Mutex
	jobs []*models.Job
}

func (m *uiMinorJobStore) GetJob(_ context.Context, _ int64, id int64) (*models.Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, j := range m.jobs {
		if j.ID == id {
			cp := *j
			return &cp, nil
		}
	}
	return nil, nil
}

func (m *uiMinorJobStore) ListJobs(_ context.Context, _ int64, f store.ListJobsFilter) ([]models.Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []models.Job
outer:
	for _, j := range m.jobs {
		for _, excl := range f.ExcludeStatuses {
			if j.Status == excl {
				continue outer
			}
		}
		out = append(out, *j)
	}
	return out, nil
}

func (m *uiMinorJobStore) UpdateJobStatus(_ context.Context, _ int64, _ int64, _ models.JobStatus) error {
	return nil
}

func (m *uiMinorJobStore) UpdateApplicationStatus(_ context.Context, _ int64, _ int64, _ models.ApplicationStatus) error {
	return nil
}

// ─── helper: build a Server and serve one GET / request with an auth'd user ──

// dashboardWithUser builds a Server, wraps its handler with a middleware that
// injects a *models.User into the context (simulating a logged-in session),
// then returns the response body of GET /.
func dashboardWithUser(t *testing.T, srv *Server, user *models.User) string {
	t.Helper()

	// Wrap the server handler with an auth-injection middleware.
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), userContextKey, user)
		srv.Handler().ServeHTTP(w, r.WithContext(ctx))
	})

	ts := httptest.NewServer(handler)
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200; body: %s", resp.StatusCode, b)
	}
	return string(b)
}

func newTestUser() *models.User {
	return &models.User{
		ID:          1,
		Email:       "qa@example.com",
		DisplayName: "QA Tester",
	}
}

// ─── NextScrapeAt computation tests ──────────────────────────────────────────

// TestNextScrapeAt_NilFn verifies that when no lastScrapeFn is configured the
// dashboard shows the "Scrape running soon" fallback rather than a countdown.
func TestNextScrapeAt_NilFn(t *testing.T) {
	// No WithLastScrapeFn — fn stays nil.
	srv := NewServer(&uiMinorJobStore{}).WithScrapeInterval(time.Hour)

	body := dashboardWithUser(t, srv, newTestUser())
	if !strings.Contains(body, "Scrape running soon") {
		t.Error("expected 'Scrape running soon' text when lastScrapeFn is nil")
	}
	if strings.Contains(body, "data-next-scrape=") {
		t.Error("did not expect data-next-scrape attribute when lastScrapeFn is nil")
	}
}

// TestNextScrapeAt_ZeroLastScrape verifies that when lastScrapeFn returns the
// zero time, NextScrapeAt remains zero and the fallback text is shown.
func TestNextScrapeAt_ZeroLastScrape(t *testing.T) {
	fn := func() time.Time { return time.Time{} }
	srv := NewServer(&uiMinorJobStore{}).
		WithLastScrapeFn(fn).
		WithScrapeInterval(time.Hour)

	body := dashboardWithUser(t, srv, newTestUser())
	if !strings.Contains(body, "Scrape running soon") {
		t.Error("expected 'Scrape running soon' text when lastScrapeFn returns zero time")
	}
	if strings.Contains(body, "data-next-scrape=") {
		t.Error("did not expect data-next-scrape attribute when last scrape time is zero")
	}
}

// TestNextScrapeAt_ValidTime verifies that NextScrapeAt == lastScrapeAt +
// scrapeInterval and that the data-next-scrape attribute is rendered with the
// correct RFC-3339 UTC timestamp.
func TestNextScrapeAt_ValidTime(t *testing.T) {
	last := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)
	interval := 2 * time.Hour
	want := last.Add(interval) // 2026-01-15T12:00:00Z

	fn := func() time.Time { return last }
	srv := NewServer(&uiMinorJobStore{}).
		WithLastScrapeFn(fn).
		WithScrapeInterval(interval)

	body := dashboardWithUser(t, srv, newTestUser())

	wantAttr := `data-next-scrape="` + want.UTC().Format("2006-01-02T15:04:05Z") + `"`
	if !strings.Contains(body, wantAttr) {
		t.Errorf("expected attribute %q in dashboard body", wantAttr)
	}
	if strings.Contains(body, "Scrape running soon") {
		t.Error("did not expect 'Scrape running soon' fallback when last scrape time is valid")
	}
}

// TestNextScrapeAt_ZeroInterval verifies that NextScrapeAt remains zero when
// scrapeInterval is 0, even if lastScrapeFn returns a real time.
func TestNextScrapeAt_ZeroInterval(t *testing.T) {
	last := time.Now().Add(-5 * time.Minute)
	fn := func() time.Time { return last }
	srv := NewServer(&uiMinorJobStore{}).
		WithLastScrapeFn(fn).
		WithScrapeInterval(0) // zero interval → no countdown

	body := dashboardWithUser(t, srv, newTestUser())
	if !strings.Contains(body, "Scrape running soon") {
		t.Error("expected 'Scrape running soon' text when scrapeInterval is 0")
	}
	if strings.Contains(body, "data-next-scrape=") {
		t.Error("did not expect data-next-scrape attribute when scrapeInterval is 0")
	}
}

// ─── Dashboard template smoke check (countdown widget) ───────────────────────

// TestDashboardTemplate_CountdownWidget verifies the rendered dashboard contains
// both the scrape-countdown CSS class and the data-next-scrape attribute when
// an authenticated user has a valid lastScrapeAt time.
func TestDashboardTemplate_CountdownWidget(t *testing.T) {
	last := time.Date(2026, 3, 1, 8, 0, 0, 0, time.UTC)
	fn := func() time.Time { return last }
	srv := NewServer(&uiMinorJobStore{}).
		WithLastScrapeFn(fn).
		WithScrapeInterval(time.Hour)

	body := dashboardWithUser(t, srv, newTestUser())

	markers := []struct {
		name    string
		snippet string
	}{
		{"scrape-countdown class", `class="scrape-countdown"`},
		{"data-next-scrape attribute", `data-next-scrape="`},
	}
	for _, m := range markers {
		if !strings.Contains(body, m.snippet) {
			t.Errorf("dashboard template missing %s: snippet %q not found", m.name, m.snippet)
		}
	}
}

// ─── Rejected jobs excluded from dashboard ────────────────────────────────────

// TestDashboard_DoesNotShowRejectedJobs verifies that GET / renders a discovered
// job but not a rejected job, confirming the ExcludeStatuses filter is wired up.
func TestDashboard_DoesNotShowRejectedJobs(t *testing.T) {
	discoveredJob := &models.Job{
		ID:     10,
		Title:  "Discovered Position",
		Status: models.StatusDiscovered,
	}
	rejectedJob := &models.Job{
		ID:     11,
		Title:  "Rejected Position",
		Status: models.StatusRejected,
	}

	ms := &uiMinorJobStore{
		jobs: []*models.Job{discoveredJob, rejectedJob},
	}
	srv := NewServer(ms)

	body := dashboardWithUser(t, srv, newTestUser())

	if strings.Contains(body, rejectedJob.Title) {
		t.Errorf("dashboard body contains rejected job title %q; expected it to be hidden", rejectedJob.Title)
	}
	if !strings.Contains(body, discoveredJob.Title) {
		t.Errorf("dashboard body missing discovered job title %q; expected it to be shown", discoveredJob.Title)
	}
}
