package list_divisions

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// AdminDivisionRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type AdminDivisionRepo interface {
	ListAdministrativeDivisions(ctx context.Context, access models.Access, page models.Page) ([]*models.AdministrativeDivision, error)
	ChildrenOfDivision(ctx context.Context, parent models.ID, access models.Access, page models.Page) ([]*models.AdministrativeDivision, error)
}
