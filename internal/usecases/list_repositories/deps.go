package list_repositories

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// RepositoryRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type RepositoryRepo interface {
	ListRepositories(ctx context.Context, access models.Access, page models.Page) ([]*models.Repository, error)
	GetCitation(ctx context.Context, id models.ID) (*models.Citation, error)
}
