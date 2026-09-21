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
	var (
		a                models.AdministrativeDivision
		rawID, divType   string
		parentID         sql.NullString
		sinceID, untilID sql.NullInt64
	)

	err := s.run(ctx).QueryRow(
		`SELECT id, name, type, parent_id, since_id, until_id
		 FROM administrative_divisions WHERE id = ?`, string(id),
	).Scan(&rawID, &a.Name, &divType, &parentID, &sinceID, &untilID)
	if notFound(err) {
		return nil, models.ErrNotFound
	}

	if err != nil {
		return nil, err
	}

	a.ID, a.Type = models.ID(rawID), models.AdminDivisionType(divType)
	a.ParentID = idPtrFrom(parentID)

	if a.Since, err = loadDate(s.run(ctx), int64From(sinceID)); err != nil {
		return nil, err
	}

	if a.Until, err = loadDate(s.run(ctx), int64From(untilID)); err != nil {
		return nil, err
	}

	if a.Items, err = loadTextRefList(s.run(ctx), "ad_items", "ad_id", string(id)); err != nil {
		return nil, err
	}

	if a.Variants, err = loadStringList(s.run(ctx), "ad_variants", "ad_id", string(id)); err != nil {
		return nil, err
	}

	if a.Renames, err = loadRenames(s.run(ctx), "ad_renames", "ad_id", string(id)); err != nil {
		return nil, err
	}

	if a.Successors, err = loadTextRefList(s.run(ctx), "ad_successors", "ad_id", string(id)); err != nil {
		return nil, err
	}

	if a.Notes, err = loadTextRefList(s.run(ctx), "ad_notes", "ad_id", string(id)); err != nil {
		return nil, err
	}

	if a.Sources, err = loadSourceLinks(s.run(ctx), models.TypeAdministrativeDivision, id); err != nil {
		return nil, err
	}

	return &a, nil
}

// ListAdministrativeDivisions возвращает все единицы деления.
func (s *Store) ListAdministrativeDivisions(ctx context.Context) ([]*models.AdministrativeDivision, error) {
	return listEntities(ctx, s, "administrative_divisions", s.GetAdministrativeDivision)
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
func (s *Store) ListChurches(ctx context.Context) ([]*models.Church, error) {
	return listEntities(ctx, s, "churches", s.GetChurch)
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
func (s *Store) ListParishes(ctx context.Context) ([]*models.Parish, error) {
	return listEntities(ctx, s, "parishes", s.GetParish)
}
