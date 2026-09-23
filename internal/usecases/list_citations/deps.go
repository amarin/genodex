package list_citations

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// CitationRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -citation $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type CitationRepo interface {
	ListCitations(ctx context.Context, access models.Access, page models.Page) ([]*models.Citation, error)
}
