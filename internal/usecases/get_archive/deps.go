package get_archive

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// ArchiveRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type ArchiveRepo interface {
	GetArchive(ctx context.Context, id models.ID) (*models.Archive, error)
}
