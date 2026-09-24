package list_churches

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// ChurchRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type ChurchRepo interface {
	ListChurches(ctx context.Context, access models.Access, page models.Page) ([]*models.Church, error)
	GetCitation(ctx context.Context, id models.ID) (*models.Citation, error)
}
