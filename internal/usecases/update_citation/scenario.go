package update_citation

import (
	"context"
	"errors"
	"fmt"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// Scenario — сценарий «изменение цитаты».
type Scenario struct {
	store CitationStore
}

// New создаёт сценарий.
func New(st CitationStore) *Scenario {
	return &Scenario{store: st}
}

// UpdateCitation полностью заменяет цитату по c.ID: проверяет инварианты, в
// одной транзакции убеждается, что цитата существует, источник существует,
// и (если якорь задан и несёт ссылку) ссылка внутри якоря существует, и
// сохраняет. По образцу create_citation.
//
// Ошибки: невалидная сущность и несуществующая ссылка —
// *models.ValidationError (source_id, anchor.node_id, anchor.document_id,
// anchor.attachment_id); нет такой цитаты — models.ErrNotFound; прочее —
// ошибки хранилища как есть.
func (s *Scenario) UpdateCitation(ctx context.Context, c models.Citation) error {
	if err := c.Validate(); err != nil {
		return err
	}

	return s.store.InTx(ctx, func(tx store.Store) error {
		if _, err := tx.GetCitation(ctx, c.ID); err != nil {
			return err
		}

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
}

// checkAnchorRefs проверяет существование ссылок внутри якоря (если он
// задан и несёт ссылку); ArchiveAnchor.DocumentID — только если задан. По
// образцу create_citation.checkAnchorRefs.
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
