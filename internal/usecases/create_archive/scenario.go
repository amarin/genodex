package create_archive

import (
	"context"
	"errors"
	"fmt"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// Scenario — сценарий «создание архива».
type Scenario struct {
	store ArchiveStore
	ids   IDGenerator
}

// New создаёт сценарий.
func New(st ArchiveStore, ids IDGenerator) *Scenario {
	return &Scenario{store: st, ids: ids}
}

// CreateArchive создаёт архив: генерирует идентификатор, проверяет инварианты,
// в одной транзакции (если хранилище задано) убеждается в его существовании и
// сохраняет. Возвращает созданный архив с заполненным ID. По образцу
// create_division's проверки родителя, но для одиночного strict FK
// (RepositoryID), а не self-referencing иерархии.
//
// Ошибки: непустой входной ID (явный идентификатор допустим только для
// импорта), невалидная сущность и несуществующее хранилище —
// *models.ValidationError (поля id, repository_id); прочее — ошибки хранилища
// как есть.
func (s *Scenario) CreateArchive(ctx context.Context, a models.Archive) (models.Archive, error) {
	if a.ID != "" {
		return models.Archive{}, &models.ValidationError{
			Entity: models.TypeArchive,
			Field:  "id",
			Reason: fmt.Sprintf("идентификатор %q задан снаружи: при создании он генерируется", a.ID),
		}
	}

	a.ID = s.ids.New(models.TypeArchive)

	if err := a.Validate(); err != nil {
		return models.Archive{}, err
	}

	err := s.store.InTx(ctx, func(tx store.Store) error {
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
	if err != nil {
		return models.Archive{}, err
	}

	return a, nil
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
