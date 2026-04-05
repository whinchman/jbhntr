// Package web (internal test) — authenticated-path tests for handleJobCardsPartial.
//
// These tests must live in package web (not web_test) because they inject an
// authenticated *models.User via the unexported userContextKey. External tests
// in job_cards_test.go cover the unauthenticated path and the respondJobAction
// card-deck target; this file covers the authenticated rendering paths.
//
// Coverage added here:
//   A. GET /partials/job-cards with authenticated user and jobs present returns
//      200 HTML containing "job-card-active" class.
//   B. GET /partials/job-cards with authenticated user and no jobs returns 200
//      HTML containing "job-card-empty" class.
//   C. GET /partials/job-cards with authenticated user and jobs present does NOT
//      return an empty body (regression: unauthenticated path returns empty).
//   D. GET /partials/job-cards with authenticated user and a rejected job returns
//      HTML that does NOT include that job (ExcludeStatuses=[rejected] is
//      honoured by the uiMinorJobStore mock which implements the filter).
//
// NOTE: Go runtime is not available in this container. Tests are written but
// not executed. Compile-time correctness has been reviewed statically.

package web

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/whinchman/jobhuntr/internal/models"
	"github.com/whinchman/jobhuntr/internal/store"
)

// ─── minimal mock JobStore with ExcludeStatuses support ─────────────────────
// (mirrors the uiMinorJobStore already in ui_minor_internal_test.go; duplicated
// here so this file is self-contained and does not create cross-test-file
// dependencies within the package.)

type cardPartialJobStore struct {
	jobs []*models.Job
}

func (m *cardPartialJobStore) GetJob(_ context.Context, _ int64, id int64) (*models.Job, error) {
	for _, j := range m.jobs {
		if j.ID == id {
			cp := *j
			return &cp, nil
		}
	}
	return nil, nil
}

func (m *cardPartialJobStore) ListJobs(_ context.Context, _ int64, f store.ListJobsFilter) ([]models.Job, error) {
	var out []models.Job
outer:
	for _, j := range m.jobs {
		for _, excl := range f.ExcludeStatuses {
			if j.Status == excl {
				continue outer
			}
		}
		if f.Status != "" && j.Status != f.Status {
			continue
		}
		out = append(out, *j)
	}
	return out, nil
}

func (m *cardPartialJobStore) UpdateJobStatus(_ context.Context, _ int64, _ int64, _ models.JobStatus) error {
	return nil
}

func (m *cardPartialJobStore) UpdateApplicationStatus(_ context.Context, _ int64, _ int64, _ models.ApplicationStatus) error {
	return nil
}

func (m *cardPartialJobStore) RetryJob(_ context.Context, _ int64, _ int64) error {
	return nil
}

// ─── helper ──────────────────────────────────────────────────────────────────

// cardPartialWithUser constructs a Server backed by the given store, wraps its
// handler with an auth-injection middleware that places user in the context
// (simulating a logged-in session without requiring a session store), then
// issues GET /partials/job-cards and returns the response body.
func cardPartialWithUser(t *testing.T, st *cardPartialJobStore, user *models.User) (int, string) {
	t.Helper()

	srv := NewServer(st)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), userContextKey, user)
		srv.Handler().ServeHTTP(w, r.WithContext(ctx))
	})

	ts := httptest.NewServer(handler)
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/partials/job-cards")
	if err != nil {
		t.Fatalf("GET /partials/job-cards: %v", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func cardPartialTestUser() *models.User {
	return &models.User{ID: 1, Email: "qa@example.com", DisplayName: "QA Tester"}
}

// ─── tests ────────────────────────────────────────────────────────────────────

// TestHandleJobCardsPartial_Authenticated_WithJobs_ReturnsActiveCard verifies
// that an authenticated GET /partials/job-cards with jobs present returns 200
// HTML that contains the "job-card-active" class (the active card in the deck).
// This covers Acceptance Criterion 1 from tinder-mobile-qa.md.
func TestHandleJobCardsPartial_Authenticated_WithJobs_ReturnsActiveCard(t *testing.T) {
	st := &cardPartialJobStore{
		jobs: []*models.Job{
			{ID: 1, Title: "Go Engineer", Company: "Acme", Status: models.StatusDiscovered, UserID: 1},
			{ID: 2, Title: "Backend Dev",  Company: "Foo",  Status: models.StatusNotified,   UserID: 1},
		},
	}

	status, body := cardPartialWithUser(t, st, cardPartialTestUser())

	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %.400s", status, body)
	}
	if !strings.Contains(body, "job-card-active") {
		t.Errorf("response does not contain 'job-card-active' class — active card not rendered; body: %.400s", body)
	}
	// Active card should reference the first job's title/company.
	if !strings.Contains(body, "Go Engineer") {
		t.Errorf("response missing first job title 'Go Engineer'; body: %.400s", body)
	}
}

// TestHandleJobCardsPartial_Authenticated_NoJobs_ReturnsEmptyState verifies
// that an authenticated GET /partials/job-cards with no jobs returns 200 HTML
// containing the "job-card-empty" class (empty state).
// This covers Acceptance Criterion 2 from tinder-mobile-qa.md.
func TestHandleJobCardsPartial_Authenticated_NoJobs_ReturnsEmptyState(t *testing.T) {
	st := &cardPartialJobStore{jobs: []*models.Job{}}

	status, body := cardPartialWithUser(t, st, cardPartialTestUser())

	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %.400s", status, body)
	}
	if !strings.Contains(body, "job-card-empty") {
		t.Errorf("response does not contain 'job-card-empty' class — empty state not rendered; body: %.400s", body)
	}
	// Active card class must not appear when there are no jobs.
	if strings.Contains(body, "job-card-active") {
		t.Error("response contains 'job-card-active' but no jobs were provided — wrong template branch")
	}
}

// TestHandleJobCardsPartial_Authenticated_NotEmpty verifies that an
// authenticated request with jobs present returns a non-empty body (regression
// guard: the unauthenticated path must return empty; authenticated must not).
func TestHandleJobCardsPartial_Authenticated_NotEmpty(t *testing.T) {
	st := &cardPartialJobStore{
		jobs: []*models.Job{
			{ID: 10, Title: "SRE", Company: "Baz", Status: models.StatusDiscovered, UserID: 1},
		},
	}

	status, body := cardPartialWithUser(t, st, cardPartialTestUser())

	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	if strings.TrimSpace(body) == "" {
		t.Error("authenticated request with jobs returned empty body — unauthenticated path taken in error")
	}
}

// TestHandleJobCardsPartial_Authenticated_ExcludesRejected verifies that
// rejected jobs are filtered out in the authenticated card-partial response.
// The cardPartialJobStore implements ExcludeStatuses so the mock honours the
// filter that handleJobCardsPartial sets.
func TestHandleJobCardsPartial_Authenticated_ExcludesRejected(t *testing.T) {
	st := &cardPartialJobStore{
		jobs: []*models.Job{
			{ID: 20, Title: "Frontend Dev", Company: "Alpha", Status: models.StatusDiscovered, UserID: 1},
			{ID: 21, Title: "Rejected Role", Company: "Beta",  Status: models.StatusRejected,   UserID: 1},
		},
	}

	status, body := cardPartialWithUser(t, st, cardPartialTestUser())

	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	// The rejected job's title must not appear in the rendered HTML.
	if strings.Contains(body, "Rejected Role") {
		t.Error("rejected job appeared in card partial response — ExcludeStatuses filter not applied")
	}
	// The non-rejected job must appear.
	if !strings.Contains(body, "Frontend Dev") {
		t.Errorf("non-rejected job 'Frontend Dev' missing from card partial; body: %.400s", body)
	}
}
