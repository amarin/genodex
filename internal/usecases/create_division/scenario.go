package create_division

import (
	"context"
	"errors"
	"fmt"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// Scenario — сценарий «создание единицы административного деления».
type Scenario struct {
	store DivisionStore
	ids   IDGenerator
}

// New создаёт сценарий.
func New(st DivisionStore, ids IDGenerator) *Scenario {
	return &Scenario{store: st, ids: ids}
}

// CreateDivision создаёт единицу деления: генерирует идентификатор, проверяет
// инварианты, в одной транзакции убеждается в существовании родителя и
// сохраняет. Возвращает созданную единицу с заполненным ID.
//
// Ошибки: непустой входной ID (явный идентификатор допустим только для
// импорта), невалидная сущность и несуществующий родитель —
// *models.ValidationError (поля id, соответствующее и parent_id); прочее —
// ошибки хранилища как есть.
func (s *Scenario) CreateDivision(ctx context.Context, d models.AdministrativeDivision) (models.AdministrativeDivision, error) {
	if d.ID != "" {
		return models.AdministrativeDivision{}, &models.ValidationError{
			Entity: models.TypeAdministrativeDivision,
			Field:  "id",
			Reason: fmt.Sprintf("идентификатор %q задан снаружи: при создании он генерируется", d.ID),
		}
	}

	d.ID = s.ids.New(models.TypeAdministrativeDivision)

	if err := d.Validate(); err != nil {
		return models.AdministrativeDivision{}, err
	}

	err := s.store.InTx(ctx, func(tx store.Store) error {
		if d.ParentID != nil {
			if _, err := tx.GetAdministrativeDivision(ctx, *d.ParentID); err != nil {
				if errors.Is(err, models.ErrNotFound) {
					return parentErr("родитель %q не найден", *d.ParentID)
				}

				return err
			}
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
	if err != nil {
		return models.AdministrativeDivision{}, err
	}

	return d, nil
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
