package admin

import (
	"context"

	"github.com/whinchman/jobhuntr/internal/models"
	"github.com/whinchman/jobhuntr/internal/store"
)

// AdminStore is the subset of store.Store used by the admin panel.
type AdminStore interface {
	ListAllUsers(ctx context.Context) ([]models.User, error)
	BanUser(ctx context.Context, userID int64) error
	UnbanUser(ctx context.Context, userID int64) error
	SetPasswordHash(ctx context.Context, userID int64, hash string) error
	ListAllFilters(ctx context.Context) ([]store.AdminFilter, error)
	GetAdminStats(ctx context.Context) (store.AdminStats, error)
}
