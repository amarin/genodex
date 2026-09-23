package create_parish

import (
	"context"
	"fmt"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «создание записи прихода».
type Scenario struct {
	store ParishStore
	ids   IDGenerator
}

// New создаёт сценарий.
func New(st ParishStore, ids IDGenerator) *Scenario {
	return &Scenario{store: st, ids: ids}
}

// CreateParish создаёт запись: генерирует идентификатор, проверяет
// инварианты, сохраняет. Возвращает созданную запись с заполненным ID.
// Флат-сущность без FK — в отличие от create_division, без транзакционной
// проверки родителя.
//
// Ошибки: непустой входной ID (явный идентификатор допустим только для
// импорта) и невалидная сущность — *models.ValidationError; прочее —
// ошибки хранилища как есть.
func (s *Scenario) CreateParish(ctx context.Context, p models.Parish) (models.Parish, error) {
	if p.ID != "" {
		return models.Parish{}, &models.ValidationError{
			Entity: models.TypeParish,
			Field:  "id",
			Reason: fmt.Sprintf("идентификатор %q задан снаружи: при создании он генерируется", p.ID),
		}
	}

	p.ID = s.ids.New(models.TypeParish)

	if err := p.Validate(); err != nil {
		return models.Parish{}, err
	}

	if err := s.store.SaveParish(ctx, &p); err != nil {
		return models.Parish{}, err
	}

	return p, nil
}
