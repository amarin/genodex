package list_given_names

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// GivenNameRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type GivenNameRepo interface {
	ListGivenNames(ctx context.Context, access models.Access, page models.Page) ([]*models.GivenName, error)
}
