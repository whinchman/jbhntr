package web_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
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

func (m *mockJobStore) GetJob(_ context.Context, id int64) (*models.Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	j, ok := m.jobs[id]
	if !ok {
		return nil, fmt.Errorf("store: job %d not found", id)
	}
	cp := *j
	return &cp, nil
}

func (m *mockJobStore) ListJobs(_ context.Context, f store.ListJobsFilter) ([]models.Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []models.Job
	for _, j := range m.jobs {
		if f.Status != "" && j.Status != f.Status {
			continue
		}
		result = append(result, *j)
	}
	return result, nil
}

func (m *mockJobStore) UpdateJobStatus(_ context.Context, id int64, newStatus models.JobStatus) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	j, ok := m.jobs[id]
	if !ok {
		return fmt.Errorf("store: job %d not found", id)
	}
	j.Status = newStatus
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

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf(`body["status"] = %q, want "ok"`, body["status"])
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
