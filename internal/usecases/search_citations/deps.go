package search_citations

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// CitationRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -citation $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type CitationRepo interface {
	Search(ctx context.Context, query string, access models.Access, page models.Page) ([]models.Hit, error)
	GetCitation(ctx context.Context, id models.ID) (*models.Citation, error)
}
