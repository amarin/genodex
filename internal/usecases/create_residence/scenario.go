package create_residence

import (
	"context"
	"errors"
	"fmt"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// Scenario — сценарий «создание проживания».
type Scenario struct {
	store ResidenceStore
	ids   IDGenerator
}

// New создаёт сценарий.
func New(st ResidenceStore, ids IDGenerator) *Scenario {
	return &Scenario{store: st, ids: ids}
}

// CreateResidence создаёт запись: генерирует идентификатор, проверяет
// инварианты, в одной транзакции убеждается в существовании персоны (person_id),
// места (place_id — строго AdministrativeDivision, не любой PlaceRef) и цитат
// из Sources, сохраняет. Возвращает созданную запись с заполненным ID.
//
// store.Store's generic SaveResidence уже подпирает эти же FK реальными SQL-
// constraint'ами — эти проверки здесь дают чистую field-specific 422 вместо
// сырой ошибки БД, по единому для программы правилу (docs/data-model/
// entity-write.md §3.8).
//
// Ошибки: непустой входной ID, невалидная сущность, несуществующая персона/
// место (поля person_id/place_id) и несуществующая цитата —
// *models.ValidationError; прочее — ошибки хранилища как есть.
func (s *Scenario) CreateResidence(ctx context.Context, r models.Residence) (models.Residence, error) {
	if r.ID != "" {
		return models.Residence{}, &models.ValidationError{
			Entity: models.TypeResidence,
			Field:  "id",
			Reason: fmt.Sprintf("идентификатор %q задан снаружи: при создании он генерируется", r.ID),
		}
	}

	r.ID = s.ids.New(models.TypeResidence)

	if err := r.Validate(); err != nil {
		return models.Residence{}, err
	}

	err := s.store.InTx(ctx, func(tx store.Store) error {
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
	if err != nil {
		return models.Residence{}, err
	}

	return r, nil
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
