package get_person

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// PersonRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type PersonRepo interface {
	GetPerson(ctx context.Context, id models.ID) (*models.Person, error)
}
