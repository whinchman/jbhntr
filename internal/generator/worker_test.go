package generator

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/whinchman/jobhuntr/internal/models"
	"github.com/whinchman/jobhuntr/internal/store"
)

// ─── mock Generator ───────────────────────────────────────────────────────────

type mockGenerator struct {
	resumeMD  string
	resumeHTML string
	coverMD   string
	coverHTML  string
	err        error
}

func (m *mockGenerator) Generate(_ context.Context, _ models.Job, _ string) (string, string, string, string, error) {
	return m.resumeMD, m.resumeHTML, m.coverMD, m.coverHTML, m.err
}

// ─── mock Converter ───────────────────────────────────────────────────────────

type mockConverter struct {
	mu    sync.Mutex
	calls []string
	err   error
}

func (m *mockConverter) PDFFromHTML(_ context.Context, _ string, outputPath string) error {
	m.mu.Lock()
	m.calls = append(m.calls, outputPath)
	m.mu.Unlock()
	return m.err
}

// ─── mock WorkerStore ─────────────────────────────────────────────────────────

type mockWorkerStore struct {
	mu        sync.Mutex
	jobs      map[int64]*models.Job
	updates   []statusUpdate
	generated []generatedUpdate
}

type statusUpdate struct {
	id     int64
	status models.JobStatus
}

type generatedUpdate struct {
	id             int64
	resumeHTML     string
	coverHTML      string
	resumeMarkdown string
	coverMarkdown  string
	resumePDF      string
	coverPDF       string
}

func newMockWorkerStore(jobs ...*models.Job) *mockWorkerStore {
	m := &mockWorkerStore{jobs: make(map[int64]*models.Job)}
	for _, j := range jobs {
		m.jobs[j.ID] = j
	}
	return m
}

func (m *mockWorkerStore) GetJob(_ context.Context, _ int64, id int64) (*models.Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	j, ok := m.jobs[id]
	if !ok {
		return nil, errors.New("not found")
	}
	cp := *j
	return &cp, nil
}

func (m *mockWorkerStore) ListJobs(_ context.Context, _ int64, f store.ListJobsFilter) ([]models.Job, error) {
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

func (m *mockWorkerStore) UpdateJobStatus(_ context.Context, _ int64, id int64, status models.JobStatus) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.updates = append(m.updates, statusUpdate{id, status})
	if j, ok := m.jobs[id]; ok {
		j.Status = status
	}
	return nil
}

func (m *mockWorkerStore) UpdateJobGenerated(_ context.Context, _ int64, id int64, resumeHTML, coverHTML, resumeMarkdown, coverMarkdown, resumePDF, coverPDF string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.generated = append(m.generated, generatedUpdate{id, resumeHTML, coverHTML, resumeMarkdown, coverMarkdown, resumePDF, coverPDF})
	return nil
}

func (m *mockWorkerStore) UpdateJobError(_ context.Context, _ int64, id int64, errMsg string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.updates = append(m.updates, statusUpdate{id, models.StatusFailed})
	if j, ok := m.jobs[id]; ok {
		j.Status = models.StatusFailed
		j.ErrorMsg = errMsg
	}
	return nil
}

// ─── tests ────────────────────────────────────────────────────────────────────

func TestWorker_processApproved(t *testing.T) {
	ctx := context.Background()

	approvedJob := &models.Job{
		ID: 1, ExternalID: "abc", Source: "serpapi",
		Title: "Go Eng", Company: "Acme", Status: models.StatusApproved,
	}

	t.Run("happy path: generates and completes job", func(t *testing.T) {
		ms := newMockWorkerStore(approvedJob)
		gen := &mockGenerator{resumeMD: "# Resume", resumeHTML: "<h1>Resume</h1>", coverMD: "# Cover", coverHTML: "<h1>Cover</h1>"}
		conv := &mockConverter{}

		w := NewWorker(ms, gen, conv, t.TempDir(), "", 0, nil)
		if err := w.processApproved(ctx); err != nil {
			t.Fatalf("processApproved() error = %v", err)
		}

		// Status should be updated to generating then complete.
		if len(ms.updates) < 2 {
			t.Fatalf("expected >=2 status updates, got %d", len(ms.updates))
		}
		statuses := make([]models.JobStatus, len(ms.updates))
		for i, u := range ms.updates {
			statuses[i] = u.status
		}
		if statuses[0] != models.StatusGenerating {
			t.Errorf("updates[0] = %q, want generating", statuses[0])
		}
		if statuses[len(statuses)-1] != models.StatusComplete {
			t.Errorf("last update = %q, want complete", statuses[len(statuses)-1])
		}

		// PDF converter should have been called twice (resume + cover).
		if len(conv.calls) != 2 {
			t.Errorf("pdf converter calls = %d, want 2", len(conv.calls))
		}

		// UpdateJobGenerated should have been called with HTML and Markdown.
		if len(ms.generated) != 1 {
			t.Fatalf("generated updates = %d, want 1", len(ms.generated))
		}
		if ms.generated[0].resumeHTML != "<h1>Resume</h1>" {
			t.Errorf("resumeHTML = %q", ms.generated[0].resumeHTML)
		}
		if ms.generated[0].resumeMarkdown != "# Resume" {
			t.Errorf("resumeMarkdown = %q", ms.generated[0].resumeMarkdown)
		}
		if ms.generated[0].coverMarkdown != "# Cover" {
			t.Errorf("coverMarkdown = %q", ms.generated[0].coverMarkdown)
		}
	})

	t.Run("nil converter: skips PDF, still completes job", func(t *testing.T) {
		job2 := &models.Job{ID: 2, ExternalID: "def", Source: "serpapi", Title: "T", Company: "C", Status: models.StatusApproved}
		ms := newMockWorkerStore(job2)
		gen := &mockGenerator{resumeMD: "# Resume", resumeHTML: "<h1>Resume</h1>", coverMD: "# Cover", coverHTML: "<h1>Cover</h1>"}

		w := NewWorker(ms, gen, nil, t.TempDir(), "", 0, nil)
		w.processApproved(ctx)

		lastStatus := ms.updates[len(ms.updates)-1].status
		if lastStatus != models.StatusComplete {
			t.Errorf("last status = %q, want complete", lastStatus)
		}
		if len(ms.generated) != 1 {
			t.Fatalf("generated updates = %d, want 1", len(ms.generated))
		}
		// PDF paths should be empty since converter was nil.
		if ms.generated[0].resumePDF != "" {
			t.Errorf("resumePDF = %q, want empty (no converter)", ms.generated[0].resumePDF)
		}
		if ms.generated[0].coverPDF != "" {
			t.Errorf("coverPDF = %q, want empty (no converter)", ms.generated[0].coverPDF)
		}
	})

	t.Run("converter error is non-fatal: job still completes with empty pdf paths", func(t *testing.T) {
		job3 := &models.Job{ID: 3, ExternalID: "ghi", Source: "serpapi", Title: "T", Company: "C", Status: models.StatusApproved}
		ms := newMockWorkerStore(job3)
		gen := &mockGenerator{resumeMD: "# Resume", resumeHTML: "<h1>Resume</h1>", coverMD: "# Cover", coverHTML: "<h1>Cover</h1>"}
		conv := &mockConverter{err: errors.New("chromium unavailable")}

		w := NewWorker(ms, gen, conv, t.TempDir(), "", 0, nil)
		w.processApproved(ctx)

		lastStatus := ms.updates[len(ms.updates)-1].status
		if lastStatus != models.StatusComplete {
			t.Errorf("last status = %q, want complete (pdf failure is non-fatal)", lastStatus)
		}
		if len(ms.generated) != 1 {
			t.Fatalf("generated updates = %d, want 1", len(ms.generated))
		}
		// PDF paths should be empty after conversion failure.
		if ms.generated[0].resumePDF != "" {
			t.Errorf("resumePDF = %q, want empty after failed conversion", ms.generated[0].resumePDF)
		}
		if ms.generated[0].coverPDF != "" {
			t.Errorf("coverPDF = %q, want empty after failed conversion", ms.generated[0].coverPDF)
		}
	})

	t.Run("generator error sets status to failed", func(t *testing.T) {
		job4 := &models.Job{ID: 4, ExternalID: "xyz", Source: "serpapi", Title: "T", Company: "C", Status: models.StatusApproved}
		ms := newMockWorkerStore(job4)
		gen := &mockGenerator{err: errors.New("api down")}
		conv := &mockConverter{}

		w := NewWorker(ms, gen, conv, t.TempDir(), "", 0, nil)
		w.processApproved(ctx)

		lastStatus := ms.updates[len(ms.updates)-1].status
		if lastStatus != models.StatusFailed {
			t.Errorf("last status = %q, want failed", lastStatus)
		}
	})

	t.Run("no approved jobs is a no-op", func(t *testing.T) {
		ms := newMockWorkerStore()
		gen := &mockGenerator{}
		conv := &mockConverter{}

		w := NewWorker(ms, gen, conv, t.TempDir(), "", 0, nil)
		if err := w.processApproved(ctx); err != nil {
			t.Fatalf("processApproved() error = %v", err)
		}
		if len(ms.updates) != 0 {
			t.Errorf("expected 0 status updates, got %d", len(ms.updates))
		}
	})
}

func TestWorker_Start(t *testing.T) {
	t.Run("stops cleanly on context cancellation", func(t *testing.T) {
		ms := newMockWorkerStore()
		w := NewWorker(ms, &mockGenerator{}, &mockConverter{}, t.TempDir(), "", 50*time.Millisecond, nil)

		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Millisecond)
		defer cancel()

		done := make(chan struct{})
		go func() {
			w.Start(ctx)
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("Start() did not return after context cancellation")
		}
	})
}
