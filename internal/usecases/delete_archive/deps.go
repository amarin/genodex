package delete_archive

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// ArchiveRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type ArchiveRepo interface {
	DeleteArchive(ctx context.Context, id models.ID) error
}
