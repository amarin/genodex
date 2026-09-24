package update_residence

import (
	"context"
	"errors"
	"fmt"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// Scenario — сценарий «изменение проживания».
type Scenario struct {
	store ResidenceStore
}

// New создаёт сценарий.
func New(st ResidenceStore) *Scenario {
	return &Scenario{store: st}
}

// UpdateResidence полностью заменяет запись по r.ID: проверяет инварианты, в
// одной транзакции убеждается, что запись существует, персона и место
// существуют, и цитаты из Sources существуют, и сохраняет.
//
// Ошибки: невалидная сущность — *models.ValidationError; нет такой записи —
// models.ErrNotFound; несуществующая персона/место/цитата —
// *models.ValidationError; прочее — ошибки хранилища как есть.
func (s *Scenario) UpdateResidence(ctx context.Context, r models.Residence) error {
	if err := r.Validate(); err != nil {
		return err
	}

	return s.store.InTx(ctx, func(tx store.Store) error {
		if _, err := tx.GetResidence(ctx, r.ID); err != nil {
			return err
		}

		if _, err := tx.GetPerson(ctx, r.PersonID); err != nil {
			if errors.Is(err, models.ErrNotFound) {
				return fieldErr("person_id", "персона %q не найдена", r.PersonID)
			}

			return err
		}

		if _, err := tx.GetAdministrativeDivision(ctx, r.PlaceID); err != nil {
			if errors.Is(err, models.ErrNotFound) {
				return fieldErr("place_id", "место %q не найдено", r.PlaceID)
			}

			return err
		}

		for i, link := range r.Sources {
			if _, err := tx.GetCitation(ctx, link.CitationID); err != nil {
				if errors.Is(err, models.ErrNotFound) {
					return sourceLinkErr(i, "цитата %q не найдена", link.CitationID)
				}

				return err
			}
		}

		return tx.SaveResidence(ctx, &r)
	})
}

// fieldErr — *models.ValidationError по указанному полю.
func fieldErr(field, format string, args ...any) *models.ValidationError {
	return &models.ValidationError{
		Entity: models.TypeResidence,
		Field:  field,
		Reason: fmt.Sprintf(format, args...),
	}
}

// sourceLinkErr — *models.ValidationError по полю sources[i].citation_id.
func sourceLinkErr(i int, format string, args ...any) *models.ValidationError {
	return &models.ValidationError{
		Entity: models.TypeResidence,
		Field:  fmt.Sprintf("sources[%d].citation_id", i),
		Reason: fmt.Sprintf(format, args...),
	}
}
