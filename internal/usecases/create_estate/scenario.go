package create_estate

import (
	"context"
	"fmt"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «создание словарной записи фамилии».
type Scenario struct {
	store EstateStore
	ids   IDGenerator
}

// New создаёт сценарий.
func New(st EstateStore, ids IDGenerator) *Scenario {
	return &Scenario{store: st, ids: ids}
}

// CreateEstate создаёт запись: генерирует идентификатор, проверяет
// инварианты, сохраняет. Возвращает созданную запись с заполненным ID.
// Флат-сущность без FK — в отличие от create_division, без транзакционной
// проверки родителя.
//
// Ошибки: непустой входной ID (явный идентификатор допустим только для
// импорта) и невалидная сущность — *models.ValidationError; прочее —
// ошибки хранилища как есть.
func (s *Scenario) CreateEstate(ctx context.Context, sn models.Estate) (models.Estate, error) {
	if sn.ID != "" {
		return models.Estate{}, &models.ValidationError{
			Entity: models.TypeEstate,
			Field:  "id",
			Reason: fmt.Sprintf("идентификатор %q задан снаружи: при создании он генерируется", sn.ID),
		}
	}

	sn.ID = s.ids.New(models.TypeEstate)

	if err := sn.Validate(); err != nil {
		return models.Estate{}, err
	}

	if err := s.store.SaveEstate(ctx, &sn); err != nil {
		return models.Estate{}, err
	}

	return sn, nil
}
