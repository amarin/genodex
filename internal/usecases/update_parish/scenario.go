package update_parish

import (
	"context"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// Scenario — сценарий «изменение записи прихода».
type Scenario struct {
	store ParishStore
}

// New создаёт сценарий.
func New(st ParishStore) *Scenario {
	return &Scenario{store: st}
}

// UpdateParish полностью заменяет запись по p.ID: проверяет инварианты, в
// одной транзакции убеждается, что запись существует, и сохраняет.
//
// Ошибки: невалидная сущность — *models.ValidationError; нет такой записи —
// models.ErrNotFound; прочее — ошибки хранилища как есть.
func (s *Scenario) UpdateParish(ctx context.Context, p models.Parish) error {
	if err := p.Validate(); err != nil {
		return err
	}

	return s.store.InTx(ctx, func(tx store.Store) error {
		if _, err := tx.GetParish(ctx, p.ID); err != nil {
			return err
		}

		return tx.SaveParish(ctx, &p)
	})
}
