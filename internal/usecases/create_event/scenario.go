package create_event

import (
	"context"
	"errors"
	"fmt"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// Scenario — сценарий «создание события».
type Scenario struct {
	store EventStore
	ids   IDGenerator
}

// New создаёт сценарий.
func New(st EventStore, ids IDGenerator) *Scenario {
	return &Scenario{store: st, ids: ids}
}

// CreateEvent создаёт запись: генерирует идентификатор, проверяет
// инварианты (Event.Validate), в одной транзакции убеждается в существовании
// каждого участника (Participants[i].PersonID — СТРОГАЯ ссылка, в отличие от
// Place) и цитат из Sources, сохраняет. Возвращает созданную запись с
// заполненным ID.
//
// Place (*models.PlaceRef) НИКОГДА не проверяется на существование — мягкая
// ссылка, тот же принцип, что и у любого другого TextRef-подобного поля в
// программе (docs/data-model/entity-write.md §3.8); только форма/ограничение
// типа проверяется в Event.Validate (вызван до InTx).
//
// Ошибки: непустой входной ID, невалидная сущность, несуществующий участник
// (поле participants[i].person_id) и несуществующая цитата —
// *models.ValidationError; прочее — ошибки хранилища как есть.
func (s *Scenario) CreateEvent(ctx context.Context, e models.Event) (models.Event, error) {
	if e.ID != "" {
		return models.Event{}, &models.ValidationError{
			Entity: models.TypeEvent,
			Field:  "id",
			Reason: fmt.Sprintf("идентификатор %q задан снаружи: при создании он генерируется", e.ID),
		}
	}

	e.ID = s.ids.New(models.TypeEvent)

	if err := e.Validate(); err != nil {
		return models.Event{}, err
	}

	err := s.store.InTx(ctx, func(tx store.Store) error {
		for i, p := range e.Participants {
			if _, err := tx.GetPerson(ctx, p.PersonID); err != nil {
				if errors.Is(err, models.ErrNotFound) {
					return participantErr(i, "персона %q не найдена", p.PersonID)
				}

				return err
			}
		}

		for i, link := range e.Sources {
			if _, err := tx.GetCitation(ctx, link.CitationID); err != nil {
				if errors.Is(err, models.ErrNotFound) {
					return sourceLinkErr(i, "цитата %q не найдена", link.CitationID)
				}

				return err
			}
		}

		return tx.SaveEvent(ctx, &e)
	})
	if err != nil {
		return models.Event{}, err
	}

	return e, nil
}

// participantErr — *models.ValidationError по полю participants[i].person_id.
func participantErr(i int, format string, args ...any) *models.ValidationError {
	return &models.ValidationError{
		Entity: models.TypeEvent,
		Field:  fmt.Sprintf("participants[%d].person_id", i),
		Reason: fmt.Sprintf(format, args...),
	}
}

// sourceLinkErr — *models.ValidationError по полю sources[i].citation_id.
func sourceLinkErr(i int, format string, args ...any) *models.ValidationError {
	return &models.ValidationError{
		Entity: models.TypeEvent,
		Field:  fmt.Sprintf("sources[%d].citation_id", i),
		Reason: fmt.Sprintf(format, args...),
	}
}
