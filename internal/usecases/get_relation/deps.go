package get_relation

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// RelationRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type RelationRepo interface {
	GetRelation(ctx context.Context, id models.ID) (*models.Relation, error)
	GetPerson(ctx context.Context, id models.ID) (*models.Person, error)
	GetCitation(ctx context.Context, id models.ID) (*models.Citation, error)
}
