// NOTE: Go is not installed in this container. These tests require a Docker
// build (or a local Go toolchain) to execute:
//
//	docker build -t jobhuntr . && docker run --rm jobhuntr go test ./internal/scraper/...
//
// Run from the repo root. All tests should pass with no regressions.

package scraper

import (
	"testing"
	"time"
)

// TestScheduler_Interval verifies that Interval() returns the duration
// supplied to NewScheduler.
func TestScheduler_Interval(t *testing.T) {
	t.Run("returns configured interval", func(t *testing.T) {
		want := 30 * time.Minute
		sched := NewScheduler([]Source{&mockSource{}}, newMockStore(), &mockUserFilterReader{}, want, nil)
		if got := sched.Interval(); got != want {
			t.Errorf("Interval() = %v, want %v", got, want)
		}
	})

	t.Run("zero interval is preserved", func(t *testing.T) {
		sched := NewScheduler([]Source{&mockSource{}}, newMockStore(), &mockUserFilterReader{}, 0, nil)
		if got := sched.Interval(); got != 0 {
			t.Errorf("Interval() = %v, want 0", got)
		}
	})

	t.Run("large interval is preserved", func(t *testing.T) {
		want := 24 * time.Hour
		sched := NewScheduler([]Source{&mockSource{}}, newMockStore(), &mockUserFilterReader{}, want, nil)
		if got := sched.Interval(); got != want {
			t.Errorf("Interval() = %v, want %v", got, want)
		}
	})
}
