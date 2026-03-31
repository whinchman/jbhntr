package generator

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/whinchman/jobhuntr/internal/models"
	"github.com/whinchman/jobhuntr/internal/pdf"
	"github.com/whinchman/jobhuntr/internal/store"
)

// WorkerStore is the subset of store.Store the Worker needs.
type WorkerStore interface {
	GetJob(ctx context.Context, id int64) (*models.Job, error)
	ListJobs(ctx context.Context, f store.ListJobsFilter) ([]models.Job, error)
	UpdateJobStatus(ctx context.Context, id int64, status models.JobStatus) error
	UpdateJobGenerated(ctx context.Context, id int64, resumeHTML, coverHTML, resumePDF, coverPDF string) error
	UpdateJobError(ctx context.Context, id int64, errMsg string) error
}

// Worker polls for approved jobs, generates documents, and converts them to PDF.
type Worker struct {
	store        WorkerStore
	generator    Generator
	converter    pdf.Converter
	outputDir    string
	resumePath   string
	pollInterval time.Duration
	logger       *slog.Logger
}

// NewWorker creates a Worker. pollInterval=0 defaults to 30s.
func NewWorker(store WorkerStore, gen Generator, conv pdf.Converter, outputDir, resumePath string, pollInterval time.Duration, logger *slog.Logger) *Worker {
	if pollInterval <= 0 {
		pollInterval = 30 * time.Second
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Worker{
		store:        store,
		generator:    gen,
		converter:    conv,
		outputDir:    outputDir,
		resumePath:   resumePath,
		pollInterval: pollInterval,
		logger:       logger,
	}
}

// Start runs the worker loop until ctx is cancelled.
func (w *Worker) Start(ctx context.Context) {
	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()
	w.logger.Info("generator worker started", "interval", w.pollInterval)
	for {
		select {
		case <-ctx.Done():
			w.logger.Info("generator worker stopped")
			return
		case <-ticker.C:
			if err := w.processApproved(ctx); err != nil {
				w.logger.Error("worker process error", "error", err)
			}
		}
	}
}

// processApproved queries for approved jobs and generates documents for each.
func (w *Worker) processApproved(ctx context.Context) error {
	jobs, err := w.store.ListJobs(ctx, store.ListJobsFilter{Status: models.StatusApproved})
	if err != nil {
		return fmt.Errorf("worker: list approved jobs: %w", err)
	}

	for _, job := range jobs {
		w.processJob(ctx, job)
	}
	return nil
}

func (w *Worker) processJob(ctx context.Context, job models.Job) {
	log := w.logger.With("job_id", job.ID, "title", job.Title)

	if err := w.store.UpdateJobStatus(ctx, job.ID, models.StatusGenerating); err != nil {
		log.Error("failed to set status generating", "error", err)
		return
	}

	baseResume := ""
	if w.resumePath != "" {
		data, err := os.ReadFile(w.resumePath)
		if err != nil {
			log.Error("failed to read base resume", "error", err, "path", w.resumePath)
		} else {
			baseResume = string(data)
		}
	}

	resumeHTML, coverHTML, err := w.generator.Generate(ctx, job, baseResume)
	if err != nil {
		log.Error("generation failed", "error", err)
		w.failJob(ctx, job.ID, err.Error())
		return
	}

	jobDir := filepath.Join(w.outputDir, fmt.Sprintf("%d", job.ID))
	resumePDF := filepath.Join(jobDir, "resume.pdf")
	coverPDF := filepath.Join(jobDir, "cover_letter.pdf")

	if err := w.converter.PDFFromHTML(ctx, resumeHTML, resumePDF); err != nil {
		log.Error("resume pdf conversion failed", "error", err)
		w.failJob(ctx, job.ID, err.Error())
		return
	}

	if err := w.converter.PDFFromHTML(ctx, coverHTML, coverPDF); err != nil {
		log.Error("cover letter pdf conversion failed", "error", err)
		w.failJob(ctx, job.ID, err.Error())
		return
	}

	if err := w.store.UpdateJobGenerated(ctx, job.ID, resumeHTML, coverHTML, resumePDF, coverPDF); err != nil {
		log.Error("failed to save generated paths", "error", err)
		w.failJob(ctx, job.ID, err.Error())
		return
	}

	if err := w.store.UpdateJobStatus(ctx, job.ID, models.StatusComplete); err != nil {
		log.Error("failed to set status complete", "error", err)
		return
	}

	log.Info("job generation complete", "resume_pdf", resumePDF, "cover_pdf", coverPDF)
}

func (w *Worker) failJob(ctx context.Context, id int64, errMsg string) {
	if err := w.store.UpdateJobError(ctx, id, errMsg); err != nil {
		w.logger.Error("failed to set job error", "job_id", id, "error", err)
	}
}
