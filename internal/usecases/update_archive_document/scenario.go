package update_archive_document

import (
	"context"
	"errors"
	"fmt"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// Scenario — сценарий «изменение документа внутри единицы учёта».
type Scenario struct {
	store ArchiveDocumentStore
}

// New создаёт сценарий.
func New(st ArchiveDocumentStore) *Scenario {
	return &Scenario{store: st}
}

// UpdateArchiveDocument полностью заменяет документ по d.ID: проверяет
// инварианты, в одной транзакции убеждается, что документ существует и
// (если задан) единица учёта существует, и сохраняет. ArchiveDocument не
// самореферентен — обхода цепочки на цикл, в отличие от
// update_archive_node, не требуется.
//
// Ошибки: невалидная сущность и несуществующая единица учёта —
// *models.ValidationError (поля соответствующего поля, unit_id); нет такого
// документа — models.ErrNotFound; прочее — ошибки хранилища как есть.
func (s *Scenario) UpdateArchiveDocument(ctx context.Context, d models.ArchiveDocument) error {
	if err := d.Validate(); err != nil {
		return err
	}

	return s.store.InTx(ctx, func(tx store.Store) error {
		if _, err := tx.GetArchiveDocument(ctx, d.ID); err != nil {
			return err
		}

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
