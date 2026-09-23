package get_title

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// TitleRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type TitleRepo interface {
	GetTitle(ctx context.Context, id models.ID) (*models.Title, error)
}
