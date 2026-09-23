package get_patronymic

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// PatronymicRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type PatronymicRepo interface {
	GetPatronymic(ctx context.Context, id models.ID) (*models.Patronymic, error)
}
