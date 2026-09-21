package list_settlements

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// AdminDivisionRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type AdminDivisionRepo interface {
	ListAdministrativeDivisions(ctx context.Context) ([]*models.AdministrativeDivision, error)
}
