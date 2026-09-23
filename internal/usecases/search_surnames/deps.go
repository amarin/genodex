package search_surnames

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// SurnameRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type SurnameRepo interface {
	Search(ctx context.Context, query string, access models.Access, page models.Page) ([]models.Hit, error)
	GetSurname(ctx context.Context, id models.ID) (*models.Surname, error)
}
