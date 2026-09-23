package list_patronymics

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// PatronymicRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type PatronymicRepo interface {
	ListPatronymics(ctx context.Context, access models.Access, page models.Page) ([]*models.Patronymic, error)
}
