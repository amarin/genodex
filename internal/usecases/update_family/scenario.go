package update_family

import (
	"context"
	"errors"
	"fmt"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// Scenario — сценарий «изменение записи рода».
type Scenario struct {
	store FamilyStore
}

// New создаёт сценарий.
func New(st FamilyStore) *Scenario {
	return &Scenario{store: st}
}

// UpdateFamily полностью заменяет запись по f.ID: проверяет инварианты, в
// одной транзакции убеждается, что запись существует, и сохраняет.
//
// Ошибки: невалидная сущность — *models.ValidationError; нет такой записи —
// models.ErrNotFound; прочее — ошибки хранилища как есть.
func (s *Scenario) UpdateFamily(ctx context.Context, f models.Family) error {
	if err := f.Validate(); err != nil {
		return err
	}

	return s.store.InTx(ctx, func(tx store.Store) error {
		if _, err := tx.GetFamily(ctx, f.ID); err != nil {
			return err
		}

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
}

// sourceLinkErr — *models.ValidationError по полю sources[i].citation_id.
func sourceLinkErr(i int, format string, args ...any) *models.ValidationError {
	return &models.ValidationError{
		Entity: models.TypeFamily,
		Field:  fmt.Sprintf("sources[%d].citation_id", i),
		Reason: fmt.Sprintf(format, args...),
	}
}
