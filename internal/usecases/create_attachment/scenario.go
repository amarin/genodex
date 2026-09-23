package create_attachment

import (
	"context"
	"errors"
	"fmt"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// Scenario — сценарий «создание файлового вложения».
type Scenario struct {
	store AttachmentStore
	ids   IDGenerator
}

// New создаёт сценарий.
func New(st AttachmentStore, ids IDGenerator) *Scenario {
	return &Scenario{store: st, ids: ids}
}

// CreateAttachment создаёт вложение: генерирует идентификатор, проверяет
// инварианты, в одной транзакции убеждается в существовании узла (всегда —
// NodeID обязателен) и документа (если задан) и сохраняет. Возвращает
// созданное вложение с заполненным ID. NodeID/DocumentID ссылаются на
// ArchiveNode/ArchiveDocument через generic-хранилище — проверка существования
// не зависит от того, есть ли у цели собственный orchestration-слой.
//
// Ошибки: непустой входной ID (явный идентификатор допустим только для
// импорта), невалидная сущность, несуществующий узел или документ —
// *models.ValidationError (поля id, node_id, document_id); прочее — ошибки
// хранилища как есть.
func (s *Scenario) CreateAttachment(ctx context.Context, a models.Attachment) (models.Attachment, error) {
	if a.ID != "" {
		return models.Attachment{}, &models.ValidationError{
			Entity: models.TypeAttachment,
			Field:  "id",
			Reason: fmt.Sprintf("идентификатор %q задан снаружи: при создании он генерируется", a.ID),
		}
	}

	a.ID = s.ids.New(models.TypeAttachment)

	if err := a.Validate(); err != nil {
		return models.Attachment{}, err
	}

	err := s.store.InTx(ctx, func(tx store.Store) error {
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
	if err != nil {
		return models.Attachment{}, err
	}

	return a, nil
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
