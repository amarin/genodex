package get_given_name

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// GivenNameRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type GivenNameRepo interface {
	GetGivenName(ctx context.Context, id models.ID) (*models.GivenName, error)
}
