package create_archive_document

import (
	"context"
	"errors"
	"fmt"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// Scenario — сценарий «создание документа внутри единицы учёта».
type Scenario struct {
	store ArchiveDocumentStore
	ids   IDGenerator
}

// New создаёт сценарий.
func New(st ArchiveDocumentStore, ids IDGenerator) *Scenario {
	return &Scenario{store: st, ids: ids}
}

// CreateArchiveDocument создаёт документ: генерирует идентификатор,
// проверяет инварианты, в одной транзакции убеждается в существовании
// единицы учёта (UnitID — обязательная ссылка на ArchiveNode) и сохраняет.
// Возвращает созданный документ с заполненным ID.
//
// Ошибки: непустой входной ID, невалидная сущность, несуществующая единица
// учёта — *models.ValidationError (поля id, unit_id); прочее — ошибки
// хранилища как есть.
func (s *Scenario) CreateArchiveDocument(ctx context.Context, d models.ArchiveDocument) (models.ArchiveDocument, error) {
	if d.ID != "" {
		return models.ArchiveDocument{}, &models.ValidationError{
			Entity: models.TypeArchiveDocument,
			Field:  "id",
			Reason: fmt.Sprintf("идентификатор %q задан снаружи: при создании он генерируется", d.ID),
		}
	}

	d.ID = s.ids.New(models.TypeArchiveDocument)

	if err := d.Validate(); err != nil {
		return models.ArchiveDocument{}, err
	}

	err := s.store.InTx(ctx, func(tx store.Store) error {
		if _, err := tx.GetArchiveNode(ctx, d.UnitID); err != nil {
			if errors.Is(err, models.ErrNotFound) {
				return unitErr("единица учёта %q не найдена", d.UnitID)
			}

			return err
		}

		for i, link := range d.Sources {
			if _, err := tx.GetCitation(ctx, link.CitationID); err != nil {
				if errors.Is(err, models.ErrNotFound) {
					return sourceLinkErr(i, "цитата %q не найдена", link.CitationID)
				}

				return err
			}
		}

		return tx.SaveArchiveDocument(ctx, &d)
	})
	if err != nil {
		return models.ArchiveDocument{}, err
	}

	return d, nil
}

// unitErr — *models.ValidationError по полю unit_id.
func unitErr(format string, args ...any) *models.ValidationError {
	return &models.ValidationError{
		Entity: models.TypeArchiveDocument,
		Field:  "unit_id",
		Reason: fmt.Sprintf(format, args...),
	}
}

// sourceLinkErr — *models.ValidationError по полю sources[i].citation_id.
func sourceLinkErr(i int, format string, args ...any) *models.ValidationError {
	return &models.ValidationError{
		Entity: models.TypeArchiveDocument,
		Field:  fmt.Sprintf("sources[%d].citation_id", i),
		Reason: fmt.Sprintf(format, args...),
	}
}
