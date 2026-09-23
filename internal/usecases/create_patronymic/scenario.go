package create_patronymic

import (
	"context"
	"fmt"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «создание словарной записи отчества».
type Scenario struct {
	store PatronymicStore
	ids   IDGenerator
}

// New создаёт сценарий.
func New(st PatronymicStore, ids IDGenerator) *Scenario {
	return &Scenario{store: st, ids: ids}
}

// CreatePatronymic создаёт запись: генерирует идентификатор, проверяет
// инварианты, сохраняет. Возвращает созданную запись с заполненным ID.
// Флат-сущность без FK — в отличие от create_division, без транзакционной
// проверки родителя.
//
// Ошибки: непустой входной ID (явный идентификатор допустим только для
// импорта) и невалидная сущность — *models.ValidationError; прочее —
// ошибки хранилища как есть.
func (s *Scenario) CreatePatronymic(ctx context.Context, sn models.Patronymic) (models.Patronymic, error) {
	if sn.ID != "" {
		return models.Patronymic{}, &models.ValidationError{
			Entity: models.TypePatronymic,
			Field:  "id",
			Reason: fmt.Sprintf("идентификатор %q задан снаружи: при создании он генерируется", sn.ID),
		}
	}

	sn.ID = s.ids.New(models.TypePatronymic)

	if err := sn.Validate(); err != nil {
		return models.Patronymic{}, err
	}

	if err := s.store.SavePatronymic(ctx, &sn); err != nil {
		return models.Patronymic{}, err
	}

	return sn, nil
}
