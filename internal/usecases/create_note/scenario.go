package create_note

import (
	"context"
	"errors"
	"fmt"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// Scenario — сценарий «создание заметки».
type Scenario struct {
	store NoteStore
	ids   IDGenerator
}

// New создаёт сценарий.
func New(st NoteStore, ids IDGenerator) *Scenario {
	return &Scenario{store: st, ids: ids}
}

// CreateNote создаёт заметку: генерирует идентификатор, проверяет
// инварианты, в одной транзакции (если родитель задан) убеждается в его
// существовании и сохраняет. Возвращает созданную заметку с заполненным ID.
// Цикл на создании невозможен (новый ID ещё нигде не встречается) — в
// отличие от UpdateNote, цепочка родителей не обходится, по образцу
// create_division.
//
// Ошибки: непустой входной ID (явный идентификатор допустим только для
// импорта), невалидная сущность и несуществующий родитель —
// *models.ValidationError (поля id, parent_id); прочее — ошибки хранилища
// как есть.
func (s *Scenario) CreateNote(ctx context.Context, n models.Note) (models.Note, error) {
	if n.ID != "" {
		return models.Note{}, &models.ValidationError{
			Entity: models.TypeNote,
			Field:  "id",
			Reason: fmt.Sprintf("идентификатор %q задан снаружи: при создании он генерируется", n.ID),
		}
	}

	n.ID = s.ids.New(models.TypeNote)

	if err := n.Validate(); err != nil {
		return models.Note{}, err
	}

	err := s.store.InTx(ctx, func(tx store.Store) error {
		if n.ParentID != nil {
			if _, err := tx.GetNote(ctx, *n.ParentID); err != nil {
				if errors.Is(err, models.ErrNotFound) {
					return parentErr("родитель %q не найден", *n.ParentID)
				}

				return err
			}
		}

		for i, link := range n.Sources {
			if _, err := tx.GetCitation(ctx, link.CitationID); err != nil {
				if errors.Is(err, models.ErrNotFound) {
					return sourceLinkErr(i, "цитата %q не найдена", link.CitationID)
				}

				return err
			}
		}

		return tx.SaveNote(ctx, &n)
	})
	if err != nil {
		return models.Note{}, err
	}

	return n, nil
}

// parentErr — *models.ValidationError по полю parent_id.
func parentErr(format string, args ...any) *models.ValidationError {
	return &models.ValidationError{
		Entity: models.TypeNote,
		Field:  "parent_id",
		Reason: fmt.Sprintf(format, args...),
	}
}

// sourceLinkErr — *models.ValidationError по полю sources[i].citation_id.
func sourceLinkErr(i int, format string, args ...any) *models.ValidationError {
	return &models.ValidationError{
		Entity: models.TypeNote,
		Field:  fmt.Sprintf("sources[%d].citation_id", i),
		Reason: fmt.Sprintf(format, args...),
	}
}
