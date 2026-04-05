package scraper

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/time/rate"

	"github.com/whinchman/jobhuntr/internal/models"
)

const defaultJSearchBaseURL = "https://jsearch.p.rapidapi.com/search"

// JSearchSource implements Source using the JSearch RapidAPI endpoint.
// It aggregates jobs from LinkedIn, Indeed, and other platforms via a single API call.
type JSearchSource struct {
	apiKey  string
	baseURL string
	client  *http.Client
	limiter *rate.Limiter
}

// NewJSearchSource creates a JSearchSource with a conservative 1 req/2s rate limit.
func NewJSearchSource(apiKey string) *JSearchSource {
	return &JSearchSource{
		apiKey:  apiKey,
		baseURL: defaultJSearchBaseURL,
		client:  &http.Client{Timeout: 30 * time.Second},
		limiter: rate.NewLimiter(rate.Every(2*time.Second), 1),
	}
}

// Name returns the source identifier for JSearch.
func (j *JSearchSource) Name() string { return "jsearch" }

// jsearchResponse is the JSON shape returned by the JSearch API.
type jsearchResponse struct {
	Data []jsearchJob `json:"data"`
}

type jsearchJob struct {
	JobID          string  `json:"job_id"`
	JobTitle       string  `json:"job_title"`
	EmployerName   string  `json:"employer_name"`
	JobCity        string  `json:"job_city"`
	JobState       string  `json:"job_state"`
	JobCountry     string  `json:"job_country"`
	JobDescription string  `json:"job_description"`
	JobMinSalary   float64 `json:"job_min_salary"`
	JobMaxSalary   float64 `json:"job_max_salary"`
	JobApplyLink   string  `json:"job_apply_link"`
}

// Search queries the JSearch API for jobs matching filter and returns discovered jobs.
func (j *JSearchSource) Search(ctx context.Context, filter models.SearchFilter) ([]models.Job, error) {
	if err := j.limiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("scraper: jsearch rate limit wait: %w", err)
	}

	// Combine keywords and location into a single query string.
	query := strings.TrimSpace(filter.Keywords)
	if loc := strings.TrimSpace(filter.Location); loc != "" {
		if query != "" {
			query += " " + loc
		} else {
			query = loc
		}
	}

	params := url.Values{}
	params.Set("query", query)
	params.Set("num_pages", "1")
	params.Set("date_posted", "week")

	reqURL := j.baseURL + "?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("scraper: jsearch build request: %w", err)
	}

	req.Header.Set("X-RapidAPI-Key", j.apiKey)
	req.Header.Set("X-RapidAPI-Host", "jsearch.p.rapidapi.com")

	resp, err := j.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("scraper: jsearch http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("scraper: jsearch returned status %d: %s", resp.StatusCode, string(body))
	}

	var apiResp jsearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("scraper: jsearch decode response: %w", err)
	}

	if len(apiResp.Data) == 0 {
		return []models.Job{}, nil
	}

	jobs := make([]models.Job, 0, len(apiResp.Data))
	for _, r := range apiResp.Data {
		jobs = append(jobs, models.Job{
			ExternalID:  r.JobID,
			Source:      "jsearch",
			Title:       r.JobTitle,
			Company:     r.EmployerName,
			Location:    formatLocation(r.JobCity, r.JobState),
			Description: r.JobDescription,
			Salary:      formatSalary(r.JobMinSalary, r.JobMaxSalary),
			ApplyURL:    r.JobApplyLink,
			Status:      models.StatusDiscovered,
		})
	}
	return jobs, nil
}

// formatLocation combines city and state into a single location string.
// Returns an empty string if both are empty.
func formatLocation(city, state string) string {
	city = strings.TrimSpace(city)
	state = strings.TrimSpace(state)
	if city == "" && state == "" {
		return ""
	}
	if city == "" {
		return state
	}
	if state == "" {
		return city
	}
	return city + ", " + state
}

// formatSalary formats min/max salary floats into a human-readable string.
// Returns empty string if both are zero.
// If only max is provided, returns "up to $<max>".
// If only min is provided, returns "$<min>+".
func formatSalary(min, max float64) string {
	if min <= 0 && max <= 0 {
		return ""
	}
	if max <= 0 {
		return fmt.Sprintf("$%.0f+", min)
	}
	if min <= 0 {
		return fmt.Sprintf("up to $%.0f", max)
	}
	return fmt.Sprintf("$%.0f\u2013$%.0f", min, max)
}
