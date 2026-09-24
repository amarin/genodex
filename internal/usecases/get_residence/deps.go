package get_residence

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// ResidenceRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type ResidenceRepo interface {
	GetResidence(ctx context.Context, id models.ID) (*models.Residence, error)
	GetPerson(ctx context.Context, id models.ID) (*models.Person, error)
	GetCitation(ctx context.Context, id models.ID) (*models.Citation, error)
}
