package get_surname

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// SurnameRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type SurnameRepo interface {
	GetSurname(ctx context.Context, id models.ID) (*models.Surname, error)
}
