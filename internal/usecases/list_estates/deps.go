package list_estates

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// EstateRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type EstateRepo interface {
	ListEstates(ctx context.Context, access models.Access, page models.Page) ([]*models.Estate, error)
}
