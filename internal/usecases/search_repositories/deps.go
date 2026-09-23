package search_repositories

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// RepositoryRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type RepositoryRepo interface {
	Search(ctx context.Context, query string, access models.Access, page models.Page) ([]models.Hit, error)
	GetRepository(ctx context.Context, id models.ID) (*models.Repository, error)
}
