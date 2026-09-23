package list_parishes

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// ParishRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type ParishRepo interface {
	ListParishes(ctx context.Context, access models.Access, page models.Page) ([]*models.Parish, error)
}
