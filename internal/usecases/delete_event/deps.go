package delete_event

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// EventRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type EventRepo interface {
	DeleteEvent(ctx context.Context, id models.ID) error
}
