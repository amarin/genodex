package get_archive_document

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// ArchiveDocumentRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type ArchiveDocumentRepo interface {
	GetArchiveDocument(ctx context.Context, id models.ID) (*models.ArchiveDocument, error)
}
