package delete_relation

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// RelationRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type RelationRepo interface {
	DeleteRelation(ctx context.Context, id models.ID) error
}
