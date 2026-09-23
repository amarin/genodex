package create_archive_node

import (
	"context"
	"errors"
	"fmt"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// Scenario — сценарий «создание узла архивного дерева».
type Scenario struct {
	store ArchiveNodeStore
	ids   IDGenerator
}

// New создаёт сценарий.
func New(st ArchiveNodeStore, ids IDGenerator) *Scenario {
	return &Scenario{store: st, ids: ids}
}

// CreateArchiveNode создаёт узел: генерирует идентификатор, проверяет
// инварианты, в одной транзакции убеждается в существовании архива и (если
// задан) родителя, проверяет, что родитель принадлежит тому же архиву, и
// сохраняет. Возвращает созданный узел с заполненным ID.
//
// Ошибки: непустой входной ID, невалидная сущность, несуществующий архив
// или родитель, родитель из другого архива —
// *models.ValidationError (поля id, archive_id, parent_id); прочее —
// ошибки хранилища как есть.
func (s *Scenario) CreateArchiveNode(ctx context.Context, n models.ArchiveNode) (models.ArchiveNode, error) {
	if n.ID != "" {
		return models.ArchiveNode{}, &models.ValidationError{
			Entity: models.TypeArchiveNode,
			Field:  "id",
			Reason: fmt.Sprintf("идентификатор %q задан снаружи: при создании он генерируется", n.ID),
		}
	}

	n.ID = s.ids.New(models.TypeArchiveNode)

	if err := n.Validate(); err != nil {
		return models.ArchiveNode{}, err
	}

	err := s.store.InTx(ctx, func(tx store.Store) error {
		if _, err := tx.GetArchive(ctx, n.ArchiveID); err != nil {
			if errors.Is(err, models.ErrNotFound) {
				return archiveErr("архив %q не найден", n.ArchiveID)
			}

			return err
		}

		if n.ParentID != nil {
			parent, err := tx.GetArchiveNode(ctx, *n.ParentID)
			if err != nil {
				if errors.Is(err, models.ErrNotFound) {
					return parentErr("родитель %q не найден", *n.ParentID)
				}

				return err
			}

			if parent.ArchiveID != n.ArchiveID {
				return parentErr("родитель %q принадлежит другому архиву", *n.ParentID)
			}
		}

		for i, link := range n.Sources {
			if _, err := tx.GetCitation(ctx, link.CitationID); err != nil {
				if errors.Is(err, models.ErrNotFound) {
					return sourceLinkErr(i, "цитата %q не найдена", link.CitationID)
				}

				return err
			}
		}

		return tx.SaveArchiveNode(ctx, &n)
	})
	if err != nil {
		return models.ArchiveNode{}, err
	}

	return n, nil
}

// archiveErr — *models.ValidationError по полю archive_id.
func archiveErr(format string, args ...any) *models.ValidationError {
	return &models.ValidationError{
		Entity: models.TypeArchiveNode,
		Field:  "archive_id",
		Reason: fmt.Sprintf(format, args...),
	}
}

// parentErr — *models.ValidationError по полю parent_id.
func parentErr(format string, args ...any) *models.ValidationError {
	return &models.ValidationError{
		Entity: models.TypeArchiveNode,
		Field:  "parent_id",
		Reason: fmt.Sprintf(format, args...),
	}
}

// sourceLinkErr — *models.ValidationError по полю sources[i].citation_id.
func sourceLinkErr(i int, format string, args ...any) *models.ValidationError {
	return &models.ValidationError{
		Entity: models.TypeArchiveNode,
		Field:  fmt.Sprintf("sources[%d].citation_id", i),
		Reason: fmt.Sprintf(format, args...),
	}
}
