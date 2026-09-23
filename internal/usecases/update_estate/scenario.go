package update_estate

import (
	"context"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// Scenario — сценарий «изменение словарной записи сословия».
type Scenario struct {
	store EstateStore
}

// New создаёт сценарий.
func New(st EstateStore) *Scenario {
	return &Scenario{store: st}
}

// UpdateEstate полностью заменяет запись по sn.ID: проверяет инварианты, в
// одной транзакции убеждается, что запись существует, и сохраняет.
//
// Ошибки: невалидная сущность — *models.ValidationError; нет такой записи —
// models.ErrNotFound; прочее — ошибки хранилища как есть.
func (s *Scenario) UpdateEstate(ctx context.Context, sn models.Estate) error {
	if err := sn.Validate(); err != nil {
		return err
	}

	return s.store.InTx(ctx, func(tx store.Store) error {
		if _, err := tx.GetEstate(ctx, sn.ID); err != nil {
			return err
		}

		return tx.SaveEstate(ctx, &sn)
	})
}
