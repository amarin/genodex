package search_divisions

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// DivisionRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type DivisionRepo interface {
	Search(ctx context.Context, query string, access models.Access, page models.Page) ([]models.Hit, error)
	GetAdministrativeDivision(ctx context.Context, id models.ID) (*models.AdministrativeDivision, error)
	GetCitation(ctx context.Context, id models.ID) (*models.Citation, error)
}
