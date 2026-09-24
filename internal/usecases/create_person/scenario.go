package create_person

import (
	"context"
	"errors"
	"fmt"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// Scenario — сценарий «создание персоны».
type Scenario struct {
	store PersonStore
	ids   IDGenerator
}

// New создаёт сценарий.
func New(st PersonStore, ids IDGenerator) *Scenario {
	return &Scenario{store: st, ids: ids}
}

// CreatePerson создаёт запись: генерирует идентификатор, проверяет
// инварианты, в одной транзакции убеждается в существовании цитат из Sources
// и сохраняет. Возвращает созданную запись с заполненным ID.
//
// Единственная строго проверяемая ссылка — Sources[i].CitationID. Все
// остальные ссылки (Names[i].Surname/.Given/.Patronymic, Estates, Titles,
// Nicknames) — мягкие TextRef и НЕ проверяются на существование, как и у
// любого другого списка TextRef в программе (docs/data-model/entity-write.md).
//
// Ошибки: непустой входной ID (явный идентификатор допустим только для
// импорта), невалидная сущность и несуществующая цитата —
// *models.ValidationError; прочее — ошибки хранилища как есть.
func (s *Scenario) CreatePerson(ctx context.Context, p models.Person) (models.Person, error) {
	if p.ID != "" {
		return models.Person{}, &models.ValidationError{
			Entity: models.TypePerson,
			Field:  "id",
			Reason: fmt.Sprintf("идентификатор %q задан снаружи: при создании он генерируется", p.ID),
		}
	}

	p.ID = s.ids.New(models.TypePerson)

	if err := p.Validate(); err != nil {
		return models.Person{}, err
	}

	err := s.store.InTx(ctx, func(tx store.Store) error {
		for i, link := range p.Sources {
			if _, err := tx.GetCitation(ctx, link.CitationID); err != nil {
				if errors.Is(err, models.ErrNotFound) {
					return sourceLinkErr(i, "цитата %q не найдена", link.CitationID)
				}

				return err
			}
		}

		return tx.SavePerson(ctx, &p)
	})
	if err != nil {
		return models.Person{}, err
	}

	return p, nil
}

// sourceLinkErr — *models.ValidationError по полю sources[i].citation_id.
func sourceLinkErr(i int, format string, args ...any) *models.ValidationError {
	return &models.ValidationError{
		Entity: models.TypePerson,
		Field:  fmt.Sprintf("sources[%d].citation_id", i),
		Reason: fmt.Sprintf(format, args...),
	}
}
