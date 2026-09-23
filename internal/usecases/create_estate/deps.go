package create_estate

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// EstateStore — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type EstateStore interface {
	SaveEstate(ctx context.Context, s *models.Estate) error
}

// IDGenerator — генератор идентификаторов сущностей (реализация — internal/idgen).
type IDGenerator interface {
	New(t models.Type) models.ID
}
