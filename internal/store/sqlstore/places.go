package sqlstore

import (
	"context"
	"database/sql"

	"github.com/amarin/genodex/internal/models"
)

// --- AdministrativeDivision -----------------------------------------------

// SaveAdministrativeDivision сохраняет единицу административного деления.
func (s *Store) SaveAdministrativeDivision(ctx context.Context, a *models.AdministrativeDivision) (err error) {
	defer wrapSave(&err, "administrative division", a.ID)

	return s.inTx(ctx, func(tx runner) error {
		old, err := collectMainRefs(tx, "administrative_divisions", string(a.ID),
			[]string{"since_id", "until_id"}, nil, nil)
		if err != nil {
			return err
		}

		sinceID, err := insertDate(tx, a.Since)
		if err != nil {
			return err
		}

		untilID, err := insertDate(tx, a.Until)
		if err != nil {
			return err
		}

		if _, err := tx.Exec(
			`INSERT INTO administrative_divisions(id, name, type, parent_id, since_id, until_id)
			 VALUES (?, ?, ?, ?, ?, ?)
			 ON CONFLICT(id) DO UPDATE SET name = excluded.name, type = excluded.type,
				parent_id = excluded.parent_id, since_id = excluded.since_id,
				until_id = excluded.until_id`,
			string(a.ID), a.Name, string(a.Type), nullIDPtr(a.ParentID),
			nullInt64(sinceID), nullInt64(untilID),
		); err != nil {
			return err
		}

		if err := old.drop(tx); err != nil {
			return err
		}

		for _, l := range []struct {
			table string
			items []models.TextRef
		}{
			{"ad_items", a.Items},
			{"ad_successors", a.Successors},
			{"ad_notes", a.Notes},
		} {
			if err := replaceTextRefList(tx, l.table, "ad_id", string(a.ID), l.items); err != nil {
				return err
			}
		}

		if err := replaceStringList(tx, "ad_variants", "ad_id", string(a.ID), a.Variants); err != nil {
			return err
		}

		if err := replaceRenames(tx, "ad_renames", "ad_id", string(a.ID), a.Renames); err != nil {
			return err
		}

		if err := replaceSourceLinks(tx, models.TypeAdministrativeDivision, a.ID, a.Sources); err != nil {
			return err
		}

		terms := append([]string{a.Name}, a.Variants...)

		return replaceSearchIndex(tx, "administrative_divisions", a.ID,
			map[string][]string{"name": terms})
	})
}

// GetAdministrativeDivision читает единицу деления по id; нет — models.ErrNotFound.
func (s *Store) GetAdministrativeDivision(ctx context.Context, id models.ID) (*models.AdministrativeDivision, error) {
	divisions, err := s.getDivisions(ctx, []models.ID{id})
	if err != nil {
		return nil, err
	}

	if len(divisions) == 0 {
		return nil, models.ErrNotFound
	}

	return divisions[0], nil
}

// divisionRow — главная строка единицы деления до разрешения value-ссылок.
type divisionRow struct {
	div              models.AdministrativeDivision
	sinceID, untilID sql.NullInt64
}

// getDivisions читает единицы деления пачкой (см. getPeople): по одному
// запросу на таблицу, результат — найденные в порядке ids.
func (s *Store) getDivisions(ctx context.Context, ids []models.ID) ([]*models.AdministrativeDivision, error) {
	q := s.run(ctx)
	owners := idStrings(ids)

	main, err := queryGrouped(q, owners, nil, func(in string) string {
		return `SELECT id, name, type, parent_id, since_id, until_id
		        FROM administrative_divisions WHERE id IN (` + in + `)`
	}, func(r *sql.Rows) (string, divisionRow, error) {
		var (
			row         divisionRow
			id, divType string
			parentID    sql.NullString
		)

		err := r.Scan(&id, &row.div.Name, &divType, &parentID, &row.sinceID, &row.untilID)
		row.div.ID, row.div.Type = models.ID(id), models.AdminDivisionType(divType)
		row.div.ParentID = idPtrFrom(parentID)

		return id, row, err
	})
	if err != nil {
		return nil, err
	}

	var dateIDs []int64

	for _, rows := range main {
		dateIDs = append(dateIDs, int64From(rows[0].sinceID), int64From(rows[0].untilID))
	}

	dates, err := loadDatesByID(q, dateIDs)
	if err != nil {
		return nil, err
	}

	textLists := []struct {
		table string
		set   func(*models.AdministrativeDivision, []models.TextRef)
	}{
		{"ad_items", func(a *models.AdministrativeDivision, v []models.TextRef) { a.Items = v }},
		{"ad_successors", func(a *models.AdministrativeDivision, v []models.TextRef) { a.Successors = v }},
		{"ad_notes", func(a *models.AdministrativeDivision, v []models.TextRef) { a.Notes = v }},
	}
	lists := make([]map[string][]models.TextRef, len(textLists))

	for i, l := range textLists {
		if lists[i], err = loadTextRefListsBatch(q, l.table, "ad_id", owners); err != nil {
			return nil, err
		}
	}

	variants, err := loadStringListsBatch(q, "ad_variants", "ad_id", owners)
	if err != nil {
		return nil, err
	}

	renames, err := loadRenamesBatch(q, "ad_renames", "ad_id", owners)
	if err != nil {
		return nil, err
	}

	sources, err := loadSourceLinksBatch(q, models.TypeAdministrativeDivision, owners)
	if err != nil {
		return nil, err
	}

	out := make([]*models.AdministrativeDivision, 0, len(ids))

	for _, id := range ids {
		found := main[string(id)]
		if len(found) == 0 {
			continue
		}

		row := found[0]
		a := row.div
		a.Since, a.Until = dates[int64From(row.sinceID)], dates[int64From(row.untilID)]

		for i, l := range textLists {
			l.set(&a, lists[i][string(id)])
		}

		a.Variants = variants[string(id)]
		a.Renames = renames[string(id)]
		a.Sources = sources[string(id)]
		out = append(out, &a)
	}

	return out, nil
}

// ListAdministrativeDivisions возвращает окно списка единиц деления.
func (s *Store) ListAdministrativeDivisions(ctx context.Context, access models.Access, page models.Page) ([]*models.AdministrativeDivision, error) {
	return listByIDs(ctx, s, "administrative_divisions", access, page, s.getDivisions)
}

// --- Church ---------------------------------------------------------------

// SaveChurch сохраняет церковь.
func (s *Store) SaveChurch(ctx context.Context, c *models.Church) (err error) {
	defer wrapSave(&err, "church", c.ID)

	return s.inTx(ctx, func(tx runner) error {
		old, err := collectMainRefs(tx, "churches", string(c.ID), nil, []string{"parish_id"}, nil)
		if err != nil {
			return err
		}

		parishID, err := insertTextRef(tx, c.Parish)
		if err != nil {
			return err
		}

		if _, err := tx.Exec(
			`INSERT INTO churches(id, name, parish_id) VALUES (?, ?, ?)
			 ON CONFLICT(id) DO UPDATE SET name = excluded.name, parish_id = excluded.parish_id`,
			string(c.ID), c.Name, nullInt64(parishID),
		); err != nil {
			return err
		}

		if err := old.drop(tx); err != nil {
			return err
		}

		if err := replaceTextRefList(tx, "church_settlements", "church_id", string(c.ID), c.Settlements); err != nil {
			return err
		}

		if err := replaceStringList(tx, "church_variants", "church_id", string(c.ID), c.Variants); err != nil {
			return err
		}

		if err := replaceTextRefList(tx, "church_notes", "church_id", string(c.ID), c.Notes); err != nil {
			return err
		}

		if err := replaceSourceLinks(tx, models.TypeChurch, c.ID, c.Sources); err != nil {
			return err
		}

		terms := append([]string{c.Name}, c.Variants...)

		return replaceSearchIndex(tx, "churches", c.ID, map[string][]string{"name": terms})
	})
}

// GetChurch читает церковь по id; не найдена — models.ErrNotFound.
func (s *Store) GetChurch(ctx context.Context, id models.ID) (*models.Church, error) {
	var (
		c        models.Church
		rawID    string
		parishID sql.NullInt64
	)

	err := s.run(ctx).QueryRow(`SELECT id, name, parish_id FROM churches WHERE id = ?`, string(id)).
		Scan(&rawID, &c.Name, &parishID)
	if notFound(err) {
		return nil, models.ErrNotFound
	}

	if err != nil {
		return nil, err
	}

	c.ID = models.ID(rawID)

	if c.Parish, err = loadTextRefPtr(s.run(ctx), int64From(parishID)); err != nil {
		return nil, err
	}

	if c.Settlements, err = loadTextRefList(s.run(ctx), "church_settlements", "church_id", string(id)); err != nil {
		return nil, err
	}

	if c.Variants, err = loadStringList(s.run(ctx), "church_variants", "church_id", string(id)); err != nil {
		return nil, err
	}

	if c.Notes, err = loadTextRefList(s.run(ctx), "church_notes", "church_id", string(id)); err != nil {
		return nil, err
	}

	if c.Sources, err = loadSourceLinks(s.run(ctx), models.TypeChurch, id); err != nil {
		return nil, err
	}

	return &c, nil
}

// ListChurches возвращает все церкви в порядке вставки.
func (s *Store) ListChurches(ctx context.Context, access models.Access, page models.Page) ([]*models.Church, error) {
	return listEntities(ctx, s, "churches", access, page, s.GetChurch)
}

// --- Parish ---------------------------------------------------------------

// SaveParish сохраняет приход.
func (s *Store) SaveParish(ctx context.Context, p *models.Parish) (err error) {
	defer wrapSave(&err, "parish", p.ID)

	return s.inTx(ctx, func(tx runner) error {
		old, err := collectMainRefs(tx, "parishes", string(p.ID),
			[]string{"since_id", "until_id"}, []string{"church_id"}, nil)
		if err != nil {
			return err
		}

		churchID, err := insertTextRef(tx, p.Church)
		if err != nil {
			return err
		}

		sinceID, err := insertDate(tx, p.Since)
		if err != nil {
			return err
		}

		untilID, err := insertDate(tx, p.Until)
		if err != nil {
			return err
		}

		if _, err := tx.Exec(
			`INSERT INTO parishes(id, name, church_id, since_id, until_id) VALUES (?, ?, ?, ?, ?)
			 ON CONFLICT(id) DO UPDATE SET name = excluded.name, church_id = excluded.church_id,
				since_id = excluded.since_id, until_id = excluded.until_id`,
			string(p.ID), p.Name, nullInt64(churchID), nullInt64(sinceID), nullInt64(untilID),
		); err != nil {
			return err
		}

		if err := old.drop(tx); err != nil {
			return err
		}

		if err := replaceTextRefList(tx, "parish_settlements", "parish_id", string(p.ID), p.Settlements); err != nil {
			return err
		}

		if err := replaceTextRefList(tx, "parish_notes", "parish_id", string(p.ID), p.Notes); err != nil {
			return err
		}

		if err := replaceSourceLinks(tx, models.TypeParish, p.ID, p.Sources); err != nil {
			return err
		}

		return replaceSearchIndex(tx, "parishes", p.ID, map[string][]string{"name": {p.Name}})
	})
}

// GetParish читает приход по id; не найден — models.ErrNotFound.
func (s *Store) GetParish(ctx context.Context, id models.ID) (*models.Parish, error) {
	var (
		p                models.Parish
		rawID            string
		churchID         sql.NullInt64
		sinceID, untilID sql.NullInt64
	)

	err := s.run(ctx).QueryRow(
		`SELECT id, name, church_id, since_id, until_id FROM parishes WHERE id = ?`, string(id),
	).Scan(&rawID, &p.Name, &churchID, &sinceID, &untilID)
	if notFound(err) {
		return nil, models.ErrNotFound
	}

	if err != nil {
		return nil, err
	}

	p.ID = models.ID(rawID)

	if p.Church, err = loadTextRefPtr(s.run(ctx), int64From(churchID)); err != nil {
		return nil, err
	}

	if p.Settlements, err = loadTextRefList(s.run(ctx), "parish_settlements", "parish_id", string(id)); err != nil {
		return nil, err
	}

	if p.Since, err = loadDate(s.run(ctx), int64From(sinceID)); err != nil {
		return nil, err
	}

	if p.Until, err = loadDate(s.run(ctx), int64From(untilID)); err != nil {
		return nil, err
	}

	if p.Notes, err = loadTextRefList(s.run(ctx), "parish_notes", "parish_id", string(id)); err != nil {
		return nil, err
	}

	if p.Sources, err = loadSourceLinks(s.run(ctx), models.TypeParish, id); err != nil {
		return nil, err
	}

	return &p, nil
}

// ListParishes возвращает все приходы в порядке вставки.
func (s *Store) ListParishes(ctx context.Context, access models.Access, page models.Page) ([]*models.Parish, error) {
	return listEntities(ctx, s, "parishes", access, page, s.GetParish)
}
