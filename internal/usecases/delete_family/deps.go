package delete_family

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// FamilyRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type FamilyRepo interface {
	DeleteFamily(ctx context.Context, id models.ID) error
}
