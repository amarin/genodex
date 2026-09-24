package get_church

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// ChurchRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type ChurchRepo interface {
	GetChurch(ctx context.Context, id models.ID) (*models.Church, error)
	GetCitation(ctx context.Context, id models.ID) (*models.Citation, error)
}
