package delete_residence

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// ResidenceRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type ResidenceRepo interface {
	DeleteResidence(ctx context.Context, id models.ID) error
}
