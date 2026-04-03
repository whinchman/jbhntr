package web_test

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/whinchman/jobhuntr/internal/models"
	"github.com/whinchman/jobhuntr/internal/store"
	"github.com/whinchman/jobhuntr/internal/web"
)

// ─── mock JobStore ───────────────────────────────────────────────────────────

type mockJobStore struct {
	mu   sync.Mutex
	jobs map[int64]*models.Job
}

func newMockJobStore(jobs ...*models.Job) *mockJobStore {
	m := &mockJobStore{jobs: make(map[int64]*models.Job)}
	for _, j := range jobs {
		m.jobs[j.ID] = j
	}
	return m
}

func (m *mockJobStore) GetJob(_ context.Context, userID int64, id int64) (*models.Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	j, ok := m.jobs[id]
	if !ok {
		return nil, fmt.Errorf("store: job %d not found", id)
	}
	if userID != 0 && j.UserID != userID {
		return nil, fmt.Errorf("store: job %d not found", id)
	}
	cp := *j
	return &cp, nil
}

func (m *mockJobStore) ListJobs(_ context.Context, userID int64, f store.ListJobsFilter) ([]models.Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []models.Job
	for _, j := range m.jobs {
		if userID != 0 && j.UserID != userID {
			continue
		}
		if f.Status != "" && j.Status != f.Status {
			continue
		}
		result = append(result, *j)
	}
	return result, nil
}

func (m *mockJobStore) UpdateJobStatus(_ context.Context, userID int64, id int64, newStatus models.JobStatus) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	j, ok := m.jobs[id]
	if !ok {
		return fmt.Errorf("store: job %d not found", id)
	}
	if userID != 0 && j.UserID != userID {
		return fmt.Errorf("store: job %d not found", id)
	}
	j.Status = newStatus
	return nil
}

func (m *mockJobStore) UpdateApplicationStatus(_ context.Context, userID int64, id int64, status models.ApplicationStatus) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	j, ok := m.jobs[id]
	if !ok {
		return fmt.Errorf("store: job %d not found", id)
	}
	if userID != 0 && j.UserID != userID {
		return fmt.Errorf("store: job %d not found", id)
	}
	j.ApplicationStatus = status
	return nil
}

// ─── mock FilterStore ───────────────────────────────────────────────────────

type mockFilterStore struct {
	mu          sync.Mutex
	filters     map[int64][]models.UserSearchFilter
	resumes     map[int64]string
	ntfyTopics  map[int64]string
	nextID      int64
}

func newMockFilterStore() *mockFilterStore {
	return &mockFilterStore{
		filters:    make(map[int64][]models.UserSearchFilter),
		resumes:    make(map[int64]string),
		ntfyTopics: make(map[int64]string),
		nextID:     1,
	}
}

func (m *mockFilterStore) CreateUserFilter(_ context.Context, userID int64, filter *models.UserSearchFilter) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	filter.ID = m.nextID
	m.nextID++
	filter.UserID = userID
	m.filters[userID] = append(m.filters[userID], *filter)
	return nil
}

func (m *mockFilterStore) ListUserFilters(_ context.Context, userID int64) ([]models.UserSearchFilter, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.filters[userID], nil
}

func (m *mockFilterStore) DeleteUserFilter(_ context.Context, userID int64, filterID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	filters := m.filters[userID]
	for i, f := range filters {
		if f.ID == filterID {
			m.filters[userID] = append(filters[:i], filters[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("store: filter %d not found for user %d", filterID, userID)
}

func (m *mockFilterStore) UpdateUserResume(_ context.Context, userID int64, markdown string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.resumes[userID] = markdown
	return nil
}

func (m *mockFilterStore) UpdateUserNtfyTopic(_ context.Context, userID int64, topic string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ntfyTopics[userID] = topic
	return nil
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func newTestJob(id int64, status models.JobStatus) *models.Job {
	return &models.Job{
		ID:       id,
		Title:    "Software Engineer",
		Company:  "Acme",
		Location: "Remote",
		Status:   status,
	}
}

func newServer(t *testing.T, jobs ...*models.Job) *httptest.Server {
	t.Helper()
	ms := newMockJobStore(jobs...)
	srv := web.NewServer(ms)
	return httptest.NewServer(srv.Handler())
}

// extractCSRFToken finds the CSRF meta tag in an HTML body and returns the
// unescaped token value.
func extractCSRFToken(body string) string {
	marker := `name="csrf-token" content="`
	idx := strings.Index(body, marker)
	if idx < 0 {
		return ""
	}
	start := idx + len(marker)
	end := strings.Index(body[start:], `"`)
	if end < 0 {
		return ""
	}
	return html.UnescapeString(body[start : start+end])
}

// ─── tests ────────────────────────────────────────────────────────────────────

func TestHealth(t *testing.T) {
	ts := newServer(t)
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/health")
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf(`body["status"] = %v, want "ok"`, body["status"])
	}
	if _, ok := body["uptime"]; !ok {
		t.Error(`body["uptime"] missing from health response`)
	}
}

func TestListJobs(t *testing.T) {
	ts := newServer(t,
		newTestJob(1, models.StatusDiscovered),
		newTestJob(2, models.StatusNotified),
	)
	defer ts.Close()

	t.Run("no filter returns all jobs", func(t *testing.T) {
		resp, err := ts.Client().Get(ts.URL + "/api/jobs")
		if err != nil {
			t.Fatalf("GET /api/jobs: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("status = %d, want 200", resp.StatusCode)
		}

		var jobs []models.Job
		if err := json.NewDecoder(resp.Body).Decode(&jobs); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if len(jobs) != 2 {
			t.Errorf("len(jobs) = %d, want 2", len(jobs))
		}
	})

	t.Run("status filter returns matching jobs", func(t *testing.T) {
		resp, err := ts.Client().Get(ts.URL + "/api/jobs?status=discovered")
		if err != nil {
			t.Fatalf("GET /api/jobs?status=discovered: %v", err)
		}
		defer resp.Body.Close()

		var jobs []models.Job
		if err := json.NewDecoder(resp.Body).Decode(&jobs); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if len(jobs) != 1 {
			t.Errorf("len(jobs) = %d, want 1", len(jobs))
		}
		if jobs[0].Status != models.StatusDiscovered {
			t.Errorf("status = %q, want discovered", jobs[0].Status)
		}
	})
}

func TestGetJob(t *testing.T) {
	ts := newServer(t, newTestJob(42, models.StatusDiscovered))
	defer ts.Close()

	t.Run("found", func(t *testing.T) {
		resp, err := ts.Client().Get(ts.URL + "/api/jobs/42")
		if err != nil {
			t.Fatalf("GET /api/jobs/42: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("status = %d, want 200", resp.StatusCode)
		}
		var job models.Job
		if err := json.NewDecoder(resp.Body).Decode(&job); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if job.ID != 42 {
			t.Errorf("id = %d, want 42", job.ID)
		}
	})

	t.Run("not found returns 404", func(t *testing.T) {
		resp, err := ts.Client().Get(ts.URL + "/api/jobs/999")
		if err != nil {
			t.Fatalf("GET /api/jobs/999: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("status = %d, want 404", resp.StatusCode)
		}
	})

	t.Run("invalid id returns 400", func(t *testing.T) {
		resp, err := ts.Client().Get(ts.URL + "/api/jobs/notanid")
		if err != nil {
			t.Fatalf("GET /api/jobs/notanid: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", resp.StatusCode)
		}
	})
}

func TestApproveJob(t *testing.T) {
	t.Run("approve discovered job", func(t *testing.T) {
		ts := newServer(t, newTestJob(1, models.StatusDiscovered))
		defer ts.Close()

		resp, err := ts.Client().Post(ts.URL+"/api/jobs/1/approve", "application/json", nil)
		if err != nil {
			t.Fatalf("POST /api/jobs/1/approve: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("status = %d, want 200", resp.StatusCode)
		}
		var job models.Job
		if err := json.NewDecoder(resp.Body).Decode(&job); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if job.Status != models.StatusApproved {
			t.Errorf("status = %q, want approved", job.Status)
		}
	})

	t.Run("approve notified job", func(t *testing.T) {
		ts := newServer(t, newTestJob(2, models.StatusNotified))
		defer ts.Close()

		resp, err := ts.Client().Post(ts.URL+"/api/jobs/2/approve", "application/json", nil)
		if err != nil {
			t.Fatalf("POST: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("status = %d, want 200", resp.StatusCode)
		}
	})

	t.Run("approve already-approved job returns 409", func(t *testing.T) {
		ts := newServer(t, newTestJob(3, models.StatusApproved))
		defer ts.Close()

		resp, err := ts.Client().Post(ts.URL+"/api/jobs/3/approve", "application/json", nil)
		if err != nil {
			t.Fatalf("POST: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusConflict {
			t.Errorf("status = %d, want 409", resp.StatusCode)
		}
	})
}

func TestRejectJob(t *testing.T) {
	t.Run("reject discovered job", func(t *testing.T) {
		ts := newServer(t, newTestJob(1, models.StatusDiscovered))
		defer ts.Close()

		resp, err := ts.Client().Post(ts.URL+"/api/jobs/1/reject", "application/json", nil)
		if err != nil {
			t.Fatalf("POST /api/jobs/1/reject: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("status = %d, want 200", resp.StatusCode)
		}
		var job models.Job
		if err := json.NewDecoder(resp.Body).Decode(&job); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if job.Status != models.StatusRejected {
			t.Errorf("status = %q, want rejected", job.Status)
		}
	})

	t.Run("reject non-existent job returns 404", func(t *testing.T) {
		ts := newServer(t)
		defer ts.Close()

		resp, err := ts.Client().Post(ts.URL+"/api/jobs/999/reject", "application/json", nil)
		if err != nil {
			t.Fatalf("POST: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("status = %d, want 404", resp.StatusCode)
		}
	})
}

// newCompleteJob creates a complete job whose PDF paths point to real temp files.
// The caller is responsible for removing the files when done (via t.Cleanup).
func newCompleteJob(t *testing.T, id int64) *models.Job {
	t.Helper()
	writeTemp := func(name, content string) string {
		f, err := os.CreateTemp(t.TempDir(), name)
		if err != nil {
			t.Fatalf("create temp file: %v", err)
		}
		if _, err := f.WriteString(content); err != nil {
			t.Fatalf("write temp file: %v", err)
		}
		f.Close()
		return f.Name()
	}
	return &models.Job{
		ID:         id,
		Title:      "Staff Engineer",
		Company:    "Globex",
		Location:   "Remote",
		Status:     models.StatusComplete,
		ResumeHTML: "<p>Resume</p>",
		CoverHTML:  "<p>Cover</p>",
		ResumePDF:  writeTemp("resume*.pdf", "%PDF-resume"),
		CoverPDF:   writeTemp("cover*.pdf", "%PDF-cover"),
	}
}

func TestJobDetail(t *testing.T) {
	t.Run("known job returns 200 HTML", func(t *testing.T) {
		job := newTestJob(42, models.StatusDiscovered)
		ts := newServer(t, job)
		defer ts.Close()

		resp, err := ts.Client().Get(ts.URL + "/jobs/42")
		if err != nil {
			t.Fatalf("GET /jobs/42: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("status = %d, want 200", resp.StatusCode)
		}
		ct := resp.Header.Get("Content-Type")
		if ct == "" || ct[:9] != "text/html" {
			t.Errorf("Content-Type = %q, want text/html", ct)
		}
	})

	t.Run("unknown job returns 404", func(t *testing.T) {
		ts := newServer(t)
		defer ts.Close()

		resp, err := ts.Client().Get(ts.URL + "/jobs/999")
		if err != nil {
			t.Fatalf("GET /jobs/999: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("status = %d, want 404", resp.StatusCode)
		}
	})
}

func TestDownloadResumePDF(t *testing.T) {
	t.Run("complete job with PDF returns 200 and file data", func(t *testing.T) {
		job := newCompleteJob(t, 7)
		ts := newServer(t, job)
		defer ts.Close()

		resp, err := ts.Client().Get(ts.URL + "/output/7/resume.pdf")
		if err != nil {
			t.Fatalf("GET /output/7/resume.pdf: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("status = %d, want 200", resp.StatusCode)
		}
		if cd := resp.Header.Get("Content-Disposition"); cd == "" {
			t.Error("Content-Disposition header missing")
		}
	})

	t.Run("job with empty ResumePDF returns 404", func(t *testing.T) {
		job := newTestJob(8, models.StatusApproved) // no PDF paths
		ts := newServer(t, job)
		defer ts.Close()

		resp, err := ts.Client().Get(ts.URL + "/output/8/resume.pdf")
		if err != nil {
			t.Fatalf("GET /output/8/resume.pdf: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("status = %d, want 404", resp.StatusCode)
		}
	})

	t.Run("unknown job returns 404", func(t *testing.T) {
		ts := newServer(t)
		defer ts.Close()

		resp, err := ts.Client().Get(ts.URL + "/output/999/resume.pdf")
		if err != nil {
			t.Fatalf("GET /output/999/resume.pdf: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("status = %d, want 404", resp.StatusCode)
		}
	})
}

func TestDownloadCoverPDF(t *testing.T) {
	t.Run("complete job with PDF returns 200 and file data", func(t *testing.T) {
		job := newCompleteJob(t, 9)
		ts := newServer(t, job)
		defer ts.Close()

		resp, err := ts.Client().Get(ts.URL + "/output/9/cover_letter.pdf")
		if err != nil {
			t.Fatalf("GET /output/9/cover_letter.pdf: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("status = %d, want 200", resp.StatusCode)
		}
		if cd := resp.Header.Get("Content-Disposition"); cd == "" {
			t.Error("Content-Disposition header missing")
		}
	})

	t.Run("job with empty CoverPDF returns 404", func(t *testing.T) {
		job := newTestJob(10, models.StatusApproved)
		ts := newServer(t, job)
		defer ts.Close()

		resp, err := ts.Client().Get(ts.URL + "/output/10/cover_letter.pdf")
		if err != nil {
			t.Fatalf("GET /output/10/cover_letter.pdf: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("status = %d, want 404", resp.StatusCode)
		}
	})

	t.Run("unknown job returns 404", func(t *testing.T) {
		ts := newServer(t)
		defer ts.Close()

		resp, err := ts.Client().Get(ts.URL + "/output/999/cover_letter.pdf")
		if err != nil {
			t.Fatalf("GET /output/999/cover_letter.pdf: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("status = %d, want 404", resp.StatusCode)
		}
	})
}

// ─── settings helpers ─────────────────────────────────────────────────────────

// newSettingsServer creates a test server wired with a mockFilterStore.
// It returns the test server and the mock filter store for assertions.
func newSettingsServer(t *testing.T) (*httptest.Server, *mockFilterStore) {
	t.Helper()
	ms := newMockJobStore()
	fs := newMockFilterStore()

	// Seed a filter so tests have data to work with.
	fs.CreateUserFilter(context.Background(), 0, &models.UserSearchFilter{
		Keywords:  "golang engineer",
		Location:  "Remote",
		MinSalary: 100000,
	})

	srv := web.NewServerWithConfig(ms, nil, fs, nil)
	ts := httptest.NewServer(srv.Handler())
	return ts, fs
}

// ─── settings tests ──────────────────────────────────────────────────────────

func TestSettingsPage(t *testing.T) {
	t.Run("GET /settings returns 200 HTML", func(t *testing.T) {
		ts, _ := newSettingsServer(t)
		defer ts.Close()

		resp, err := ts.Client().Get(ts.URL + "/settings")
		if err != nil {
			t.Fatalf("GET /settings: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("status = %d, want 200", resp.StatusCode)
		}
		ct := resp.Header.Get("Content-Type")
		if !strings.HasPrefix(ct, "text/html") {
			t.Errorf("Content-Type = %q, want text/html", ct)
		}
	})
}

func TestSaveResume(t *testing.T) {
	t.Run("POST /settings/resume saves to DB and redirects", func(t *testing.T) {
		ts, fs := newSettingsServer(t)
		defer ts.Close()

		form := url.Values{"resume": {"# Updated Resume\n\nNew content."}}
		resp, err := ts.Client().Post(ts.URL+"/settings/resume", "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
		if err != nil {
			t.Fatalf("POST /settings/resume: %v", err)
		}
		defer resp.Body.Close()

		// Should redirect (302) to /settings?saved=1
		if resp.StatusCode != http.StatusOK && resp.StatusCode/100 != 3 {
			t.Errorf("status = %d, want 2xx or 3xx", resp.StatusCode)
		}

		fs.mu.Lock()
		got := fs.resumes[0]
		fs.mu.Unlock()
		if got != "# Updated Resume\n\nNew content." {
			t.Errorf("resume content = %q, want updated content", got)
		}
	})
}

func TestAddFilter(t *testing.T) {
	t.Run("POST /settings/filters adds filter to DB", func(t *testing.T) {
		ts, fs := newSettingsServer(t)
		defer ts.Close()

		form := url.Values{
			"keywords":   {"senior go engineer"},
			"location":   {"New York"},
			"min_salary": {"120000"},
		}
		resp, err := ts.Client().Post(ts.URL+"/settings/filters", "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
		if err != nil {
			t.Fatalf("POST /settings/filters: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK && resp.StatusCode/100 != 3 {
			t.Errorf("status = %d, want 2xx or 3xx", resp.StatusCode)
		}

		fs.mu.Lock()
		filters := fs.filters[0]
		fs.mu.Unlock()
		if len(filters) != 2 {
			t.Fatalf("len(filters) = %d, want 2", len(filters))
		}
		last := filters[len(filters)-1]
		if last.Keywords != "senior go engineer" {
			t.Errorf("last.Keywords = %q, want senior go engineer", last.Keywords)
		}
		if last.MinSalary != 120000 {
			t.Errorf("last.MinSalary = %d, want 120000", last.MinSalary)
		}
	})
}

func TestRemoveFilter(t *testing.T) {
	t.Run("POST /settings/filters/remove?id=N removes filter from DB", func(t *testing.T) {
		ts, fs := newSettingsServer(t)
		defer ts.Close()

		// The seeded filter has ID 1.
		resp, err := ts.Client().Post(ts.URL+"/settings/filters/remove?id=1", "application/x-www-form-urlencoded", nil)
		if err != nil {
			t.Fatalf("POST /settings/filters/remove: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK && resp.StatusCode/100 != 3 {
			t.Errorf("status = %d, want 2xx or 3xx", resp.StatusCode)
		}

		fs.mu.Lock()
		filters := fs.filters[0]
		fs.mu.Unlock()
		if len(filters) != 0 {
			t.Errorf("len(filters) = %d, want 0", len(filters))
		}
	})

	t.Run("POST /settings/filters/remove with invalid id returns 400", func(t *testing.T) {
		ts, _ := newSettingsServer(t)
		defer ts.Close()

		resp, err := ts.Client().Post(ts.URL+"/settings/filters/remove?id=notanid", "application/x-www-form-urlencoded", nil)
		if err != nil {
			t.Fatalf("POST: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", resp.StatusCode)
		}
	})
}

// ─── adversarial / isolation tests ──────────────────────────────────────────

func TestSettingsPage_EmptyState(t *testing.T) {
	t.Run("settings page with no filters and empty resume returns 200", func(t *testing.T) {
		ms := newMockJobStore()
		fs := newMockFilterStore()
		// No filters seeded, no resume set.
		srv := web.NewServerWithConfig(ms, nil, fs, nil)
		ts := httptest.NewServer(srv.Handler())
		defer ts.Close()

		resp, err := ts.Client().Get(ts.URL + "/settings")
		if err != nil {
			t.Fatalf("GET /settings: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("status = %d, want 200", resp.StatusCode)
		}
	})
}

func TestSaveResume_EmptyContent(t *testing.T) {
	t.Run("saving empty resume string succeeds", func(t *testing.T) {
		ts, fs := newSettingsServer(t)
		defer ts.Close()

		form := url.Values{"resume": {""}}
		resp, err := ts.Client().Post(ts.URL+"/settings/resume", "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
		if err != nil {
			t.Fatalf("POST /settings/resume: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK && resp.StatusCode/100 != 3 {
			t.Errorf("status = %d, want 2xx or 3xx", resp.StatusCode)
		}

		fs.mu.Lock()
		got := fs.resumes[0]
		fs.mu.Unlock()
		if got != "" {
			t.Errorf("resume content = %q, want empty string", got)
		}
	})
}

func TestRemoveFilter_WrongUser(t *testing.T) {
	t.Run("removing filter belonging to another user returns 500", func(t *testing.T) {
		ms := newMockJobStore()
		fs := newMockFilterStore()

		// Create a filter for user 42.
		fs.CreateUserFilter(context.Background(), 42, &models.UserSearchFilter{
			Keywords: "user42-only",
		})

		// Server runs without auth (nil user -> userID=0), so handler uses
		// userID=0 which does not own filter ID 1.
		srv := web.NewServerWithConfig(ms, nil, fs, nil)
		ts := httptest.NewServer(srv.Handler())
		defer ts.Close()

		resp, err := ts.Client().Post(ts.URL+"/settings/filters/remove?id=1", "application/x-www-form-urlencoded", nil)
		if err != nil {
			t.Fatalf("POST /settings/filters/remove: %v", err)
		}
		defer resp.Body.Close()

		// The handler should return 500 because the store returns "not found"
		// for a filter that belongs to user 42 when accessed as user 0.
		if resp.StatusCode != http.StatusInternalServerError {
			t.Errorf("status = %d, want 500", resp.StatusCode)
		}

		// Verify the filter was NOT deleted.
		fs.mu.Lock()
		remaining := fs.filters[42]
		fs.mu.Unlock()
		if len(remaining) != 1 {
			t.Errorf("filter should not have been deleted, got %d filters for user 42", len(remaining))
		}
	})
}

func TestJobDetail_UserIsolation(t *testing.T) {
	t.Run("authenticated user cannot view another users job", func(t *testing.T) {
		// Create a job belonging to user 99.
		job := &models.Job{
			ID:       10,
			UserID:   99,
			Title:    "Secret Job",
			Company:  "Other Corp",
			Location: "Hidden",
			Status:   models.StatusDiscovered,
		}
		ms := newMockJobStore(job)
		us := newMockUserStore(&models.User{
			ID:          42,
			Provider:    "google",
			ProviderID:  "g-42",
			Email:       "user42@example.com",
			DisplayName: "User 42",
		})

		cfg := newAuthConfig()
		srv := web.NewServerWithConfig(ms, us, nil, cfg)
		ts := httptest.NewServer(srv.Handler())
		defer ts.Close()

		cookie := setSessionCookie(t, ts, 42)

		client := ts.Client()
		client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		}

		// Try to access job 10 (belongs to user 99) as user 42.
		req, _ := http.NewRequest(http.MethodGet, ts.URL+"/jobs/10", nil)
		req.AddCookie(cookie)
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("GET /jobs/10: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("status = %d, want 404 (user isolation)", resp.StatusCode)
		}
	})

	t.Run("authenticated user can view their own job", func(t *testing.T) {
		job := &models.Job{
			ID:       10,
			UserID:   42,
			Title:    "My Job",
			Company:  "My Corp",
			Location: "Remote",
			Status:   models.StatusDiscovered,
		}
		ms := newMockJobStore(job)
		us := newMockUserStore(&models.User{
			ID:          42,
			Provider:    "google",
			ProviderID:  "g-42",
			Email:       "user42@example.com",
			DisplayName: "User 42",
		})

		cfg := newAuthConfig()
		srv := web.NewServerWithConfig(ms, us, nil, cfg)
		ts := httptest.NewServer(srv.Handler())
		defer ts.Close()

		cookie := setSessionCookie(t, ts, 42)

		client := ts.Client()
		client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		}

		req, _ := http.NewRequest(http.MethodGet, ts.URL+"/jobs/10", nil)
		req.AddCookie(cookie)
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("GET /jobs/10: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("status = %d, want 200", resp.StatusCode)
		}
	})
}

func TestAPIJobDetail_UserIsolation(t *testing.T) {
	t.Run("authenticated user gets 404 for another users job via API", func(t *testing.T) {
		job := &models.Job{
			ID:       20,
			UserID:   99,
			Title:    "Secret API Job",
			Company:  "Other Corp",
			Location: "Hidden",
			Status:   models.StatusDiscovered,
		}
		ms := newMockJobStore(job)
		us := newMockUserStore(&models.User{
			ID:          42,
			Provider:    "google",
			ProviderID:  "g-42",
			Email:       "user42@example.com",
			DisplayName: "User 42",
		})

		cfg := newAuthConfig()
		srv := web.NewServerWithConfig(ms, us, nil, cfg)
		ts := httptest.NewServer(srv.Handler())
		defer ts.Close()

		cookie := setSessionCookie(t, ts, 42)

		client := ts.Client()
		client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		}

		req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/jobs/20", nil)
		req.AddCookie(cookie)
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("GET /api/jobs/20: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("status = %d, want 404 (user isolation)", resp.StatusCode)
		}
	})
}

func TestDashboard_UserIsolation(t *testing.T) {
	t.Run("dashboard only shows jobs belonging to authenticated user", func(t *testing.T) {
		job1 := &models.Job{
			ID:       1,
			UserID:   42,
			Title:    "My Job",
			Company:  "My Corp",
			Location: "Remote",
			Status:   models.StatusDiscovered,
		}
		job2 := &models.Job{
			ID:       2,
			UserID:   99,
			Title:    "Other Job",
			Company:  "Other Corp",
			Location: "Hidden",
			Status:   models.StatusDiscovered,
		}
		ms := newMockJobStore(job1, job2)
		us := newMockUserStore(&models.User{
			ID:          42,
			Provider:    "google",
			ProviderID:  "g-42",
			Email:       "user42@example.com",
			DisplayName: "User 42",
		})

		cfg := newAuthConfig()
		srv := web.NewServerWithConfig(ms, us, nil, cfg)
		ts := httptest.NewServer(srv.Handler())
		defer ts.Close()

		cookie := setSessionCookie(t, ts, 42)

		client := ts.Client()
		req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/jobs", nil)
		req.AddCookie(cookie)
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("GET /api/jobs: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("status = %d, want 200", resp.StatusCode)
		}

		var jobs []models.Job
		if err := json.NewDecoder(resp.Body).Decode(&jobs); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(jobs) != 1 {
			t.Errorf("len(jobs) = %d, want 1 (only user 42's job)", len(jobs))
		}
		if len(jobs) == 1 && jobs[0].ID != 1 {
			t.Errorf("job.ID = %d, want 1", jobs[0].ID)
		}
	})
}

func TestApproveJob_UserIsolation(t *testing.T) {
	t.Run("cannot approve another users job", func(t *testing.T) {
		job := &models.Job{
			ID:       5,
			UserID:   99,
			Title:    "Other Users Job",
			Company:  "Other Corp",
			Status:   models.StatusDiscovered,
		}
		ms := newMockJobStore(job)
		us := newMockUserStore(&models.User{
			ID:          42,
			Provider:    "google",
			ProviderID:  "g-42",
			Email:       "user42@example.com",
			DisplayName: "User 42",
		})

		cfg := newAuthConfig()
		srv := web.NewServerWithConfig(ms, us, nil, cfg)
		ts := httptest.NewServer(srv.Handler())
		defer ts.Close()

		cookie := setSessionCookie(t, ts, 42)

		// Need CSRF token for POST requests with auth enabled.
		client := ts.Client()
		client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		}

		// GET the dashboard to get the CSRF cookie.
		getReq, _ := http.NewRequest(http.MethodGet, ts.URL+"/", nil)
		getReq.AddCookie(cookie)
		getResp, err := client.Do(getReq)
		if err != nil {
			t.Fatalf("GET /: %v", err)
		}

		var csrfCookie *http.Cookie
		for _, c := range getResp.Cookies() {
			if c.Name == "_gorilla_csrf" {
				csrfCookie = c
				break
			}
		}
		bodyBytes, _ := io.ReadAll(getResp.Body)
		getResp.Body.Close()

		csrfToken := extractCSRFToken(string(bodyBytes))
		if csrfToken == "" || csrfCookie == nil {
			t.Fatal("could not extract CSRF token/cookie")
		}

		req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/jobs/5/approve", nil)
		req.AddCookie(cookie)
		req.AddCookie(csrfCookie)
		req.Header.Set("X-CSRF-Token", csrfToken)

		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("POST /api/jobs/5/approve: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("status = %d, want 404 (user isolation)", resp.StatusCode)
		}
	})
}

// ─── markdown download tests ─────────────────────────────────────────────────

// newJobWithMarkdown creates a job with ResumeMarkdown and CoverMarkdown set.
func newJobWithMarkdown(id int64) *models.Job {
	return &models.Job{
		ID:             id,
		Title:          "Staff Engineer",
		Company:        "Globex",
		Location:       "Remote",
		Status:         models.StatusComplete,
		ResumeMarkdown: "# Resume\n\nThis is my resume.",
		CoverMarkdown:  "# Cover Letter\n\nDear Hiring Manager,",
	}
}

func TestDownloadResumeMarkdown(t *testing.T) {
	t.Run("job with ResumeMarkdown returns 200 and markdown content", func(t *testing.T) {
		job := newJobWithMarkdown(20)
		ts := newServer(t, job)
		defer ts.Close()

		resp, err := ts.Client().Get(ts.URL + "/output/20/resume.md")
		if err != nil {
			t.Fatalf("GET /output/20/resume.md: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("status = %d, want 200", resp.StatusCode)
		}
		ct := resp.Header.Get("Content-Type")
		if !strings.HasPrefix(ct, "text/markdown") {
			t.Errorf("Content-Type = %q, want text/markdown", ct)
		}
		cd := resp.Header.Get("Content-Disposition")
		if !strings.Contains(cd, "resume.md") {
			t.Errorf("Content-Disposition = %q, want attachment; filename=resume.md", cd)
		}
		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), "# Resume") {
			t.Errorf("body = %q, want to contain resume markdown", string(body))
		}
	})

	t.Run("job with empty ResumeMarkdown returns 404", func(t *testing.T) {
		job := newTestJob(21, models.StatusComplete)
		ts := newServer(t, job)
		defer ts.Close()

		resp, err := ts.Client().Get(ts.URL + "/output/21/resume.md")
		if err != nil {
			t.Fatalf("GET /output/21/resume.md: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("status = %d, want 404", resp.StatusCode)
		}
	})

	t.Run("unknown job returns 404", func(t *testing.T) {
		ts := newServer(t)
		defer ts.Close()

		resp, err := ts.Client().Get(ts.URL + "/output/999/resume.md")
		if err != nil {
			t.Fatalf("GET /output/999/resume.md: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("status = %d, want 404", resp.StatusCode)
		}
	})
}

func TestDownloadCoverMarkdown(t *testing.T) {
	t.Run("job with CoverMarkdown returns 200 and markdown content", func(t *testing.T) {
		job := newJobWithMarkdown(30)
		ts := newServer(t, job)
		defer ts.Close()

		resp, err := ts.Client().Get(ts.URL + "/output/30/cover_letter.md")
		if err != nil {
			t.Fatalf("GET /output/30/cover_letter.md: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("status = %d, want 200", resp.StatusCode)
		}
		ct := resp.Header.Get("Content-Type")
		if !strings.HasPrefix(ct, "text/markdown") {
			t.Errorf("Content-Type = %q, want text/markdown", ct)
		}
		cd := resp.Header.Get("Content-Disposition")
		if !strings.Contains(cd, "cover_letter.md") {
			t.Errorf("Content-Disposition = %q, want attachment; filename=cover_letter.md", cd)
		}
		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), "# Cover Letter") {
			t.Errorf("body = %q, want to contain cover letter markdown", string(body))
		}
	})

	t.Run("job with empty CoverMarkdown returns 404", func(t *testing.T) {
		job := newTestJob(31, models.StatusComplete)
		ts := newServer(t, job)
		defer ts.Close()

		resp, err := ts.Client().Get(ts.URL + "/output/31/cover_letter.md")
		if err != nil {
			t.Fatalf("GET /output/31/cover_letter.md: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("status = %d, want 404", resp.StatusCode)
		}
	})

	t.Run("unknown job returns 404", func(t *testing.T) {
		ts := newServer(t)
		defer ts.Close()

		resp, err := ts.Client().Get(ts.URL + "/output/999/cover_letter.md")
		if err != nil {
			t.Fatalf("GET /output/999/cover_letter.md: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("status = %d, want 404", resp.StatusCode)
		}
	})
}

func TestDownloadResumeDocx(t *testing.T) {
	t.Run("job with ResumeMarkdown returns 200 and DOCX content", func(t *testing.T) {
		job := newJobWithMarkdown(40)
		ts := newServer(t, job)
		defer ts.Close()

		resp, err := ts.Client().Get(ts.URL + "/output/40/resume.docx")
		if err != nil {
			t.Fatalf("GET /output/40/resume.docx: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("status = %d, want 200", resp.StatusCode)
		}
		ct := resp.Header.Get("Content-Type")
		wantCT := "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
		if ct != wantCT {
			t.Errorf("Content-Type = %q, want %q", ct, wantCT)
		}
		cd := resp.Header.Get("Content-Disposition")
		if !strings.Contains(cd, "resume.docx") {
			t.Errorf("Content-Disposition = %q, want attachment; filename=resume.docx", cd)
		}
		body, _ := io.ReadAll(resp.Body)
		// DOCX files start with PK (zip magic bytes)
		if len(body) < 2 || body[0] != 'P' || body[1] != 'K' {
			t.Errorf("body does not look like a DOCX/ZIP file (first bytes: %v)", body[:4])
		}
	})

	t.Run("job with empty ResumeMarkdown returns 404", func(t *testing.T) {
		job := newTestJob(41, models.StatusComplete)
		ts := newServer(t, job)
		defer ts.Close()

		resp, err := ts.Client().Get(ts.URL + "/output/41/resume.docx")
		if err != nil {
			t.Fatalf("GET /output/41/resume.docx: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("status = %d, want 404", resp.StatusCode)
		}
	})

	t.Run("unknown job returns 404", func(t *testing.T) {
		ts := newServer(t)
		defer ts.Close()

		resp, err := ts.Client().Get(ts.URL + "/output/999/resume.docx")
		if err != nil {
			t.Fatalf("GET /output/999/resume.docx: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("status = %d, want 404", resp.StatusCode)
		}
	})
}

func TestDownloadCoverDocx(t *testing.T) {
	t.Run("job with CoverMarkdown returns 200 and DOCX content", func(t *testing.T) {
		job := newJobWithMarkdown(50)
		ts := newServer(t, job)
		defer ts.Close()

		resp, err := ts.Client().Get(ts.URL + "/output/50/cover_letter.docx")
		if err != nil {
			t.Fatalf("GET /output/50/cover_letter.docx: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("status = %d, want 200", resp.StatusCode)
		}
		ct := resp.Header.Get("Content-Type")
		wantCT := "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
		if ct != wantCT {
			t.Errorf("Content-Type = %q, want %q", ct, wantCT)
		}
		cd := resp.Header.Get("Content-Disposition")
		if !strings.Contains(cd, "cover_letter.docx") {
			t.Errorf("Content-Disposition = %q, want attachment; filename=cover_letter.docx", cd)
		}
		body, _ := io.ReadAll(resp.Body)
		if len(body) < 2 || body[0] != 'P' || body[1] != 'K' {
			t.Errorf("body does not look like a DOCX/ZIP file (first bytes: %v)", body[:4])
		}
	})

	t.Run("job with empty CoverMarkdown returns 404", func(t *testing.T) {
		job := newTestJob(51, models.StatusComplete)
		ts := newServer(t, job)
		defer ts.Close()

		resp, err := ts.Client().Get(ts.URL + "/output/51/cover_letter.docx")
		if err != nil {
			t.Fatalf("GET /output/51/cover_letter.docx: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("status = %d, want 404", resp.StatusCode)
		}
	})

	t.Run("unknown job returns 404", func(t *testing.T) {
		ts := newServer(t)
		defer ts.Close()

		resp, err := ts.Client().Get(ts.URL + "/output/999/cover_letter.docx")
		if err != nil {
			t.Fatalf("GET /output/999/cover_letter.docx: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("status = %d, want 404", resp.StatusCode)
		}
	})
}
