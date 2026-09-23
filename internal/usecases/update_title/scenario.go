package update_title

import (
	"context"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// Scenario — сценарий «изменение словарной записи фамилии».
type Scenario struct {
	store TitleStore
}

// New создаёт сценарий.
func New(st TitleStore) *Scenario {
	return &Scenario{store: st}
}

// UpdateTitle полностью заменяет запись по sn.ID: проверяет инварианты, в
// одной транзакции убеждается, что запись существует, и сохраняет.
//
// Ошибки: невалидная сущность — *models.ValidationError; нет такой записи —
// models.ErrNotFound; прочее — ошибки хранилища как есть.
func (s *Scenario) UpdateTitle(ctx context.Context, sn models.Title) error {
	if err := sn.Validate(); err != nil {
		return err
	}

	return s.store.InTx(ctx, func(tx store.Store) error {
		if _, err := tx.GetTitle(ctx, sn.ID); err != nil {
			return err
		}

		return tx.SaveTitle(ctx, &sn)
	})
}
