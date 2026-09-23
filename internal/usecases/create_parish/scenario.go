package create_parish

import (
	"context"
	"errors"
	"fmt"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
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
// инварианты, в одной транзакции убеждается в существовании цитат из Sources
// и сохраняет. Возвращает созданную запись с заполненным ID.
//
// Ошибки: непустой входной ID (явный идентификатор допустим только для
// импорта), невалидная сущность и несуществующая цитата —
// *models.ValidationError; прочее — ошибки хранилища как есть.
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

	err := s.store.InTx(ctx, func(tx store.Store) error {
		for i, link := range p.Sources {
			if _, err := tx.GetCitation(ctx, link.CitationID); err != nil {
				if errors.Is(err, models.ErrNotFound) {
					return sourceLinkErr(i, "цитата %q не найдена", link.CitationID)
				}

				return err
			}
		}

		return tx.SaveParish(ctx, &p)
	})
	if err != nil {
		return models.Parish{}, err
	}

	return p, nil
}

// sourceLinkErr — *models.ValidationError по полю sources[i].citation_id.
func sourceLinkErr(i int, format string, args ...any) *models.ValidationError {
	return &models.ValidationError{
		Entity: models.TypeParish,
		Field:  fmt.Sprintf("sources[%d].citation_id", i),
		Reason: fmt.Sprintf(format, args...),
	}
}
