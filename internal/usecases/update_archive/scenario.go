package update_archive

import (
	"context"
	"errors"
	"fmt"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// Scenario — сценарий «изменение архива».
type Scenario struct {
	store ArchiveStore
}

// New создаёт сценарий.
func New(st ArchiveStore) *Scenario {
	return &Scenario{store: st}
}

// UpdateArchive полностью заменяет архив по a.ID: проверяет инварианты, в
// одной транзакции убеждается, что архив существует и (если хранилище
// задано) хранилище существует, и сохраняет.
//
// Ошибки: невалидная сущность и несуществующее хранилище —
// *models.ValidationError (соответствующее поле, repository_id); нет такого
// архива — models.ErrNotFound; прочее — ошибки хранилища как есть.
func (s *Scenario) UpdateArchive(ctx context.Context, a models.Archive) error {
	if err := a.Validate(); err != nil {
		return err
	}

	return s.store.InTx(ctx, func(tx store.Store) error {
		if _, err := tx.GetArchive(ctx, a.ID); err != nil {
			return err
		}

		if a.RepositoryID != "" {
			if _, err := tx.GetRepository(ctx, a.RepositoryID); err != nil {
				if errors.Is(err, models.ErrNotFound) {
					return repositoryErr("хранилище %q не найдено", a.RepositoryID)
				}

				return err
			}
		}

		for i, link := range a.Sources {
			if _, err := tx.GetCitation(ctx, link.CitationID); err != nil {
				if errors.Is(err, models.ErrNotFound) {
					return sourceLinkErr(i, "цитата %q не найдена", link.CitationID)
				}

				return err
			}
		}

		return tx.SaveArchive(ctx, &a)
	})
}

// repositoryErr — *models.ValidationError по полю repository_id.
func repositoryErr(format string, args ...any) *models.ValidationError {
	return &models.ValidationError{
		Entity: models.TypeArchive,
		Field:  "repository_id",
		Reason: fmt.Sprintf(format, args...),
	}
}

// sourceLinkErr — *models.ValidationError по полю sources[i].citation_id.
func sourceLinkErr(i int, format string, args ...any) *models.ValidationError {
	return &models.ValidationError{
		Entity: models.TypeArchive,
		Field:  fmt.Sprintf("sources[%d].citation_id", i),
		Reason: fmt.Sprintf(format, args...),
	}
}
