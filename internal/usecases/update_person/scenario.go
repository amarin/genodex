package update_person

import (
	"context"
	"errors"
	"fmt"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// Scenario — сценарий «изменение персоны».
type Scenario struct {
	store PersonStore
}

// New создаёт сценарий.
func New(st PersonStore) *Scenario {
	return &Scenario{store: st}
}

// UpdatePerson полностью заменяет запись по p.ID: проверяет инварианты, в
// одной транзакции убеждается, что запись существует, и сохраняет.
//
// Единственная строго проверяемая ссылка — Sources[i].CitationID (как в
// CreatePerson); Names[i].Surname/.Given/.Patronymic, Estates, Titles,
// Nicknames — мягкие TextRef, без проверки существования.
//
// Ошибки: невалидная сущность — *models.ValidationError; нет такой записи —
// models.ErrNotFound; прочее — ошибки хранилища как есть.
func (s *Scenario) UpdatePerson(ctx context.Context, p models.Person) error {
	if err := p.Validate(); err != nil {
		return err
	}

	return s.store.InTx(ctx, func(tx store.Store) error {
		if _, err := tx.GetPerson(ctx, p.ID); err != nil {
			return err
		}

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
}

// sourceLinkErr — *models.ValidationError по полю sources[i].citation_id.
func sourceLinkErr(i int, format string, args ...any) *models.ValidationError {
	return &models.ValidationError{
		Entity: models.TypePerson,
		Field:  fmt.Sprintf("sources[%d].citation_id", i),
		Reason: fmt.Sprintf(format, args...),
	}
}
