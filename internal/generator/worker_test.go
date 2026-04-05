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
	resumeMD   string
	resumeHTML string
	coverMD    string
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
	users     map[int64]*models.User
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
	m := &mockWorkerStore{
		jobs:  make(map[int64]*models.Job),
		users: make(map[int64]*models.User),
	}
	for _, j := range jobs {
		m.jobs[j.ID] = j
	}
	return m
}

// withUser registers a user in the mock store and returns the store for
// convenient chaining in test setup.
func (m *mockWorkerStore) withUser(u *models.User) *mockWorkerStore {
	m.mu.Lock()
	m.users[u.ID] = u
	m.mu.Unlock()
	return m
}

func (m *mockWorkerStore) GetUser(_ context.Context, id int64) (*models.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.users[id]
	if !ok {
		return nil, errors.New("user not found")
	}
	cp := *u
	return &cp, nil
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

// ─── helpers ──────────────────────────────────────────────────────────────────

// userWithResume returns a minimal User with the given resume markdown.
func userWithResume(id int64, resume string) *models.User {
	return &models.User{ID: id, Email: "test@example.com", ResumeMarkdown: resume}
}

// ─── tests ────────────────────────────────────────────────────────────────────

func TestWorker_processApproved(t *testing.T) {
	ctx := context.Background()

	approvedJob := &models.Job{
		ID: 1, UserID: 10, ExternalID: "abc", Source: "serpapi",
		Title: "Go Eng", Company: "Acme", Status: models.StatusApproved,
	}

	t.Run("happy path: generates and completes job", func(t *testing.T) {
		ms := newMockWorkerStore(approvedJob).withUser(userWithResume(10, "# My Resume"))
		gen := &mockGenerator{resumeMD: "# Resume", resumeHTML: "<h1>Resume</h1>", coverMD: "# Cover", coverHTML: "<h1>Cover</h1>"}
		conv := &mockConverter{}

		w := NewWorker(ms, gen, conv, t.TempDir(), 0, nil)
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
		job2 := &models.Job{ID: 2, UserID: 10, ExternalID: "def", Source: "serpapi", Title: "T", Company: "C", Status: models.StatusApproved}
		ms := newMockWorkerStore(job2).withUser(userWithResume(10, "# My Resume"))
		gen := &mockGenerator{resumeMD: "# Resume", resumeHTML: "<h1>Resume</h1>", coverMD: "# Cover", coverHTML: "<h1>Cover</h1>"}

		w := NewWorker(ms, gen, nil, t.TempDir(), 0, nil)
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
		job3 := &models.Job{ID: 3, UserID: 10, ExternalID: "ghi", Source: "serpapi", Title: "T", Company: "C", Status: models.StatusApproved}
		ms := newMockWorkerStore(job3).withUser(userWithResume(10, "# My Resume"))
		gen := &mockGenerator{resumeMD: "# Resume", resumeHTML: "<h1>Resume</h1>", coverMD: "# Cover", coverHTML: "<h1>Cover</h1>"}
		conv := &mockConverter{err: errors.New("chromium unavailable")}

		w := NewWorker(ms, gen, conv, t.TempDir(), 0, nil)
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
		job4 := &models.Job{ID: 4, UserID: 10, ExternalID: "xyz", Source: "serpapi", Title: "T", Company: "C", Status: models.StatusApproved}
		ms := newMockWorkerStore(job4).withUser(userWithResume(10, "# My Resume"))
		gen := &mockGenerator{err: errors.New("api down")}
		conv := &mockConverter{}

		w := NewWorker(ms, gen, conv, t.TempDir(), 0, nil)
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

		w := NewWorker(ms, gen, conv, t.TempDir(), 0, nil)
		if err := w.processApproved(ctx); err != nil {
			t.Fatalf("processApproved() error = %v", err)
		}
		if len(ms.updates) != 0 {
			t.Errorf("expected 0 status updates, got %d", len(ms.updates))
		}
	})

	t.Run("per-user DB resume is used when available", func(t *testing.T) {
		job5 := &models.Job{ID: 5, UserID: 20, ExternalID: "per-user", Source: "serpapi", Title: "T", Company: "C", Status: models.StatusApproved}
		ms := newMockWorkerStore(job5).withUser(userWithResume(20, "# DB Resume"))

		var capturedResume string
		capGen := &captureGenerator{
			onGenerate: func(_ context.Context, _ models.Job, resume string) (string, string, string, string, error) {
				capturedResume = resume
				return "# R", "<h1>R</h1>", "# C", "<h1>C</h1>", nil
			},
		}

		w := NewWorker(ms, capGen, nil, t.TempDir(), 0, nil)
		w.processApproved(ctx)

		if capturedResume != "# DB Resume" {
			t.Errorf("Generate received resume = %q, want %q", capturedResume, "# DB Resume")
		}
		lastStatus := ms.updates[len(ms.updates)-1].status
		if lastStatus != models.StatusComplete {
			t.Errorf("last status = %q, want complete", lastStatus)
		}
	})

	t.Run("empty DB resume fails the job", func(t *testing.T) {
		job6 := &models.Job{ID: 6, UserID: 30, ExternalID: "no-resume", Source: "serpapi", Title: "T", Company: "C", Status: models.StatusApproved}
		ms := newMockWorkerStore(job6).withUser(userWithResume(30, ""))
		gen := &mockGenerator{resumeMD: "# R", resumeHTML: "<h1>R</h1>"}
		conv := &mockConverter{}

		w := NewWorker(ms, gen, conv, t.TempDir(), 0, nil)
		w.processApproved(ctx)

		lastStatus := ms.updates[len(ms.updates)-1].status
		if lastStatus != models.StatusFailed {
			t.Errorf("last status = %q, want failed (no resume)", lastStatus)
		}
		// Generate should not have been called.
		if len(ms.generated) != 0 {
			t.Errorf("generated updates = %d, want 0 (generation should be skipped)", len(ms.generated))
		}
	})

	t.Run("user not found in store fails the job", func(t *testing.T) {
		job7 := &models.Job{ID: 7, UserID: 99, ExternalID: "no-user", Source: "serpapi", Title: "T", Company: "C", Status: models.StatusApproved}
		// No user with ID 99 registered in the store.
		ms := newMockWorkerStore(job7)
		gen := &mockGenerator{}
		conv := &mockConverter{}

		w := NewWorker(ms, gen, conv, t.TempDir(), 0, nil)
		w.processApproved(ctx)

		lastStatus := ms.updates[len(ms.updates)-1].status
		if lastStatus != models.StatusFailed {
			t.Errorf("last status = %q, want failed (user not found)", lastStatus)
		}
	})

	t.Run("job with UserID=0 is aborted without calling Generate", func(t *testing.T) {
		// Jobs with UserID=0 have no owner and must be failed immediately.
		// This guards against orphaned jobs that could never have a resume loaded.
		job8 := &models.Job{ID: 8, UserID: 0, ExternalID: "orphan", Source: "serpapi", Title: "T", Company: "C", Status: models.StatusApproved}
		ms := newMockWorkerStore(job8)
		gen := &mockGenerator{resumeMD: "# R", resumeHTML: "<h1>R</h1>", coverMD: "# C", coverHTML: "<h1>C</h1>"}
		conv := &mockConverter{}

		w := NewWorker(ms, gen, conv, t.TempDir(), 0, nil)
		w.processApproved(ctx)

		lastStatus := ms.updates[len(ms.updates)-1].status
		if lastStatus != models.StatusFailed {
			t.Errorf("last status = %q, want failed (UserID=0)", lastStatus)
		}
		// Generate should never have been called — no generated updates expected.
		if len(ms.generated) != 0 {
			t.Errorf("generated updates = %d, want 0 (generation must be skipped for UserID=0)", len(ms.generated))
		}
	})

	t.Run("multiple approved jobs are all processed", func(t *testing.T) {
		jobA := &models.Job{ID: 10, UserID: 50, ExternalID: "a", Source: "serpapi", Title: "TA", Company: "CA", Status: models.StatusApproved}
		jobB := &models.Job{ID: 11, UserID: 50, ExternalID: "b", Source: "serpapi", Title: "TB", Company: "CB", Status: models.StatusApproved}
		ms := newMockWorkerStore(jobA, jobB).withUser(userWithResume(50, "# Resume"))
		gen := &mockGenerator{resumeMD: "# R", resumeHTML: "<h1>R</h1>", coverMD: "# C", coverHTML: "<h1>C</h1>"}

		w := NewWorker(ms, gen, nil, t.TempDir(), 0, nil)
		if err := w.processApproved(ctx); err != nil {
			t.Fatalf("processApproved() error = %v", err)
		}

		// Both jobs should have had UpdateJobGenerated called.
		if len(ms.generated) != 2 {
			t.Errorf("generated updates = %d, want 2 (one per job)", len(ms.generated))
		}
	})
}

// captureGenerator is a test helper that calls an arbitrary onGenerate function
// so tests can inspect the arguments passed to Generate.
type captureGenerator struct {
	onGenerate func(ctx context.Context, job models.Job, baseResume string) (string, string, string, string, error)
}

func (c *captureGenerator) Generate(ctx context.Context, job models.Job, baseResume string) (string, string, string, string, error) {
	return c.onGenerate(ctx, job, baseResume)
}

func TestWorker_Start(t *testing.T) {
	t.Run("stops cleanly on context cancellation", func(t *testing.T) {
		ms := newMockWorkerStore()
		w := NewWorker(ms, &mockGenerator{}, &mockConverter{}, t.TempDir(), 50*time.Millisecond, nil)

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
