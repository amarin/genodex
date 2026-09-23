package search_archives

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// ArchiveRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type ArchiveRepo interface {
	Search(ctx context.Context, query string, access models.Access, page models.Page) ([]models.Hit, error)
	GetArchive(ctx context.Context, id models.ID) (*models.Archive, error)
}
