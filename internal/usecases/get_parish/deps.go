package get_parish

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// ParishRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type ParishRepo interface {
	GetParish(ctx context.Context, id models.ID) (*models.Parish, error)
	GetCitation(ctx context.Context, id models.ID) (*models.Citation, error)
}
