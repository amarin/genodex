package create_relation

import (
	"context"
	"errors"
	"fmt"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// Scenario — сценарий «создание ребра графа родства».
type Scenario struct {
	store RelationStore
	ids   IDGenerator
}

// New создаёт сценарий.
func New(st RelationStore, ids IDGenerator) *Scenario {
	return &Scenario{store: st, ids: ids}
}

// CreateRelation создаёт ребро: генерирует идентификатор, проверяет
// инварианты (Relation.Validate — в т.ч. различность person_a/person_b и
// условное правило rel_type), в одной транзакции убеждается в существовании
// обеих персон и цитат из Sources, сохраняет. Возвращает созданную запись с
// заполненным ID.
//
// Первая сущность программы с двумя строгими ссылками на один и тот же тип
// (PersonA/PersonB → Person, docs/data-model/entity-write.md §3.8): обе
// стороны проверяются своим вызовом GetPerson, но последовательно, с ранним
// return на первой же неудаче — та же форма, что и у любого usecase с
// несколькими проверками существования в программе. Если не найдены обе
// персоны, вернётся ошибка только по person_a; до проверки person_b дело не
// дойдёт.
//
// Ошибки: непустой входной ID, невалидная сущность, несуществующая персона
// (поля person_a/person_b) и несуществующая цитата — *models.ValidationError;
// прочее — ошибки хранилища как есть.
func (s *Scenario) CreateRelation(ctx context.Context, r models.Relation) (models.Relation, error) {
	if r.ID != "" {
		return models.Relation{}, &models.ValidationError{
			Entity: models.TypeRelation,
			Field:  "id",
			Reason: fmt.Sprintf("идентификатор %q задан снаружи: при создании он генерируется", r.ID),
		}
	}

	r.ID = s.ids.New(models.TypeRelation)

	if err := r.Validate(); err != nil {
		return models.Relation{}, err
	}

	err := s.store.InTx(ctx, func(tx store.Store) error {
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
	if err != nil {
		return models.Relation{}, err
	}

	return r, nil
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
