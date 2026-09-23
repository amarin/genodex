package get_citation

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// CitationRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type CitationRepo interface {
	GetCitation(ctx context.Context, id models.ID) (*models.Citation, error)
}
