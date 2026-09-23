package create_repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// Scenario — сценарий «создание записи хранилища».
type Scenario struct {
	store RepositoryStore
	ids   IDGenerator
}

// New создаёт сценарий.
func New(st RepositoryStore, ids IDGenerator) *Scenario {
	return &Scenario{store: st, ids: ids}
}

// CreateRepository создаёт запись: генерирует идентификатор, проверяет
// инварианты, в одной транзакции убеждается в существовании цитат из Sources
// и сохраняет. Возвращает созданную запись с заполненным ID.
//
// Ошибки: непустой входной ID (явный идентификатор допустим только для
// импорта), невалидная сущность и несуществующая цитата —
// *models.ValidationError; прочее — ошибки хранилища как есть.
func (s *Scenario) CreateRepository(ctx context.Context, r models.Repository) (models.Repository, error) {
	if r.ID != "" {
		return models.Repository{}, &models.ValidationError{
			Entity: models.TypeRepository,
			Field:  "id",
			Reason: fmt.Sprintf("идентификатор %q задан снаружи: при создании он генерируется", r.ID),
		}
	}

	r.ID = s.ids.New(models.TypeRepository)

	if err := r.Validate(); err != nil {
		return models.Repository{}, err
	}

	err := s.store.InTx(ctx, func(tx store.Store) error {
		for i, link := range r.Sources {
			if _, err := tx.GetCitation(ctx, link.CitationID); err != nil {
				if errors.Is(err, models.ErrNotFound) {
					return sourceLinkErr(i, "цитата %q не найдена", link.CitationID)
				}

				return err
			}
		}

		return tx.SaveRepository(ctx, &r)
	})
	if err != nil {
		return models.Repository{}, err
	}

	return r, nil
}

// sourceLinkErr — *models.ValidationError по полю sources[i].citation_id.
func sourceLinkErr(i int, format string, args ...any) *models.ValidationError {
	return &models.ValidationError{
		Entity: models.TypeRepository,
		Field:  fmt.Sprintf("sources[%d].citation_id", i),
		Reason: fmt.Sprintf(format, args...),
	}
}
