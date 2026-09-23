package delete_archive_document

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// ArchiveDocumentRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type ArchiveDocumentRepo interface {
	DeleteArchiveDocument(ctx context.Context, id models.ID) error
}
