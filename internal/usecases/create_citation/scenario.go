package create_citation

import (
	"context"
	"errors"
	"fmt"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// Scenario — сценарий «создание цитаты».
type Scenario struct {
	store CitationStore
	ids   IDGenerator
}

// New создаёт сценарий.
func New(st CitationStore, ids IDGenerator) *Scenario {
	return &Scenario{store: st, ids: ids}
}

// CreateCitation создаёт цитату: генерирует идентификатор, проверяет
// инварианты, в одной транзакции убеждается в существовании источника
// (SourceID — обязательная строгая ссылка, в отличие от Archive.RepositoryID)
// и, если якорь задан и ссылается на другую сущность (ArchiveAnchor.NodeID/
// DocumentID, FileAnchor.AttachmentID), убеждается в её существовании тоже —
// по тому же принципу, что create_attachment проверяет NodeID/DocumentID
// через generic-хранилище (ArchiveNode/ArchiveDocument), независимо от
// собственного CRUD-слоя цели. URLAnchor ссылок не несёт.
//
// Ошибки: непустой входной ID, невалидная сущность и несуществующая
// ссылка — *models.ValidationError (поля id, source_id, anchor.node_id,
// anchor.document_id, anchor.attachment_id); прочее — ошибки хранилища как
// есть.
func (s *Scenario) CreateCitation(ctx context.Context, c models.Citation) (models.Citation, error) {
	if c.ID != "" {
		return models.Citation{}, &models.ValidationError{
			Entity: models.TypeCitation,
			Field:  "id",
			Reason: fmt.Sprintf("идентификатор %q задан снаружи: при создании он генерируется", c.ID),
		}
	}

	c.ID = s.ids.New(models.TypeCitation)

	if err := c.Validate(); err != nil {
		return models.Citation{}, err
	}

	err := s.store.InTx(ctx, func(tx store.Store) error {
		if _, err := tx.GetSource(ctx, c.SourceID); err != nil {
			if errors.Is(err, models.ErrNotFound) {
				return fieldErr("source_id", "источник %q не найден", c.SourceID)
			}

			return err
		}

		if err := checkAnchorRefs(ctx, tx, c.Anchor); err != nil {
			return err
		}

		return tx.SaveCitation(ctx, &c)
	})
	if err != nil {
		return models.Citation{}, err
	}

	return c, nil
}

// checkAnchorRefs проверяет существование ссылок внутри якоря (если он
// задан и несёт ссылку); ArchiveAnchor.DocumentID — только если задан.
func checkAnchorRefs(ctx context.Context, tx store.Store, a models.Anchor) error {
	switch v := a.(type) {
	case nil:
		return nil
	case *models.ArchiveAnchor:
		if v == nil {
			return nil
		}

		if _, err := tx.GetArchiveNode(ctx, v.NodeID); err != nil {
			if errors.Is(err, models.ErrNotFound) {
				return fieldErr("anchor.node_id", "архивный узел %q не найден", v.NodeID)
			}

			return err
		}

		if v.DocumentID != "" {
			if _, err := tx.GetArchiveDocument(ctx, v.DocumentID); err != nil {
				if errors.Is(err, models.ErrNotFound) {
					return fieldErr("anchor.document_id", "архивный документ %q не найден", v.DocumentID)
				}

				return err
			}
		}

		return nil
	case *models.FileAnchor:
		if v == nil {
			return nil
		}

		if _, err := tx.GetAttachment(ctx, v.AttachmentID); err != nil {
			if errors.Is(err, models.ErrNotFound) {
				return fieldErr("anchor.attachment_id", "вложение %q не найдено", v.AttachmentID)
			}

			return err
		}

		return nil
	default:
		return nil
	}
}

// fieldErr — *models.ValidationError по указанному полю.
func fieldErr(field, format string, args ...any) *models.ValidationError {
	return &models.ValidationError{
		Entity: models.TypeCitation,
		Field:  field,
		Reason: fmt.Sprintf(format, args...),
	}
}
