package create_repository

import (
	"context"
	"fmt"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «создание записи хранилища».
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
func (s *Scenario) CreateRepository(ctx context.Context, r models.Repository) (models.Repository, error) {
	if r.ID != "" {
		return models.Repository{}, &models.ValidationError{
			Entity: models.TypeRepository,
			Field:  "id",
			Reason: fmt.Sprintf("идентификатор %q задан снаружи: при создании он генерируется", r.ID),
		}
	}

	r.ID = s.ids.New(models.TypeRepository)

	if err := r.Validate(); err != nil {
		return models.Repository{}, err
	}

	if err := s.store.SaveRepository(ctx, &r); err != nil {
		return models.Repository{}, err
	}

	return r, nil
}
