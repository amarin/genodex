package create_surname

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// SurnameStore — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type SurnameStore interface {
	SaveSurname(ctx context.Context, s *models.Surname) error
}

// IDGenerator — генератор идентификаторов сущностей (реализация — internal/idgen).
type IDGenerator interface {
	New(t models.Type) models.ID
}
