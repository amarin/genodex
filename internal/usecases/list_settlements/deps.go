package list_settlements

import "github.com/amarin/genodex/internal/models"

// AdminDivisionRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type AdminDivisionRepo interface {
	ListAdministrativeDivisions() ([]*models.AdministrativeDivision, error)
}
