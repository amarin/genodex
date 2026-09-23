package search_patronymics

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// PatronymicRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type PatronymicRepo interface {
	Search(ctx context.Context, query string, access models.Access, page models.Page) ([]models.Hit, error)
	GetPatronymic(ctx context.Context, id models.ID) (*models.Patronymic, error)
}
