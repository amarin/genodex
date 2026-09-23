package search_families

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// FamilyRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type FamilyRepo interface {
	Search(ctx context.Context, query string, access models.Access, page models.Page) ([]models.Hit, error)
	GetFamily(ctx context.Context, id models.ID) (*models.Family, error)
}
