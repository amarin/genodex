package create_church

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// ChurchStore — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type ChurchStore interface {
	SaveChurch(ctx context.Context, s *models.Church) error
}

// IDGenerator — генератор идентификаторов сущностей (реализация — internal/idgen).
type IDGenerator interface {
	New(t models.Type) models.ID
}
