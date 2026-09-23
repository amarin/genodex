package create_repository

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// RepositoryStore — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type RepositoryStore interface {
	SaveRepository(ctx context.Context, s *models.Repository) error
}

// IDGenerator — генератор идентификаторов сущностей (реализация — internal/idgen).
type IDGenerator interface {
	New(t models.Type) models.ID
}
