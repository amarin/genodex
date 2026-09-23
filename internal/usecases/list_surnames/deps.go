package list_surnames

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// SurnameRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type SurnameRepo interface {
	ListSurnames(ctx context.Context, access models.Access, page models.Page) ([]*models.Surname, error)
}
