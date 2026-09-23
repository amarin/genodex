package update_repository

import (
	"context"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// Scenario — сценарий «изменение записи хранилища».
type Scenario struct {
	store RepositoryStore
}

// New создаёт сценарий.
func New(st RepositoryStore) *Scenario {
	return &Scenario{store: st}
}

// UpdateRepository полностью заменяет запись по r.ID: проверяет инварианты, в
// одной транзакции убеждается, что запись существует, и сохраняет.
//
// Ошибки: невалидная сущность — *models.ValidationError; нет такой записи —
// models.ErrNotFound; прочее — ошибки хранилища как есть.
func (s *Scenario) UpdateRepository(ctx context.Context, r models.Repository) error {
	if err := r.Validate(); err != nil {
		return err
	}

	return s.store.InTx(ctx, func(tx store.Store) error {
		if _, err := tx.GetRepository(ctx, r.ID); err != nil {
			return err
		}

		return tx.SaveRepository(ctx, &r)
	})
}
