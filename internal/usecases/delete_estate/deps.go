package delete_estate

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// EstateRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type EstateRepo interface {
	DeleteEstate(ctx context.Context, id models.ID) error
}
