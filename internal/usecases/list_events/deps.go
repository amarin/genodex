package list_events

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// EventRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type EventRepo interface {
	ListEvents(ctx context.Context, access models.Access, page models.Page) ([]*models.Event, error)
	GetPerson(ctx context.Context, id models.ID) (*models.Person, error)
	GetCitation(ctx context.Context, id models.ID) (*models.Citation, error)
}
