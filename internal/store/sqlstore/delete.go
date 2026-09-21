package sqlstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/amarin/genodex/internal/models"
)

// deleteEntity удаляет сущность вместе с её связными строками, записями
// search_index и source_links и осиротевшими value-строками.
//
//   - Нет такой сущности — models.ErrNotFound.
//   - На сущность ссылаются другие (строгие FK) — *models.InUseError со списком
//     ссылающихся; ничего не удаляется. FK RESTRICT остаётся страховкой.
//   - Ссылки ON DELETE SET NULL (например, attachments.document_id) обнуляются.
//
// Устройство схемы (связные таблицы, value-колонки, ссылающиеся строки) берётся
// из графа внешних ключей, см. fkgraph.go.
func (s *Store) deleteEntity(ctx context.Context, typ models.Type, id models.ID) (err error) {
	table, ok := entityTables[typ]
	if !ok {
		return fmt.Errorf("delete: неизвестный тип %q", typ)
	}

	defer func() {
		// ErrNotFound и *InUseError несут смысл сами, остальное — сбой хранилища.
		var inUse *models.InUseError
		if err == nil || errors.Is(err, models.ErrNotFound) || errors.As(err, &inUse) {
			return
		}

		err = fmt.Errorf("delete %s %q: %w", typ, string(id), err)
	}()

	g, err := s.graph(ctx)
	if err != nil {
		return err
	}

	return s.inTx(ctx, func(tx runner) error {
		var exists int
		if err := tx.QueryRow(`SELECT 1 FROM `+table+` WHERE id = ?`, string(id)).Scan(&exists); err != nil {
			if notFound(err) {
				return models.ErrNotFound
			}

			return err
		}

		refs, err := referrers(tx, g, typ, id, models.MaxReferrers)
		if err != nil {
			return err
		}

		if len(refs) > 0 {
			return &models.InUseError{Type: typ, ID: id, Referrers: refs}
		}

		values, err := collectOwnedValues(tx, g, table, string(id))
		if err != nil {
			return err
		}

		if _, err := tx.Exec(
			`DELETE FROM source_links WHERE target_type = ? AND target_id = ?`, string(typ), string(id),
		); err != nil {
			return err
		}

		if _, err := tx.Exec(
			`DELETE FROM search_index WHERE entity_table = ? AND entity_id = ?`, table, string(id),
		); err != nil {
			return err
		}

		// каскад сносит связные строки; value-строки на них ссылались, поэтому
		// удаляются только после главной строки.
		if _, err := tx.Exec(`DELETE FROM `+table+` WHERE id = ?`, string(id)); err != nil {
			return err
		}

		for _, v := range valueTables {
			if err := deleteValues(tx, v, values[v]); err != nil {
				return err
			}
		}

		return nil
	})
}

// collectOwnedValues собирает id value-строк (text_refs/dates/anchors), которыми
// владеют главная строка сущности и её связные строки.
func collectOwnedValues(tx runner, g *schemaGraph, table, id string) (map[string][]int64, error) {
	out := map[string][]int64{}

	collect := func(from, ownerCol string) error {
		for v, cols := range g.valueCols(from) {
			ids, err := collectIDs(tx,
				`SELECT `+strings.Join(cols, ", ")+` FROM `+from+` WHERE `+ownerCol+` = ?`, id)
			if err != nil {
				return err
			}

			out[v] = append(out[v], ids...)
		}

		return nil
	}

	if err := collect(table, "id"); err != nil {
		return nil, err
	}

	for _, e := range g.cascadeChildren(table) {
		if err := collect(e.child, e.col); err != nil {
			return nil, err
		}
	}

	return out, nil
}

// referrers возвращает до limit сущностей, строго ссылающихся на (typ, id): без
// повторов, в стабильном порядке. Ссылка из самой сущности на себя не считается.
// Запрос каждой ссылки отдаёт пары (тип, id) ссылающейся сущности.
func referrers(tx runner, g *schemaGraph, typ models.Type, id models.ID, limit int) ([]models.EntityRef, error) {
	var (
		out  []models.EntityRef
		seen = map[models.EntityRef]bool{}
	)

	for _, e := range g.restrictInto(entityTables[typ]) {
		if len(out) >= limit {
			break
		}

		var (
			query string
			args  []any
		)

		switch owner, isChild := g.owner(e.child); {
		case e.child == "source_links":
			// полиморфная связь «утверждение → цитата»: ссылающийся — цель ссылки
			query = `SELECT DISTINCT target_type, target_id FROM source_links WHERE ` + e.col + ` = ?
			         ORDER BY target_type, target_id LIMIT ?`
			args = []any{string(id), limit}
		case isChild:
			// связная таблица: ссылающийся — её владелец
			query = `SELECT DISTINCT ?, ` + owner.col + ` FROM ` + e.child +
				` WHERE ` + e.col + ` = ? ORDER BY ` + owner.col + ` LIMIT ?`
			args = []any{string(typeOfTable[owner.parent]), string(id), limit}
		default:
			// главная таблица другой сущности (или этой же — тогда без самой себя)
			query = `SELECT ?, id FROM ` + e.child + ` WHERE ` + e.col + ` = ? AND id <> ? ORDER BY id LIMIT ?`
			args = []any{string(typeOfTable[e.child]), string(id), string(id), limit}
		}

		pairs, err := scanRows(tx, func(r *sql.Rows) (models.EntityRef, error) {
			var t, i string
			err := r.Scan(&t, &i)

			return models.EntityRef{Type: models.Type(t), ID: models.ID(i)}, err
		}, query, args...)
		if err != nil {
			return nil, err
		}

		for _, ref := range pairs {
			if !seen[ref] && len(out) < limit {
				seen[ref] = true
				out = append(out, ref)
			}
		}
	}

	return out, nil
}

// --- порт: Delete* ------------------------------------------------------

// DeletePerson удаляет сущность; см. deleteEntity.
func (s *Store) DeletePerson(ctx context.Context, id models.ID) error {
	return s.deleteEntity(ctx, models.TypePerson, id)
}

// DeleteRelation удаляет сущность; см. deleteEntity.
func (s *Store) DeleteRelation(ctx context.Context, id models.ID) error {
	return s.deleteEntity(ctx, models.TypeRelation, id)
}

// DeleteResidence удаляет сущность; см. deleteEntity.
func (s *Store) DeleteResidence(ctx context.Context, id models.ID) error {
	return s.deleteEntity(ctx, models.TypeResidence, id)
}

// DeleteFamily удаляет сущность; см. deleteEntity.
func (s *Store) DeleteFamily(ctx context.Context, id models.ID) error {
	return s.deleteEntity(ctx, models.TypeFamily, id)
}

// DeleteSurname удаляет сущность; см. deleteEntity.
func (s *Store) DeleteSurname(ctx context.Context, id models.ID) error {
	return s.deleteEntity(ctx, models.TypeSurname, id)
}

// DeleteGivenName удаляет сущность; см. deleteEntity.
func (s *Store) DeleteGivenName(ctx context.Context, id models.ID) error {
	return s.deleteEntity(ctx, models.TypeGivenName, id)
}

// DeletePatronymic удаляет сущность; см. deleteEntity.
func (s *Store) DeletePatronymic(ctx context.Context, id models.ID) error {
	return s.deleteEntity(ctx, models.TypePatronymic, id)
}

// DeleteEstate удаляет сущность; см. deleteEntity.
func (s *Store) DeleteEstate(ctx context.Context, id models.ID) error {
	return s.deleteEntity(ctx, models.TypeEstate, id)
}

// DeleteTitle удаляет сущность; см. deleteEntity.
func (s *Store) DeleteTitle(ctx context.Context, id models.ID) error {
	return s.deleteEntity(ctx, models.TypeTitle, id)
}

// DeleteAdministrativeDivision удаляет сущность; см. deleteEntity.
func (s *Store) DeleteAdministrativeDivision(ctx context.Context, id models.ID) error {
	return s.deleteEntity(ctx, models.TypeAdministrativeDivision, id)
}

// DeleteChurch удаляет сущность; см. deleteEntity.
func (s *Store) DeleteChurch(ctx context.Context, id models.ID) error {
	return s.deleteEntity(ctx, models.TypeChurch, id)
}

// DeleteParish удаляет сущность; см. deleteEntity.
func (s *Store) DeleteParish(ctx context.Context, id models.ID) error {
	return s.deleteEntity(ctx, models.TypeParish, id)
}

// DeleteEvent удаляет сущность; см. deleteEntity.
func (s *Store) DeleteEvent(ctx context.Context, id models.ID) error {
	return s.deleteEntity(ctx, models.TypeEvent, id)
}

// DeleteSource удаляет сущность; см. deleteEntity.
func (s *Store) DeleteSource(ctx context.Context, id models.ID) error {
	return s.deleteEntity(ctx, models.TypeSource, id)
}

// DeleteArchive удаляет сущность; см. deleteEntity.
func (s *Store) DeleteArchive(ctx context.Context, id models.ID) error {
	return s.deleteEntity(ctx, models.TypeArchive, id)
}

// DeleteArchiveNode удаляет сущность; см. deleteEntity.
func (s *Store) DeleteArchiveNode(ctx context.Context, id models.ID) error {
	return s.deleteEntity(ctx, models.TypeArchiveNode, id)
}

// DeleteArchiveDocument удаляет сущность; см. deleteEntity.
func (s *Store) DeleteArchiveDocument(ctx context.Context, id models.ID) error {
	return s.deleteEntity(ctx, models.TypeArchiveDocument, id)
}

// DeleteAttachment удаляет сущность; см. deleteEntity.
func (s *Store) DeleteAttachment(ctx context.Context, id models.ID) error {
	return s.deleteEntity(ctx, models.TypeAttachment, id)
}

// DeleteCitation удаляет сущность; см. deleteEntity.
func (s *Store) DeleteCitation(ctx context.Context, id models.ID) error {
	return s.deleteEntity(ctx, models.TypeCitation, id)
}

// DeleteNote удаляет сущность; см. deleteEntity.
func (s *Store) DeleteNote(ctx context.Context, id models.ID) error {
	return s.deleteEntity(ctx, models.TypeNote, id)
}

// DeleteRepository удаляет сущность; см. deleteEntity.
func (s *Store) DeleteRepository(ctx context.Context, id models.ID) error {
	return s.deleteEntity(ctx, models.TypeRepository, id)
}
