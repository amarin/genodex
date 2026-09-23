package search_given_names

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// GivenNameRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type GivenNameRepo interface {
	Search(ctx context.Context, query string, access models.Access, page models.Page) ([]models.Hit, error)
	GetGivenName(ctx context.Context, id models.ID) (*models.GivenName, error)
}
