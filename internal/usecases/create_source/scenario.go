package create_source

import (
	"context"
	"errors"
	"fmt"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// Scenario — сценарий «создание источника».
type Scenario struct {
	store SourceStore
	ids   IDGenerator
}

// New создаёт сценарий.
func New(st SourceStore, ids IDGenerator) *Scenario {
	return &Scenario{store: st, ids: ids}
}

// CreateSource создаёт источник: генерирует идентификатор, проверяет
// инварианты, в одной транзакции (если хранилище задано) убеждается в его
// существовании и сохраняет. Возвращает созданный источник с заполненным ID.
// По образцу create_archive (одиночный опциональный strict FK RepositoryID).
//
// Ошибки: непустой входной ID, невалидная сущность и несуществующее
// хранилище — *models.ValidationError (поля id, repository_id); прочее —
// ошибки хранилища как есть.
func (s *Scenario) CreateSource(ctx context.Context, src models.Source) (models.Source, error) {
	if src.ID != "" {
		return models.Source{}, &models.ValidationError{
			Entity: models.TypeSource,
			Field:  "id",
			Reason: fmt.Sprintf("идентификатор %q задан снаружи: при создании он генерируется", src.ID),
		}
	}

	src.ID = s.ids.New(models.TypeSource)

	if err := src.Validate(); err != nil {
		return models.Source{}, err
	}

	err := s.store.InTx(ctx, func(tx store.Store) error {
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
	if err != nil {
		return models.Source{}, err
	}

	return src, nil
}

// repositoryErr — *models.ValidationError по полю repository_id.
func repositoryErr(format string, args ...any) *models.ValidationError {
	return &models.ValidationError{
		Entity: models.TypeSource,
		Field:  "repository_id",
		Reason: fmt.Sprintf(format, args...),
	}
}
