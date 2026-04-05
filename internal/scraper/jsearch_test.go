package scraper

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/whinchman/jobhuntr/internal/models"
)

const sampleJSearchResponse = `{
  "status": "OK",
  "request_id": "test-request-id",
  "data": [
    {
      "job_id": "jsearch-abc123",
      "job_title": "Senior Go Engineer",
      "employer_name": "Acme Corp",
      "job_city": "San Francisco",
      "job_state": "CA",
      "job_country": "US",
      "job_description": "Build scalable backend systems in Go.",
      "job_min_salary": 150000,
      "job_max_salary": 200000,
      "job_apply_link": "https://linkedin.com/jobs/view/123"
    },
    {
      "job_id": "jsearch-def456",
      "job_title": "Staff Engineer",
      "employer_name": "Beta Inc",
      "job_city": "New York",
      "job_state": "NY",
      "job_country": "US",
      "job_description": "Lead technical direction.",
      "job_min_salary": 0,
      "job_max_salary": 0,
      "job_apply_link": "https://indeed.com/viewjob?jk=456"
    }
  ]
}`

const emptyJSearchResponse = `{
  "status": "OK",
  "request_id": "test-empty",
  "data": []
}`

// newTestJSearchSource creates a JSearchSource pointing at the given test server.
func newTestJSearchSource(t *testing.T, handler http.HandlerFunc) (*JSearchSource, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	src := NewJSearchSource("test-rapidapi-key")
	src.baseURL = srv.URL
	return src, srv
}

func TestJSearchSource_Name(t *testing.T) {
	src := NewJSearchSource("key")
	if src.Name() != "jsearch" {
		t.Errorf("Name() = %q, want %q", src.Name(), "jsearch")
	}
}

func TestJSearchSource_Search(t *testing.T) {
	ctx := context.Background()

	t.Run("parses data array correctly", func(t *testing.T) {
		src, _ := newTestJSearchSource(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(sampleJSearchResponse))
		})

		filter := models.SearchFilter{Keywords: "golang", Location: "San Francisco"}
		jobs, err := src.Search(ctx, filter)
		if err != nil {
			t.Fatalf("Search() error = %v", err)
		}
		if len(jobs) != 2 {
			t.Fatalf("len(jobs) = %d, want 2", len(jobs))
		}

		j := jobs[0]
		if j.ExternalID != "jsearch-abc123" {
			t.Errorf("ExternalID = %q, want jsearch-abc123", j.ExternalID)
		}
		if j.Source != "jsearch" {
			t.Errorf("Source = %q, want jsearch", j.Source)
		}
		if j.Title != "Senior Go Engineer" {
			t.Errorf("Title = %q, want Senior Go Engineer", j.Title)
		}
		if j.Company != "Acme Corp" {
			t.Errorf("Company = %q, want Acme Corp", j.Company)
		}
		if j.Location != "San Francisco, CA" {
			t.Errorf("Location = %q, want San Francisco, CA", j.Location)
		}
		if j.Description != "Build scalable backend systems in Go." {
			t.Errorf("Description = %q", j.Description)
		}
		if j.Salary != "$150000\u2013$200000" {
			t.Errorf("Salary = %q, want $150000\u2013$200000", j.Salary)
		}
		if j.ApplyURL != "https://linkedin.com/jobs/view/123" {
			t.Errorf("ApplyURL = %q", j.ApplyURL)
		}
		if j.Status != models.StatusDiscovered {
			t.Errorf("Status = %q, want discovered", j.Status)
		}
	})

	t.Run("zero salary returns empty string", func(t *testing.T) {
		src, _ := newTestJSearchSource(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(sampleJSearchResponse))
		})

		jobs, err := src.Search(ctx, models.SearchFilter{Keywords: "staff"})
		if err != nil {
			t.Fatalf("Search() error = %v", err)
		}
		if len(jobs) < 2 {
			t.Fatalf("expected at least 2 jobs, got %d", len(jobs))
		}
		if jobs[1].Salary != "" {
			t.Errorf("jobs[1].Salary = %q, want empty string for zero salary", jobs[1].Salary)
		}
	})

	t.Run("empty data returns empty slice not error", func(t *testing.T) {
		src, _ := newTestJSearchSource(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(emptyJSearchResponse))
		})

		jobs, err := src.Search(ctx, models.SearchFilter{Keywords: "niche role"})
		if err != nil {
			t.Fatalf("Search() error = %v", err)
		}
		if jobs == nil {
			t.Error("Search() returned nil, want empty non-nil slice")
		}
		if len(jobs) != 0 {
			t.Errorf("len(jobs) = %d, want 0", len(jobs))
		}
	})

	t.Run("http 500 returns error", func(t *testing.T) {
		src, _ := newTestJSearchSource(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"message": "Internal Server Error"}`))
		})

		_, err := src.Search(ctx, models.SearchFilter{Keywords: "golang"})
		if err == nil {
			t.Error("Search() expected error for HTTP 500 response, got nil")
		}
	})

	t.Run("http 429 returns error", func(t *testing.T) {
		src, _ := newTestJSearchSource(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"message": "Rate limit exceeded"}`))
		})

		_, err := src.Search(ctx, models.SearchFilter{Keywords: "golang"})
		if err == nil {
			t.Error("Search() expected error for HTTP 429 response, got nil")
		}
	})

	t.Run("sends RapidAPI auth headers", func(t *testing.T) {
		var gotKey, gotHost string
		src, _ := newTestJSearchSource(t, func(w http.ResponseWriter, r *http.Request) {
			gotKey = r.Header.Get("X-RapidAPI-Key")
			gotHost = r.Header.Get("X-RapidAPI-Host")
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(emptyJSearchResponse))
		})

		src.Search(ctx, models.SearchFilter{Keywords: "golang"})

		if gotKey != "test-rapidapi-key" {
			t.Errorf("X-RapidAPI-Key = %q, want test-rapidapi-key", gotKey)
		}
		if gotHost != "jsearch.p.rapidapi.com" {
			t.Errorf("X-RapidAPI-Host = %q, want jsearch.p.rapidapi.com", gotHost)
		}
	})

	t.Run("combines keywords and location into query param", func(t *testing.T) {
		var gotQuery string
		src, _ := newTestJSearchSource(t, func(w http.ResponseWriter, r *http.Request) {
			gotQuery = r.URL.Query().Get("query")
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(emptyJSearchResponse))
		})

		src.Search(ctx, models.SearchFilter{Keywords: "golang", Location: "New York"})

		if gotQuery != "golang New York" {
			t.Errorf("query param = %q, want %q", gotQuery, "golang New York")
		}
	})

	t.Run("keywords only when location is empty", func(t *testing.T) {
		var gotQuery string
		src, _ := newTestJSearchSource(t, func(w http.ResponseWriter, r *http.Request) {
			gotQuery = r.URL.Query().Get("query")
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(emptyJSearchResponse))
		})

		src.Search(ctx, models.SearchFilter{Keywords: "backend developer"})

		if gotQuery != "backend developer" {
			t.Errorf("query param = %q, want %q", gotQuery, "backend developer")
		}
	})

	t.Run("sends date_posted=week and num_pages=1 params", func(t *testing.T) {
		var gotDatePosted, gotNumPages string
		src, _ := newTestJSearchSource(t, func(w http.ResponseWriter, r *http.Request) {
			gotDatePosted = r.URL.Query().Get("date_posted")
			gotNumPages = r.URL.Query().Get("num_pages")
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(emptyJSearchResponse))
		})

		src.Search(ctx, models.SearchFilter{Keywords: "golang"})

		if gotDatePosted != "week" {
			t.Errorf("date_posted = %q, want week", gotDatePosted)
		}
		if gotNumPages != "1" {
			t.Errorf("num_pages = %q, want 1", gotNumPages)
		}
	})

	t.Run("DedupHash is not set by Search", func(t *testing.T) {
		src, _ := newTestJSearchSource(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(sampleJSearchResponse))
		})

		jobs, err := src.Search(ctx, models.SearchFilter{Keywords: "golang"})
		if err != nil {
			t.Fatalf("Search() error = %v", err)
		}
		for _, j := range jobs {
			if j.DedupHash != "" {
				t.Errorf("DedupHash should be empty from Search, got %q for job %q", j.DedupHash, j.ExternalID)
			}
		}
	})
}

// TestFormatLocation exercises the location formatting helper.
func TestFormatLocation(t *testing.T) {
	tests := []struct {
		city, state string
		want        string
	}{
		{"San Francisco", "CA", "San Francisco, CA"},
		{"", "CA", "CA"},
		{"Austin", "", "Austin"},
		{"", "", ""},
		{"  Seattle  ", "  WA  ", "Seattle, WA"},
	}
	for _, tc := range tests {
		got := formatLocation(tc.city, tc.state)
		if got != tc.want {
			t.Errorf("formatLocation(%q, %q) = %q, want %q", tc.city, tc.state, got, tc.want)
		}
	}
}

// TestFormatSalary exercises the salary formatting helper.
func TestFormatSalary(t *testing.T) {
	tests := []struct {
		min, max float64
		want     string
	}{
		{150000, 200000, "$150000\u2013$200000"},
		{0, 0, ""},
		{80000, 0, "$80000+"},
		{0, 120000, "up to $120000"},
		{-1, -1, ""},
		{50000, -1, "$50000+"},
		{-1, 90000, "up to $90000"},
	}
	for _, tc := range tests {
		got := formatSalary(tc.min, tc.max)
		if got != tc.want {
			t.Errorf("formatSalary(%v, %v) = %q, want %q", tc.min, tc.max, got, tc.want)
		}
	}
}
