package update_patronymic

import (
	"context"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// Scenario — сценарий «изменение словарной записи фамилии».
type Scenario struct {
	store PatronymicStore
}

// New создаёт сценарий.
func New(st PatronymicStore) *Scenario {
	return &Scenario{store: st}
}

// UpdatePatronymic полностью заменяет запись по sn.ID: проверяет инварианты, в
// одной транзакции убеждается, что запись существует, и сохраняет.
//
// Ошибки: невалидная сущность — *models.ValidationError; нет такой записи —
// models.ErrNotFound; прочее — ошибки хранилища как есть.
func (s *Scenario) UpdatePatronymic(ctx context.Context, sn models.Patronymic) error {
	if err := sn.Validate(); err != nil {
		return err
	}

	return s.store.InTx(ctx, func(tx store.Store) error {
		if _, err := tx.GetPatronymic(ctx, sn.ID); err != nil {
			return err
		}

		return tx.SavePatronymic(ctx, &sn)
	})
}
