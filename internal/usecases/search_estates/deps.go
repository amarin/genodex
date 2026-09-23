package search_estates

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// EstateRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type EstateRepo interface {
	Search(ctx context.Context, query string, access models.Access, page models.Page) ([]models.Hit, error)
	GetEstate(ctx context.Context, id models.ID) (*models.Estate, error)
}
