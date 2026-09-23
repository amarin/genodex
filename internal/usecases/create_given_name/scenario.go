package create_given_name

import (
	"context"
	"fmt"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «создание словарной записи фамилии».
type Scenario struct {
	store GivenNameStore
	ids   IDGenerator
}

// New создаёт сценарий.
func New(st GivenNameStore, ids IDGenerator) *Scenario {
	return &Scenario{store: st, ids: ids}
}

// CreateGivenName создаёт запись: генерирует идентификатор, проверяет
// инварианты, сохраняет. Возвращает созданную запись с заполненным ID.
// Флат-сущность без FK — в отличие от create_division, без транзакционной
// проверки родителя.
//
// Ошибки: непустой входной ID (явный идентификатор допустим только для
// импорта) и невалидная сущность — *models.ValidationError; прочее —
// ошибки хранилища как есть.
func (s *Scenario) CreateGivenName(ctx context.Context, sn models.GivenName) (models.GivenName, error) {
	if sn.ID != "" {
		return models.GivenName{}, &models.ValidationError{
			Entity: models.TypeGivenName,
			Field:  "id",
			Reason: fmt.Sprintf("идентификатор %q задан снаружи: при создании он генерируется", sn.ID),
		}
	}

	sn.ID = s.ids.New(models.TypeGivenName)

	if err := sn.Validate(); err != nil {
		return models.GivenName{}, err
	}

	if err := s.store.SaveGivenName(ctx, &sn); err != nil {
		return models.GivenName{}, err
	}

	return sn, nil
}
