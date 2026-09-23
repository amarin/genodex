package update_source

import (
	"context"
	"errors"
	"fmt"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// Scenario — сценарий «изменение источника».
type Scenario struct {
	store SourceStore
}

// New создаёт сценарий.
func New(st SourceStore) *Scenario {
	return &Scenario{store: st}
}

// UpdateSource полностью заменяет источник по src.ID: проверяет инварианты, в
// одной транзакции убеждается, что источник существует и (если хранилище
// задано) хранилище существует, и сохраняет.
//
// Ошибки: невалидная сущность и несуществующее хранилище —
// *models.ValidationError (соответствующее поле, repository_id); нет такого
// источника — models.ErrNotFound; прочее — ошибки хранилища как есть.
func (s *Scenario) UpdateSource(ctx context.Context, src models.Source) error {
	if err := src.Validate(); err != nil {
		return err
	}

	return s.store.InTx(ctx, func(tx store.Store) error {
		if _, err := tx.GetSource(ctx, src.ID); err != nil {
			return err
		}

		if src.RepositoryID != "" {
			if _, err := tx.GetRepository(ctx, src.RepositoryID); err != nil {
				if errors.Is(err, models.ErrNotFound) {
					return repositoryErr("хранилище %q не найдено", src.RepositoryID)
				}

				return err
			}
		}

		return tx.SaveSource(ctx, &src)
	})
}

// repositoryErr — *models.ValidationError по полю repository_id.
func repositoryErr(format string, args ...any) *models.ValidationError {
	return &models.ValidationError{
		Entity: models.TypeSource,
		Field:  "repository_id",
		Reason: fmt.Sprintf(format, args...),
	}
}
