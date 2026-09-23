package list_archives

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// ArchiveRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type ArchiveRepo interface {
	ListArchives(ctx context.Context, access models.Access, page models.Page) ([]*models.Archive, error)
}
