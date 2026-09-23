package update_given_name

import (
	"context"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// Scenario — сценарий «изменение словарной записи имени».
type Scenario struct {
	store GivenNameStore
}

// New создаёт сценарий.
func New(st GivenNameStore) *Scenario {
	return &Scenario{store: st}
}

// UpdateGivenName полностью заменяет запись по sn.ID: проверяет инварианты, в
// одной транзакции убеждается, что запись существует, и сохраняет.
//
// Ошибки: невалидная сущность — *models.ValidationError; нет такой записи —
// models.ErrNotFound; прочее — ошибки хранилища как есть.
func (s *Scenario) UpdateGivenName(ctx context.Context, sn models.GivenName) error {
	if err := sn.Validate(); err != nil {
		return err
	}

	return s.store.InTx(ctx, func(tx store.Store) error {
		if _, err := tx.GetGivenName(ctx, sn.ID); err != nil {
			return err
		}

		return tx.SaveGivenName(ctx, &sn)
	})
}
