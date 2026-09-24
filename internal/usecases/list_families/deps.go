package list_families

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// FamilyRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type FamilyRepo interface {
	ListFamilies(ctx context.Context, access models.Access, page models.Page) ([]*models.Family, error)
	GetCitation(ctx context.Context, id models.ID) (*models.Citation, error)
}
