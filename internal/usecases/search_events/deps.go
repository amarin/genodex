package search_events

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// EventRepo — зависимость сценария: поиск по общему индексу + чтение по id.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type EventRepo interface {
	Search(ctx context.Context, text string, access models.Access, page models.Page) ([]models.Hit, error)
	GetEvent(ctx context.Context, id models.ID) (*models.Event, error)
	GetPerson(ctx context.Context, id models.ID) (*models.Person, error)
}
