package update_note

import (
	"context"
	"errors"
	"fmt"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// Scenario — сценарий «изменение заметки».
type Scenario struct {
	store NoteStore
}

// New создаёт сценарий.
func New(st NoteStore) *Scenario {
	return &Scenario{store: st}
}

// UpdateNote полностью заменяет заметку по n.ID: проверяет инварианты, в
// одной транзакции убеждается, что заметка существует, а цепочка родителей
// не проходит через неё саму (цикл), и сохраняет.
//
// Ошибки: невалидная сущность, несуществующий родитель и цикл по parent_id —
// *models.ValidationError (соответствующее поле и parent_id); нет такой
// заметки — models.ErrNotFound; прочее — ошибки хранилища как есть.
func (s *Scenario) UpdateNote(ctx context.Context, n models.Note) error {
	if err := n.Validate(); err != nil {
		return err
	}

	return s.store.InTx(ctx, func(tx store.Store) error {
		if _, err := tx.GetNote(ctx, n.ID); err != nil {
			return err
		}

		if err := checkParentChain(ctx, tx, &n); err != nil {
			return err
		}

		return tx.SaveNote(ctx, &n)
	})
}

// checkParentChain обходит цепочку родителей n вверх: каждый предок должен
// существовать, и цепочка не должна проходить через саму заметку или
// замыкаться (защита от испорченных данных). Ошибка цепочки —
// *models.ValidationError по полю parent_id. По образцу
// update_division.checkParentChain.
func checkParentChain(ctx context.Context, tx store.Store, n *models.Note) error {
	seen := map[models.ID]bool{n.ID: true}

	for cur := n.ParentID; cur != nil; {
		if seen[*cur] {
			return parentErr("цепочка родителей %q проходит через саму заметку или замыкается на %q", n.ID, *cur)
		}
		seen[*cur] = true

		p, err := tx.GetNote(ctx, *cur)
		if err != nil {
			if errors.Is(err, models.ErrNotFound) {
				return parentErr("родитель %q не найден", *cur)
			}

			return err
		}

		cur = p.ParentID
	}

	return nil
}

// parentErr — *models.ValidationError по полю parent_id.
func parentErr(format string, args ...any) *models.ValidationError {
	return &models.ValidationError{
		Entity: models.TypeNote,
		Field:  "parent_id",
		Reason: fmt.Sprintf(format, args...),
	}
}
