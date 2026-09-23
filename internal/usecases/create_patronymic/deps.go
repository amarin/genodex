package create_patronymic

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// PatronymicStore — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type PatronymicStore interface {
	SavePatronymic(ctx context.Context, s *models.Patronymic) error
}

// IDGenerator — генератор идентификаторов сущностей (реализация — internal/idgen).
type IDGenerator interface {
	New(t models.Type) models.ID
}
