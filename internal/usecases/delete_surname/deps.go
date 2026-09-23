package delete_surname

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// SurnameRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type SurnameRepo interface {
	DeleteSurname(ctx context.Context, id models.ID) error
}
