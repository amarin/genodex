package create_surname

import (
	"context"
	"fmt"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «создание словарной записи фамилии».
type Scenario struct {
	store SurnameStore
	ids   IDGenerator
}

// New создаёт сценарий.
func New(st SurnameStore, ids IDGenerator) *Scenario {
	return &Scenario{store: st, ids: ids}
}

// CreateSurname создаёт запись: генерирует идентификатор, проверяет
// инварианты, сохраняет. Возвращает созданную запись с заполненным ID.
// Флат-сущность без FK — в отличие от create_division, без транзакционной
// проверки родителя.
//
// Ошибки: непустой входной ID (явный идентификатор допустим только для
// импорта) и невалидная сущность — *models.ValidationError; прочее —
// ошибки хранилища как есть.
func (s *Scenario) CreateSurname(ctx context.Context, sn models.Surname) (models.Surname, error) {
	if sn.ID != "" {
		return models.Surname{}, &models.ValidationError{
			Entity: models.TypeSurname,
			Field:  "id",
			Reason: fmt.Sprintf("идентификатор %q задан снаружи: при создании он генерируется", sn.ID),
		}
	}

	sn.ID = s.ids.New(models.TypeSurname)

	if err := sn.Validate(); err != nil {
		return models.Surname{}, err
	}

	if err := s.store.SaveSurname(ctx, &sn); err != nil {
		return models.Surname{}, err
	}

	return sn, nil
}
