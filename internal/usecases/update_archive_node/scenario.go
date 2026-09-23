package update_archive_node

import (
	"context"
	"errors"
	"fmt"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// Scenario — сценарий «изменение узла архивного дерева».
type Scenario struct {
	store ArchiveNodeStore
}

// New создаёт сценарий.
func New(st ArchiveNodeStore) *Scenario {
	return &Scenario{store: st}
}

// UpdateArchiveNode полностью заменяет узел по n.ID: проверяет инварианты, в
// одной транзакции убеждается, что узел существует, архив существует,
// родитель (если задан) существует и принадлежит тому же архиву, а цепочка
// родителей не проходит через сам узел (цикл), и сохраняет.
//
// Ошибки: невалидная сущность — *models.ValidationError соответствующего
// поля; несуществующий архив/родитель, родитель из другого архива, цикл по
// parent_id — *models.ValidationError (archive_id/parent_id); нет такого
// узла — models.ErrNotFound; прочее — ошибки хранилища как есть.
func (s *Scenario) UpdateArchiveNode(ctx context.Context, n models.ArchiveNode) error {
	if err := n.Validate(); err != nil {
		return err
	}

	return s.store.InTx(ctx, func(tx store.Store) error {
		if _, err := tx.GetArchiveNode(ctx, n.ID); err != nil {
			return err
		}

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

		if err := checkParentChain(ctx, tx, &n); err != nil {
			return err
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
}

// checkParentChain обходит цепочку родителей n вверх: каждый предок должен
// существовать, и цепочка не должна проходить через сам узел или
// замыкаться (защита от испорченных данных). Ошибка цепочки —
// *models.ValidationError по полю parent_id. По образцу
// update_note.checkParentChain — существование непосредственного родителя и
// совпадение архива уже проверены выше, здесь только обход выше по цепочке.
func checkParentChain(ctx context.Context, tx store.Store, n *models.ArchiveNode) error {
	seen := map[models.ID]bool{n.ID: true}

	for cur := n.ParentID; cur != nil; {
		if seen[*cur] {
			return parentErr("цепочка родителей %q проходит через сам узел или замыкается на %q", n.ID, *cur)
		}
		seen[*cur] = true

		p, err := tx.GetArchiveNode(ctx, *cur)
		if err != nil {
			if errors.Is(err, models.ErrNotFound) {
				return parentErr("родитель %q не найден", *cur)
			}

			return err
		}

		cur = p.ParentID
	}

	return nil
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
