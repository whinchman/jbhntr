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

const defaultSerpAPIBaseURL = "https://serpapi.com/search"

// SerpAPISource implements Source using the SerpAPI google_jobs engine.
type SerpAPISource struct {
	apiKey  string
	baseURL string
	client  *http.Client
	limiter *rate.Limiter
}

// NewSerpAPISource creates a SerpAPISource with a 1 req/s rate limit.
func NewSerpAPISource(apiKey string) *SerpAPISource {
	return &SerpAPISource{
		apiKey:  apiKey,
		baseURL: defaultSerpAPIBaseURL,
		client:  &http.Client{Timeout: 30 * time.Second},
		limiter: rate.NewLimiter(rate.Every(time.Second), 1),
	}
}

// serpAPIResponse is the JSON shape returned by SerpAPI google_jobs.
type serpAPIResponse struct {
	JobsResults []serpAPIJob `json:"jobs_results"`
}

type serpAPIJob struct {
	Title          string            `json:"title"`
	CompanyName    string            `json:"company_name"`
	Location       string            `json:"location"`
	Description    string            `json:"description"`
	JobID          string            `json:"job_id"`
	DetectedExt    serpAPIDetectedExt `json:"detected_extensions"`
	ApplyOptions   []serpAPIApply    `json:"apply_options"`
}

type serpAPIDetectedExt struct {
	Salary string `json:"salary"`
}

type serpAPIApply struct {
	Link string `json:"link"`
}

// Name returns the source identifier for SerpAPI.
func (s *SerpAPISource) Name() string { return "serpapi" }

// Search queries SerpAPI for jobs matching filter and returns discovered jobs.
func (s *SerpAPISource) Search(ctx context.Context, filter models.SearchFilter) ([]models.Job, error) {
	if err := s.limiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("scraper: rate limit wait: %w", err)
	}

	params := url.Values{}
	params.Set("engine", "google_jobs")
	params.Set("api_key", s.apiKey)

	q := filter.Keywords
	location := filter.Location

	// SerpAPI doesn't accept "remote" as a location. Fold it into the
	// query string instead so Google Jobs treats it as a keyword filter.
	if strings.EqualFold(location, "remote") {
		if q != "" {
			q += " remote"
		} else {
			q = "remote"
		}
		location = ""
	}

	if q != "" {
		params.Set("q", q)
	}
	if location != "" {
		params.Set("location", location)
	}

	reqURL := s.baseURL + "?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("scraper: build request: %w", err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("scraper: http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("scraper: serpapi returned status %d: %s", resp.StatusCode, string(body))
	}

	var apiResp serpAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("scraper: decode response: %w", err)
	}

	jobs := make([]models.Job, 0, len(apiResp.JobsResults))
	for _, r := range apiResp.JobsResults {
		applyURL := ""
		if len(r.ApplyOptions) > 0 {
			applyURL = r.ApplyOptions[0].Link
		}
		jobs = append(jobs, models.Job{
			ExternalID:  r.JobID,
			Source:      "serpapi",
			Title:       r.Title,
			Company:     r.CompanyName,
			Location:    r.Location,
			Description: r.Description,
			Salary:      r.DetectedExt.Salary,
			ApplyURL:    applyURL,
			Status:      models.StatusDiscovered,
		})
	}
	return jobs, nil
}
