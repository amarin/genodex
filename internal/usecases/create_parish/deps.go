package create_parish

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// ParishStore — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type ParishStore interface {
	SaveParish(ctx context.Context, s *models.Parish) error
}

// IDGenerator — генератор идентификаторов сущностей (реализация — internal/idgen).
type IDGenerator interface {
	New(t models.Type) models.ID
}
