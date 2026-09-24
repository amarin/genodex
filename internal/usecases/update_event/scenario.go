package update_event

import (
	"context"
	"errors"
	"fmt"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// Scenario — сценарий «изменение события».
type Scenario struct {
	store EventStore
}

// New создаёт сценарий.
func New(st EventStore) *Scenario {
	return &Scenario{store: st}
}

// UpdateEvent полностью заменяет запись по e.ID: проверяет инварианты, в
// одной транзакции убеждается, что запись существует, каждый участник
// (Participants[i].PersonID) и цитаты из Sources существуют, и сохраняет.
// Place (мягкая ссылка) не проверяется на существование — см. CreateEvent.
//
// Ошибки: невалидная сущность — *models.ValidationError; нет такой записи —
// models.ErrNotFound; несуществующий участник/цитата —
// *models.ValidationError; прочее — ошибки хранилища как есть.
func (s *Scenario) UpdateEvent(ctx context.Context, e models.Event) error {
	if err := e.Validate(); err != nil {
		return err
	}

	return s.store.InTx(ctx, func(tx store.Store) error {
		if _, err := tx.GetEvent(ctx, e.ID); err != nil {
			return err
		}

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
