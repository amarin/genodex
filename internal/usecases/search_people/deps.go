package search_people

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// PersonRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type PersonRepo interface {
	Search(ctx context.Context, query string, access models.Access, page models.Page) ([]models.Hit, error)
	GetPerson(ctx context.Context, id models.ID) (*models.Person, error)
	GetCitation(ctx context.Context, id models.ID) (*models.Citation, error)
}
