package create_church

import (
	"context"
	"errors"
	"fmt"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// Scenario — сценарий «создание записи церкви».
type Scenario struct {
	store ChurchStore
	ids   IDGenerator
}

// New создаёт сценарий.
func New(st ChurchStore, ids IDGenerator) *Scenario {
	return &Scenario{store: st, ids: ids}
}

// CreateChurch создаёт запись: генерирует идентификатор, проверяет
// инварианты, в одной транзакции убеждается в существовании цитат из Sources
// и сохраняет. Возвращает созданную запись с заполненным ID.
//
// Ошибки: непустой входной ID (явный идентификатор допустим только для
// импорта), невалидная сущность и несуществующая цитата —
// *models.ValidationError; прочее — ошибки хранилища как есть.
func (s *Scenario) CreateChurch(ctx context.Context, c models.Church) (models.Church, error) {
	if c.ID != "" {
		return models.Church{}, &models.ValidationError{
			Entity: models.TypeChurch,
			Field:  "id",
			Reason: fmt.Sprintf("идентификатор %q задан снаружи: при создании он генерируется", c.ID),
		}
	}

	c.ID = s.ids.New(models.TypeChurch)

	if err := c.Validate(); err != nil {
		return models.Church{}, err
	}

	err := s.store.InTx(ctx, func(tx store.Store) error {
		for i, link := range c.Sources {
			if _, err := tx.GetCitation(ctx, link.CitationID); err != nil {
				if errors.Is(err, models.ErrNotFound) {
					return sourceLinkErr(i, "цитата %q не найдена", link.CitationID)
				}

				return err
			}
		}

		return tx.SaveChurch(ctx, &c)
	})
	if err != nil {
		return models.Church{}, err
	}

	return c, nil
}

// sourceLinkErr — *models.ValidationError по полю sources[i].citation_id.
func sourceLinkErr(i int, format string, args ...any) *models.ValidationError {
	return &models.ValidationError{
		Entity: models.TypeChurch,
		Field:  fmt.Sprintf("sources[%d].citation_id", i),
		Reason: fmt.Sprintf(format, args...),
	}
}
