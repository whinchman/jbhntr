package scraper

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/whinchman/jobhuntr/internal/models"
)

const sampleSerpAPIResponse = `{
  "search_metadata": {"status": "Success"},
  "jobs_results": [
    {
      "title": "Senior Go Engineer",
      "company_name": "Acme Corp",
      "location": "Remote",
      "description": "Build scalable systems in Go.",
      "job_id": "abc123",
      "detected_extensions": {
        "salary": "$150,000 - $200,000 a year"
      },
      "apply_options": [
        {"title": "Apply on company website", "link": "https://example.com/apply/1"}
      ]
    },
    {
      "title": "Staff Engineer",
      "company_name": "Beta Inc",
      "location": "New York, NY",
      "description": "Lead technical direction.",
      "job_id": "def456",
      "detected_extensions": {},
      "apply_options": []
    }
  ]
}`

const emptySerpAPIResponse = `{
  "search_metadata": {"status": "Success"},
  "jobs_results": []
}`

func newTestSource(t *testing.T, handler http.HandlerFunc) (*SerpAPISource, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	src := NewSerpAPISource("test-api-key")
	src.baseURL = srv.URL
	return src, srv
}

func TestSerpAPISource_Search(t *testing.T) {
	ctx := context.Background()

	t.Run("parses jobs_results correctly", func(t *testing.T) {
		src, _ := newTestSource(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(sampleSerpAPIResponse))
		})

		filter := models.SearchFilter{Keywords: "senior go engineer", Location: "Remote"}
		jobs, err := src.Search(ctx, filter)
		if err != nil {
			t.Fatalf("Search() error = %v", err)
		}
		if len(jobs) != 2 {
			t.Fatalf("len(jobs) = %d, want 2", len(jobs))
		}

		j := jobs[0]
		if j.Title != "Senior Go Engineer" {
			t.Errorf("Title = %q, want %q", j.Title, "Senior Go Engineer")
		}
		if j.Company != "Acme Corp" {
			t.Errorf("Company = %q, want %q", j.Company, "Acme Corp")
		}
		if j.Location != "Remote" {
			t.Errorf("Location = %q", j.Location)
		}
		if j.Salary != "$150,000 - $200,000 a year" {
			t.Errorf("Salary = %q", j.Salary)
		}
		if j.ApplyURL != "https://example.com/apply/1" {
			t.Errorf("ApplyURL = %q", j.ApplyURL)
		}
		if j.ExternalID != "abc123" {
			t.Errorf("ExternalID = %q, want %q", j.ExternalID, "abc123")
		}
		if j.Source != "serpapi" {
			t.Errorf("Source = %q, want serpapi", j.Source)
		}
		if j.Status != models.StatusDiscovered {
			t.Errorf("Status = %q, want discovered", j.Status)
		}
	})

	t.Run("empty results returns empty slice", func(t *testing.T) {
		src, _ := newTestSource(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(emptySerpAPIResponse))
		})

		jobs, err := src.Search(ctx, models.SearchFilter{Keywords: "niche job"})
		if err != nil {
			t.Fatalf("Search() error = %v", err)
		}
		if len(jobs) != 0 {
			t.Errorf("len(jobs) = %d, want 0", len(jobs))
		}
	})

	t.Run("job without apply_options has empty apply_url", func(t *testing.T) {
		src, _ := newTestSource(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(sampleSerpAPIResponse))
		})

		jobs, _ := src.Search(ctx, models.SearchFilter{})
		if jobs[1].ApplyURL != "" {
			t.Errorf("jobs[1].ApplyURL = %q, want empty", jobs[1].ApplyURL)
		}
	})

	t.Run("http error returns error", func(t *testing.T) {
		src, _ := newTestSource(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error": "internal server error"}`))
		})

		_, err := src.Search(ctx, models.SearchFilter{Keywords: "golang"})
		if err == nil {
			t.Error("Search() expected error for 500 response, got nil")
		}
	})

	t.Run("passes api_key and query params", func(t *testing.T) {
		var gotQuery string
		src, _ := newTestSource(t, func(w http.ResponseWriter, r *http.Request) {
			gotQuery = r.URL.RawQuery
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(emptySerpAPIResponse))
		})

		src.Search(ctx, models.SearchFilter{Keywords: "golang dev", Location: "Austin"})

		if gotQuery == "" {
			t.Error("no query string received by server")
		}
		// Should contain api_key, engine, and q params
		for _, param := range []string{"api_key=test-api-key", "engine=google_jobs"} {
			if !contains(gotQuery, param) {
				t.Errorf("query %q missing param %q", gotQuery, param)
			}
		}
	})
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		len(s) > 0 && (s[:len(substr)] == substr ||
			containsAt(s, substr)))
}

func containsAt(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
