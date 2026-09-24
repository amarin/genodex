package list_people

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// PersonRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type PersonRepo interface {
	ListPeople(ctx context.Context, access models.Access, page models.Page) ([]*models.Person, error)
}
