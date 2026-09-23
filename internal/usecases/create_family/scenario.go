package create_family

import (
	"context"
	"errors"
	"fmt"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// Scenario — сценарий «создание записи рода».
type Scenario struct {
	store FamilyStore
	ids   IDGenerator
}

// New создаёт сценарий.
func New(st FamilyStore, ids IDGenerator) *Scenario {
	return &Scenario{store: st, ids: ids}
}

// CreateFamily создаёт запись: генерирует идентификатор, проверяет
// инварианты, в одной транзакции убеждается в существовании цитат из Sources
// и сохраняет. Возвращает созданную запись с заполненным ID.
//
// Ошибки: непустой входной ID (явный идентификатор допустим только для
// импорта), невалидная сущность и несуществующая цитата —
// *models.ValidationError; прочее — ошибки хранилища как есть.
func (s *Scenario) CreateFamily(ctx context.Context, f models.Family) (models.Family, error) {
	if f.ID != "" {
		return models.Family{}, &models.ValidationError{
			Entity: models.TypeFamily,
			Field:  "id",
			Reason: fmt.Sprintf("идентификатор %q задан снаружи: при создании он генерируется", f.ID),
		}
	}

	f.ID = s.ids.New(models.TypeFamily)

	if err := f.Validate(); err != nil {
		return models.Family{}, err
	}

	err := s.store.InTx(ctx, func(tx store.Store) error {
		for i, link := range f.Sources {
			if _, err := tx.GetCitation(ctx, link.CitationID); err != nil {
				if errors.Is(err, models.ErrNotFound) {
					return sourceLinkErr(i, "цитата %q не найдена", link.CitationID)
				}

				return err
			}
		}

		return tx.SaveFamily(ctx, &f)
	})
	if err != nil {
		return models.Family{}, err
	}

	return f, nil
}

// sourceLinkErr — *models.ValidationError по полю sources[i].citation_id.
func sourceLinkErr(i int, format string, args ...any) *models.ValidationError {
	return &models.ValidationError{
		Entity: models.TypeFamily,
		Field:  fmt.Sprintf("sources[%d].citation_id", i),
		Reason: fmt.Sprintf(format, args...),
	}
}
