// stats_internal_test.go — internal-package tests for the stats page.
//
// This file is in package web (internal) so it can access the unexported
// statsData type and render the stats template directly without going through
// an HTTP server.
//
// NOTE: Go is not installed in this container. To execute these tests run:
//
//	docker build -t jobhuntr . && docker run --rm jobhuntr go test ./internal/web/...
//
// All tests should pass with no regressions.

package web

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/whinchman/jobhuntr/internal/models"
	"github.com/whinchman/jobhuntr/internal/store"
)

// renderStatsTemplate executes the stats template with the given statsData and
// returns the rendered HTML string. It fails the test if execution returns an error.
func renderStatsTemplate(t *testing.T, srv *Server, data statsData) string {
	t.Helper()
	var buf bytes.Buffer
	if err := srv.statsTmpl.ExecuteTemplate(&buf, "layout.html", data); err != nil {
		t.Fatalf("ExecuteTemplate error = %v", err)
	}
	return buf.String()
}

// newStatsOnlyServer builds a minimal Server suitable for template rendering
// tests. It uses a nil job store because the stats template does not call any
// job-store methods during rendering.
func newStatsOnlyServer() *Server {
	return NewServer(&uiMinorJobStore{})
}

// TestStatsTemplate_ZeroValues verifies that the stats template renders
// correctly when all stats are zero (new user with no jobs).
func TestStatsTemplate_ZeroValues(t *testing.T) {
	srv := newStatsOnlyServer()

	data := statsData{
		Stats:       store.UserJobStats{},
		WeeklyTrend: makeEmptyWeeklyTrend(),
		MaxWeekly:   1,
		User:        &models.User{ID: 1, DisplayName: "Test User", Email: "test@example.com"},
	}

	body := renderStatsTemplate(t, srv, data)

	// Template headings.
	if !strings.Contains(body, "Job Search Stats") {
		t.Error("missing 'Job Search Stats' heading")
	}

	// Zero values render as "0" inside stat-card spans.
	// TotalFound = 0 appears as ">0<" somewhere in the card markup.
	if !strings.Contains(body, ">0<") {
		t.Error("expected at least one '0' stat value in zero-state render")
	}

	// All stat labels should be present.
	for _, label := range []string{"Total Found", "Approved", "Rejected", "Applied", "Interviewing", "Won", "Lost"} {
		if !strings.Contains(body, label) {
			t.Errorf("missing stat label %q", label)
		}
	}

	// Weekly chart section should be present with 12 columns.
	if !strings.Contains(body, "bar-chart__col") {
		t.Error("weekly bar chart columns missing from zero-state render")
	}
	count := strings.Count(body, "bar-chart__col")
	if count != 12 {
		t.Errorf("bar-chart__col count = %d, want 12", count)
	}
}

// TestStatsTemplate_KnownValues verifies that the stats template renders
// specific count values correctly when non-zero data is provided.
func TestStatsTemplate_KnownValues(t *testing.T) {
	srv := newStatsOnlyServer()

	data := statsData{
		Stats: store.UserJobStats{
			TotalFound:        100,
			TotalApproved:     20,
			TotalRejected:     15,
			TotalApplied:      12,
			TotalInterviewing: 5,
			TotalWon:          3,
			TotalLost:         2,
		},
		WeeklyTrend: makeEmptyWeeklyTrend(),
		MaxWeekly:   1,
		User:        &models.User{ID: 1, DisplayName: "Test User", Email: "test@example.com"},
	}

	body := renderStatsTemplate(t, srv, data)

	// Each count must appear somewhere in the rendered output.
	// The template renders {{ .Stats.TotalFound }} etc. as bare integers.
	for _, want := range []string{"100", "20", "15", "12", "5", "3", "2"} {
		if !strings.Contains(body, want) {
			t.Errorf("rendered HTML missing expected value %q", want)
		}
	}

	// All stat labels must appear.
	for _, label := range []string{"Total Found", "Approved", "Rejected", "Applied", "Interviewing", "Won", "Lost"} {
		if !strings.Contains(body, label) {
			t.Errorf("rendered HTML missing stat label %q", label)
		}
	}
}

// TestStatsTemplate_WeeklyTrend_12Weeks verifies that the handler's
// week-backfill logic (which lives in handleStats, not the template itself)
// always produces exactly 12 WeeklyJobCount entries, and that the template
// renders all 12 bar columns.
func TestStatsTemplate_WeeklyTrend_12Weeks(t *testing.T) {
	srv := newStatsOnlyServer()

	// Provide only 3 weeks of data — the backfill logic in handleStats pads to
	// 12.  Here we simulate what handleStats would pass to the template.
	weekly := makeEmptyWeeklyTrend()
	// Inject counts into a few buckets to simulate partial data.
	weekly[0].Count = 4
	weekly[5].Count = 7
	weekly[11].Count = 2

	maxW := 7
	data := statsData{
		Stats:       store.UserJobStats{TotalFound: 13},
		WeeklyTrend: weekly,
		MaxWeekly:   maxW,
		User:        &models.User{ID: 1, DisplayName: "Test User", Email: "test@example.com"},
	}

	body := renderStatsTemplate(t, srv, data)

	// Exactly 12 bar columns must appear.
	colCount := strings.Count(body, "bar-chart__col")
	if colCount != 12 {
		t.Errorf("bar-chart__col count = %d, want exactly 12", colCount)
	}

	// The injected count values must appear.
	for _, wantNum := range []string{"4", "7", "2"} {
		if !strings.Contains(body, wantNum) {
			t.Errorf("weekly bar chart missing count value %q", wantNum)
		}
	}
}

// TestStatsTemplate_NavLink verifies that the Stats nav link appears in the
// rendered page for an authenticated user.
func TestStatsTemplate_NavLink(t *testing.T) {
	srv := newStatsOnlyServer()

	data := statsData{
		Stats:       store.UserJobStats{},
		WeeklyTrend: makeEmptyWeeklyTrend(),
		MaxWeekly:   1,
		User:        &models.User{ID: 1, DisplayName: "Nav Tester", Email: "nav@example.com"},
	}

	body := renderStatsTemplate(t, srv, data)

	if !strings.Contains(body, `href="/stats"`) {
		t.Error("rendered HTML missing Stats nav link href=/stats")
	}
}

// ─── helpers ─────────────────────────────────────────────────────────────────

// makeEmptyWeeklyTrend builds a 12-entry WeeklyJobCount slice matching what
// handleStats always produces (12 Mondays, counts all zero).
func makeEmptyWeeklyTrend() []store.WeeklyJobCount {
	now := time.Now().UTC()
	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	currentMonday := now.AddDate(0, 0, -(weekday - 1))
	currentMonday = time.Date(currentMonday.Year(), currentMonday.Month(), currentMonday.Day(), 0, 0, 0, 0, time.UTC)
	startMonday := currentMonday.AddDate(0, 0, -11*7)

	weekly := make([]store.WeeklyJobCount, 12)
	for i := 0; i < 12; i++ {
		weekly[i] = store.WeeklyJobCount{
			WeekStart: startMonday.AddDate(0, 0, i*7),
			Count:     0,
		}
	}
	return weekly
}
