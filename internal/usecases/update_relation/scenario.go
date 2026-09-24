package update_relation

import (
	"context"
	"errors"
	"fmt"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// Scenario — сценарий «изменение ребра графа родства».
type Scenario struct {
	store RelationStore
}

// New создаёт сценарий.
func New(st RelationStore) *Scenario {
	return &Scenario{store: st}
}

// UpdateRelation полностью заменяет запись по r.ID: проверяет инварианты, в
// одной транзакции убеждается, что запись существует, обе персоны (person_a/
// person_b) и цитаты из Sources существуют, и сохраняет.
//
// Ошибки: невалидная сущность — *models.ValidationError; нет такой записи —
// models.ErrNotFound; несуществующая персона/цитата — *models.ValidationError;
// прочее — ошибки хранилища как есть.
func (s *Scenario) UpdateRelation(ctx context.Context, r models.Relation) error {
	if err := r.Validate(); err != nil {
		return err
	}

	return s.store.InTx(ctx, func(tx store.Store) error {
		if _, err := tx.GetRelation(ctx, r.ID); err != nil {
			return err
		}

		if _, err := tx.GetPerson(ctx, r.PersonA); err != nil {
			if errors.Is(err, models.ErrNotFound) {
				return personErr("person_a", "персона %q не найдена", r.PersonA)
			}

			return err
		}

		if _, err := tx.GetPerson(ctx, r.PersonB); err != nil {
			if errors.Is(err, models.ErrNotFound) {
				return personErr("person_b", "персона %q не найдена", r.PersonB)
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

		return tx.SaveRelation(ctx, &r)
	})
}

// personErr — *models.ValidationError по полю person_a/person_b.
func personErr(field, format string, args ...any) *models.ValidationError {
	return &models.ValidationError{
		Entity: models.TypeRelation,
		Field:  field,
		Reason: fmt.Sprintf(format, args...),
	}
}

// sourceLinkErr — *models.ValidationError по полю sources[i].citation_id.
func sourceLinkErr(i int, format string, args ...any) *models.ValidationError {
	return &models.ValidationError{
		Entity: models.TypeRelation,
		Field:  fmt.Sprintf("sources[%d].citation_id", i),
		Reason: fmt.Sprintf(format, args...),
	}
}
