package search_archive_documents

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// ArchiveDocumentRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type ArchiveDocumentRepo interface {
	Search(ctx context.Context, query string, access models.Access, page models.Page) ([]models.Hit, error)
	GetArchiveDocument(ctx context.Context, id models.ID) (*models.ArchiveDocument, error)
	GetCitation(ctx context.Context, id models.ID) (*models.Citation, error)
}
