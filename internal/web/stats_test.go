package web_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/whinchman/jobhuntr/internal/models"
	"github.com/whinchman/jobhuntr/internal/store"
	"github.com/whinchman/jobhuntr/internal/web"
)

// mockStatsStore satisfies web.StatsStore for unit tests.
type mockStatsStore struct {
	stats  store.UserJobStats
	weekly []store.WeeklyJobCount
	err    error
}

func (m *mockStatsStore) GetUserJobStats(_ context.Context, _ int64) (store.UserJobStats, error) {
	return m.stats, m.err
}

func (m *mockStatsStore) GetJobsPerWeek(_ context.Context, _ int64, _ int) ([]store.WeeklyJobCount, error) {
	return m.weekly, m.err
}

// newStatsServer builds a test server with auth + stats store wired up.
func newStatsServer(t *testing.T, us *mockUserStore, ss *mockStatsStore) *httptest.Server { //nolint:unparam
	t.Helper()
	cfg := newAuthConfig()
	ms := newMockJobStore()
	srv := web.NewServerWithConfig(ms, us, nil, cfg).
		WithStatsStore(ss)
	return httptest.NewServer(srv.Handler())
}

// TestHandleStats_Unauthenticated verifies that GET /stats redirects to /login
// when no session cookie is present.
func TestHandleStats_Unauthenticated(t *testing.T) {
	us := newMockUserStore()
	ss := &mockStatsStore{}
	ts := newStatsServer(t, us, ss)
	defer ts.Close()

	client := ts.Client()
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}

	resp, err := client.Get(ts.URL + "/stats")
	if err != nil {
		t.Fatalf("GET /stats: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("status = %d, want %d (SeeOther)", resp.StatusCode, http.StatusSeeOther)
	}
	loc := resp.Header.Get("Location")
	if loc != "/login" {
		t.Errorf("Location = %q, want /login", loc)
	}
}

// TestHandleStats_Authenticated verifies that GET /stats returns 200 and
// renders the stats page for an authenticated user.
func TestHandleStats_Authenticated(t *testing.T) {
	testUser := &models.User{
		ID:          7,
		Provider:    "google",
		ProviderID:  "g-007",
		Email:       "agent@example.com",
		DisplayName: "Agent User",
	}
	us := newMockUserStore(testUser)
	ss := &mockStatsStore{
		stats: store.UserJobStats{
			TotalFound:        42,
			TotalApproved:     10,
			TotalRejected:     5,
			TotalApplied:      8,
			TotalInterviewing: 3,
			TotalWon:          1,
			TotalLost:         2,
		},
		weekly: []store.WeeklyJobCount{
			{WeekStart: time.Now().AddDate(0, 0, -7), Count: 3},
			{WeekStart: time.Now(), Count: 5},
		},
	}
	ts := newStatsServer(t, us, ss)
	defer ts.Close()

	cookie := setSessionCookie(t, ts, testUser.ID)

	req, err := http.NewRequest(http.MethodGet, ts.URL+"/stats", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.AddCookie(cookie)

	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("GET /stats: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	body := string(bodyBytes)

	// Check page title.
	if !strings.Contains(body, "Job Search Stats") {
		t.Error("response body missing 'Job Search Stats' heading")
	}

	// Spot-check a stat value rendered by the template.
	if !strings.Contains(body, "42") {
		t.Error("response body missing TotalFound value '42'")
	}

	// Verify the Stats nav link is present for logged-in users.
	if !strings.Contains(body, `href="/stats"`) {
		t.Error("response body missing Stats nav link")
	}
}
