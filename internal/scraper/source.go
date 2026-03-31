// Package scraper defines the Source interface and SerpAPI implementation.
package scraper

import (
	"context"

	"github.com/whinchman/jobhuntr/internal/models"
)

// Source is the interface for job search backends.
type Source interface {
	Search(ctx context.Context, filter models.SearchFilter) ([]models.Job, error)
}
