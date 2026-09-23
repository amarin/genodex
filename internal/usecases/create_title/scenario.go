package create_title

import (
	"context"
	"fmt"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «создание словарной записи фамилии».
type Scenario struct {
	store TitleStore
	ids   IDGenerator
}

// New создаёт сценарий.
func New(st TitleStore, ids IDGenerator) *Scenario {
	return &Scenario{store: st, ids: ids}
}

// CreateTitle создаёт запись: генерирует идентификатор, проверяет
// инварианты, сохраняет. Возвращает созданную запись с заполненным ID.
// Флат-сущность без FK — в отличие от create_division, без транзакционной
// проверки родителя.
//
// Ошибки: непустой входной ID (явный идентификатор допустим только для
// импорта) и невалидная сущность — *models.ValidationError; прочее —
// ошибки хранилища как есть.
func (s *Scenario) CreateTitle(ctx context.Context, sn models.Title) (models.Title, error) {
	if sn.ID != "" {
		return models.Title{}, &models.ValidationError{
			Entity: models.TypeTitle,
			Field:  "id",
			Reason: fmt.Sprintf("идентификатор %q задан снаружи: при создании он генерируется", sn.ID),
		}
	}

	sn.ID = s.ids.New(models.TypeTitle)

	if err := sn.Validate(); err != nil {
		return models.Title{}, err
	}

	if err := s.store.SaveTitle(ctx, &sn); err != nil {
		return models.Title{}, err
	}

	return sn, nil
}
