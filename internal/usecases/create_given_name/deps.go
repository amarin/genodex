package create_given_name

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// GivenNameStore — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type GivenNameStore interface {
	SaveGivenName(ctx context.Context, s *models.GivenName) error
}

// IDGenerator — генератор идентификаторов сущностей (реализация — internal/idgen).
type IDGenerator interface {
	New(t models.Type) models.ID
}
