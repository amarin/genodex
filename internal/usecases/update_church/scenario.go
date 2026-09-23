package update_church

import (
	"context"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// Scenario — сценарий «изменение записи церкви».
type Scenario struct {
	store ChurchStore
}

// New создаёт сценарий.
func New(st ChurchStore) *Scenario {
	return &Scenario{store: st}
}

// UpdateChurch полностью заменяет запись по c.ID: проверяет инварианты, в
// одной транзакции убеждается, что запись существует, и сохраняет.
//
// Ошибки: невалидная сущность — *models.ValidationError; нет такой записи —
// models.ErrNotFound; прочее — ошибки хранилища как есть.
func (s *Scenario) UpdateChurch(ctx context.Context, c models.Church) error {
	if err := c.Validate(); err != nil {
		return err
	}

	return s.store.InTx(ctx, func(tx store.Store) error {
		if _, err := tx.GetChurch(ctx, c.ID); err != nil {
			return err
		}

		return tx.SaveChurch(ctx, &c)
	})
}
