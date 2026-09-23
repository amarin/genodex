package create_church

import (
	"context"
	"fmt"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «создание словарной записи фамилии».
type Scenario struct {
	store ChurchStore
	ids   IDGenerator
}

// New создаёт сценарий.
func New(st ChurchStore, ids IDGenerator) *Scenario {
	return &Scenario{store: st, ids: ids}
}

// CreateChurch создаёт запись: генерирует идентификатор, проверяет
// инварианты, сохраняет. Возвращает созданную запись с заполненным ID.
// Флат-сущность без FK — в отличие от create_division, без транзакционной
// проверки родителя.
//
// Ошибки: непустой входной ID (явный идентификатор допустим только для
// импорта) и невалидная сущность — *models.ValidationError; прочее —
// ошибки хранилища как есть.
func (s *Scenario) CreateChurch(ctx context.Context, sn models.Church) (models.Church, error) {
	if sn.ID != "" {
		return models.Church{}, &models.ValidationError{
			Entity: models.TypeChurch,
			Field:  "id",
			Reason: fmt.Sprintf("идентификатор %q задан снаружи: при создании он генерируется", sn.ID),
		}
	}

	sn.ID = s.ids.New(models.TypeChurch)

	if err := sn.Validate(); err != nil {
		return models.Church{}, err
	}

	if err := s.store.SaveChurch(ctx, &sn); err != nil {
		return models.Church{}, err
	}

	return sn, nil
}
