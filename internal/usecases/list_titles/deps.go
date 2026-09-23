package list_titles

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// TitleRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type TitleRepo interface {
	ListTitles(ctx context.Context, access models.Access, page models.Page) ([]*models.Title, error)
}
