package list_relations

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// RelationRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type RelationRepo interface {
	ListRelations(ctx context.Context, access models.Access, page models.Page) ([]*models.Relation, error)
	GetPerson(ctx context.Context, id models.ID) (*models.Person, error)
}
