package get_division

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// DivisionRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type DivisionRepo interface {
	GetAdministrativeDivision(ctx context.Context, id models.ID) (*models.AdministrativeDivision, error)
	GetCitation(ctx context.Context, id models.ID) (*models.Citation, error)
}
