package create_repository

import (
	"context"
	"fmt"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «создание словарной записи фамилии».
type Scenario struct {
	store RepositoryStore
	ids   IDGenerator
}

// New создаёт сценарий.
func New(st RepositoryStore, ids IDGenerator) *Scenario {
	return &Scenario{store: st, ids: ids}
}

// CreateRepository создаёт запись: генерирует идентификатор, проверяет
// инварианты, сохраняет. Возвращает созданную запись с заполненным ID.
// Флат-сущность без FK — в отличие от create_division, без транзакционной
// проверки родителя.
//
// Ошибки: непустой входной ID (явный идентификатор допустим только для
// импорта) и невалидная сущность — *models.ValidationError; прочее —
// ошибки хранилища как есть.
func (s *Scenario) CreateRepository(ctx context.Context, sn models.Repository) (models.Repository, error) {
	if sn.ID != "" {
		return models.Repository{}, &models.ValidationError{
			Entity: models.TypeRepository,
			Field:  "id",
			Reason: fmt.Sprintf("идентификатор %q задан снаружи: при создании он генерируется", sn.ID),
		}
	}

	sn.ID = s.ids.New(models.TypeRepository)

	if err := sn.Validate(); err != nil {
		return models.Repository{}, err
	}

	if err := s.store.SaveRepository(ctx, &sn); err != nil {
		return models.Repository{}, err
	}

	return sn, nil
}
