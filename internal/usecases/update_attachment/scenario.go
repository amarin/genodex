package update_attachment

import (
	"context"
	"errors"
	"fmt"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// Scenario — сценарий «изменение файлового вложения».
type Scenario struct {
	store AttachmentStore
}

// New создаёт сценарий.
func New(st AttachmentStore) *Scenario {
	return &Scenario{store: st}
}

// UpdateAttachment полностью заменяет вложение по a.ID: проверяет
// инварианты, в одной транзакции убеждается, что вложение существует, узел
// существует (документ — если задан), и сохраняет.
//
// Ошибки: невалидная сущность, несуществующий узел или документ —
// *models.ValidationError (соответствующее поле, node_id, document_id); нет
// такого вложения — models.ErrNotFound; прочее — ошибки хранилища как есть.
func (s *Scenario) UpdateAttachment(ctx context.Context, a models.Attachment) error {
	if err := a.Validate(); err != nil {
		return err
	}

	return s.store.InTx(ctx, func(tx store.Store) error {
		if _, err := tx.GetAttachment(ctx, a.ID); err != nil {
			return err
		}

		if _, err := tx.GetArchiveNode(ctx, a.NodeID); err != nil {
			if errors.Is(err, models.ErrNotFound) {
				return nodeErr("узел %q не найден", a.NodeID)
			}

			return err
		}

		if a.DocumentID != nil {
			if _, err := tx.GetArchiveDocument(ctx, *a.DocumentID); err != nil {
				if errors.Is(err, models.ErrNotFound) {
					return documentErr("документ %q не найден", *a.DocumentID)
				}

				return err
			}
		}

		return tx.SaveAttachment(ctx, &a)
	})
}

// nodeErr — *models.ValidationError по полю node_id.
func nodeErr(format string, args ...any) *models.ValidationError {
	return &models.ValidationError{
		Entity: models.TypeAttachment,
		Field:  "node_id",
		Reason: fmt.Sprintf(format, args...),
	}
}

// documentErr — *models.ValidationError по полю document_id.
func documentErr(format string, args ...any) *models.ValidationError {
	return &models.ValidationError{
		Entity: models.TypeAttachment,
		Field:  "document_id",
		Reason: fmt.Sprintf(format, args...),
	}
}
