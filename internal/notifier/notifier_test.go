package notifier

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/whinchman/jobhuntr/internal/models"
)

func TestNtfyNotifier_Notify(t *testing.T) {
	ctx := context.Background()

	t.Run("posts correct payload with salary", func(t *testing.T) {
		var got ntfyPayload
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("method = %s, want POST", r.Method)
			}
			body, _ := io.ReadAll(r.Body)
			if err := json.Unmarshal(body, &got); err != nil {
				t.Fatalf("unmarshal payload: %v", err)
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		n := NewNtfyNotifier(srv.URL, "http://localhost:8080")
		job := models.Job{
			ID:       42,
			Title:    "Senior Go Engineer",
			Company:  "Acme Corp",
			Location: "Remote",
			Salary:   "$150k–$200k",
		}

		if err := n.Notify(ctx, job, "test-topic"); err != nil {
			t.Fatalf("Notify() error = %v", err)
		}

		wantTitle := "New: Senior Go Engineer at Acme Corp"
		if got.Title != wantTitle {
			t.Errorf("title = %q, want %q", got.Title, wantTitle)
		}
		if !strings.Contains(got.Message, "Remote") {
			t.Errorf("message %q should contain location", got.Message)
		}
		if !strings.Contains(got.Message, "$150k–$200k") {
			t.Errorf("message %q should contain salary", got.Message)
		}
		if got.Click != "http://localhost:8080/jobs/42" {
			t.Errorf("click = %q, want http://localhost:8080/jobs/42", got.Click)
		}
		if got.Priority != 3 {
			t.Errorf("priority = %d, want 3", got.Priority)
		}
		if len(got.Tags) != 1 || got.Tags[0] != "briefcase" {
			t.Errorf("tags = %v, want [briefcase]", got.Tags)
		}
	})

	t.Run("posts correct payload without salary", func(t *testing.T) {
		var got ntfyPayload
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			json.Unmarshal(body, &got) //nolint:errcheck
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		n := NewNtfyNotifier(srv.URL, "http://app")
		job := models.Job{ID: 1, Title: "SRE", Company: "Foo", Location: "NYC"}

		if err := n.Notify(ctx, job, "jobs"); err != nil {
			t.Fatalf("Notify() error = %v", err)
		}

		if strings.Contains(got.Message, "·") {
			t.Errorf("message %q should not contain salary separator when salary is empty", got.Message)
		}
	})

	t.Run("empty topic is a no-op", func(t *testing.T) {
		called := false
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		n := NewNtfyNotifier(srv.URL, "http://app")
		if err := n.Notify(ctx, models.Job{ID: 1, Title: "T", Company: "C"}, ""); err != nil {
			t.Fatalf("Notify() error = %v", err)
		}
		if called {
			t.Error("expected no HTTP request when topic is empty")
		}
	})

	t.Run("non-2xx response returns error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer srv.Close()

		n := NewNtfyNotifier(srv.URL, "http://app")
		err := n.Notify(ctx, models.Job{ID: 1, Title: "T", Company: "C"}, "topic")
		if err == nil {
			t.Error("expected error for non-2xx response")
		}
	})

	t.Run("posts to correct topic URL", func(t *testing.T) {
		var gotPath string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		n := NewNtfyNotifier(srv.URL, "http://app")
		n.Notify(ctx, models.Job{ID: 1, Title: "T", Company: "C"}, "my-jobs") //nolint:errcheck

		if gotPath != "/my-jobs" {
			t.Errorf("path = %q, want /my-jobs", gotPath)
		}
	})
}
