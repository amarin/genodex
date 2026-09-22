package update_division

import (
	"context"
	"errors"
	"fmt"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// Scenario — сценарий «изменение единицы административного деления».
type Scenario struct {
	store DivisionStore
}

// New создаёт сценарий.
func New(st DivisionStore) *Scenario {
	return &Scenario{store: st}
}

// UpdateDivision полностью заменяет единицу деления по d.ID: проверяет
// инварианты, в одной транзакции убеждается, что единица существует, а цепочка
// родителей не проходит через неё саму (цикл), и сохраняет.
//
// Ошибки: невалидная сущность, несуществующий родитель и цикл по parent_id —
// *models.ValidationError (соответствующее поле и parent_id); нет такой
// единицы — models.ErrNotFound; прочее — ошибки хранилища как есть.
func (s *Scenario) UpdateDivision(ctx context.Context, d models.AdministrativeDivision) error {
	if err := d.Validate(); err != nil {
		return err
	}

	return s.store.InTx(ctx, func(tx store.Store) error {
		if _, err := tx.GetAdministrativeDivision(ctx, d.ID); err != nil {
			return err
		}

		if err := checkParentChain(ctx, tx, &d); err != nil {
			return err
		}

		return tx.SaveAdministrativeDivision(ctx, &d)
	})
}

// checkParentChain обходит цепочку родителей d вверх: каждый предок должен
// существовать, и цепочка не должна проходить через саму единицу или
// замыкаться (защита от испорченных данных). Ошибка цепочки —
// *models.ValidationError по полю parent_id.
func checkParentChain(ctx context.Context, tx store.Store, d *models.AdministrativeDivision) error {
	seen := map[models.ID]bool{d.ID: true}

	for cur := d.ParentID; cur != nil; {
		if seen[*cur] {
			return parentErr("цепочка родителей %q проходит через саму единицу или замыкается на %q", d.ID, *cur)
		}
		seen[*cur] = true

		p, err := tx.GetAdministrativeDivision(ctx, *cur)
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
		Entity: models.TypeAdministrativeDivision,
		Field:  "parent_id",
		Reason: fmt.Sprintf(format, args...),
	}
}
