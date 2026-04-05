// Package scraper defines the Source interface and SerpAPI implementation.
package scraper

import (
	"context"

	"github.com/whinchman/jobhuntr/internal/models"
)

// Source is the interface for job search backends.
type Source interface {
	// Name returns the source identifier (e.g. "serpapi", "jsearch").
	Name() string
	Search(ctx context.Context, filter models.SearchFilter) ([]models.Job, error)
}
