package delete_citation

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// CitationRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type CitationRepo interface {
	DeleteCitation(ctx context.Context, id models.ID) error
}
