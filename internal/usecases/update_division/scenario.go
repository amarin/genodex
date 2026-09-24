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
// родителей не проходит через неё саму (цикл), вложенность типов допустима
// (checkNesting), и сохраняет.
//
// Ошибки: невалидная сущность, несуществующий родитель, цикл по parent_id и
// недопустимая вложенность — *models.ValidationError (соответствующее поле,
// parent_id и type); нет такой
// единицы — models.ErrNotFound; прочее — ошибки хранилища как есть.
func (s *Scenario) UpdateDivision(ctx context.Context, d models.AdministrativeDivision) error {
	if err := d.Validate(); err != nil {
		return err
	}

	return s.store.InTx(ctx, func(tx store.Store) error {
		cur, err := tx.GetAdministrativeDivision(ctx, d.ID)
		if err != nil {
			return err
		}

		if err := checkParentChain(ctx, tx, &d); err != nil {
			return err
		}

		if err := checkNesting(ctx, tx, cur, &d); err != nil {
			return err
		}

		for i, link := range d.Sources {
			if _, err := tx.GetCitation(ctx, link.CitationID); err != nil {
				if errors.Is(err, models.ErrNotFound) {
					return sourceLinkErr(i, "цитата %q не найдена", link.CitationID)
				}

				return err
			}
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

// checkNesting проверяет вложенность типов (models.AdminDivisionType.CanContain),
// но только если меняется тип или родитель: правка остальных полей единицы,
// сохранённой до появления правил, не должна ломаться. Новый родитель должен
// допускать тип единицы (ошибка по parent_id, а при неизменном родителе — по
// type); при смене типа он должен допускать типы всех текущих дочерних единиц
// (ошибка по type). Родитель к этому моменту уже проверен checkParentChain.
func checkNesting(ctx context.Context, tx store.Store, cur, d *models.AdministrativeDivision) error {
	typeChanged := cur.Type != d.Type
	parentChanged := !sameID(cur.ParentID, d.ParentID)

	if !typeChanged && !parentChanged {
		return nil
	}

	if d.ParentID != nil {
		parent, err := tx.GetAdministrativeDivision(ctx, *d.ParentID)
		if err != nil {
			return err
		}

		field := "type"
		if parentChanged {
			field = "parent_id"
		}

		if e := models.NestingError(field, parent.Type, d.Type); e != nil {
			return e
		}
	}

	if !typeChanged {
		return nil
	}

	for offset := 0; ; offset += models.MaxPageLimit {
		children, err := tx.ChildrenOfDivision(ctx, d.ID, models.AccessFull,
			models.Page{Limit: models.MaxPageLimit, Offset: offset})
		if err != nil {
			return err
		}

		for _, c := range children {
			if e := models.NestingError("type", d.Type, c.Type); e != nil {
				e.Reason = fmt.Sprintf("дочерняя единица %q: %s", c.ID, e.Reason)

				return e
			}
		}

		if len(children) < models.MaxPageLimit {
			return nil
		}
	}
}

// sameID сообщает, указывают ли два необязательных id на одно и то же.
func sameID(a, b *models.ID) bool {
	if a == nil || b == nil {
		return a == b
	}

	return *a == *b
}

// parentErr — *models.ValidationError по полю parent_id.
func parentErr(format string, args ...any) *models.ValidationError {
	return &models.ValidationError{
		Entity: models.TypeAdministrativeDivision,
		Field:  "parent_id",
		Reason: fmt.Sprintf(format, args...),
	}
}

// sourceLinkErr — *models.ValidationError по полю sources[i].citation_id.
func sourceLinkErr(i int, format string, args ...any) *models.ValidationError {
	return &models.ValidationError{
		Entity: models.TypeAdministrativeDivision,
		Field:  fmt.Sprintf("sources[%d].citation_id", i),
		Reason: fmt.Sprintf(format, args...),
	}
}
