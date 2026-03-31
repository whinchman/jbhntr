# Plan: 2A — Scraper Interface & SerpAPI

## Overview
Define the Source interface and implement a SerpAPI Google Jobs client with rate limiting.

## Steps

### Step 1: Source interface (internal/scraper/source.go)
- Source interface: Search(ctx context.Context, filter models.SearchFilter) ([]models.Job, error)

### Step 2: SerpAPISource (internal/scraper/serpapi.go)
- Struct with http.Client, API key, rate.Limiter (1 req/s)
- Search() builds GET request to https://serpapi.com/search with:
  engine=google_jobs, q=filter.Keywords, location=filter.Location, api_key=key
- Parse JSON response: jobs_results[].{title, company_name, location, description,
  detected_extensions.salary, job_id, apply_options[0].link → apply_url}
- Map to []models.Job with Source="serpapi", Status=StatusDiscovered
- Rate limit via golang.org/x/time/rate

### Step 3: Tests (internal/scraper/serpapi_test.go)
- Use httptest.NewServer returning canned SerpAPI JSON
- Test: successful parse of jobs_results array
- Test: empty response returns empty slice
- Test: HTTP error returns wrapped error
- Test: fields mapped correctly (title, company, location, salary, apply_url, external_id)

## Files Created/Modified
- internal/scraper/source.go (replaces doc.go)
- internal/scraper/serpapi.go
- internal/scraper/serpapi_test.go
