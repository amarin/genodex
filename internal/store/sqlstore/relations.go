package sqlstore

import (
	"context"
	"database/sql"

	"github.com/amarin/genodex/internal/models"
)

// --- Relation -------------------------------------------------------------

// SaveRelation сохраняет ребро графа родства.
func (s *Store) SaveRelation(ctx context.Context, r *models.Relation) (err error) {
	defer wrapSave(&err, "relation", r.ID)

	return s.inTx(ctx, func(tx runner) error {
		old, err := collectMainRefs(tx, "relations", string(r.ID),
			[]string{"since_id", "until_id"}, nil, nil)
		if err != nil {
			return err
		}

		sinceID, err := insertDate(tx, r.Since)
		if err != nil {
			return err
		}

		untilID, err := insertDate(tx, r.Until)
		if err != nil {
			return err
		}

		if _, err := tx.Exec(
			`INSERT INTO relations(id, kind, rel_type, person_a, person_b, since_id, until_id, private)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			 ON CONFLICT(id) DO UPDATE SET kind = excluded.kind, rel_type = excluded.rel_type,
				person_a = excluded.person_a, person_b = excluded.person_b,
				since_id = excluded.since_id, until_id = excluded.until_id,
				private = excluded.private`,
			string(r.ID), string(r.Kind), string(r.RelType),
			string(r.PersonA), string(r.PersonB),
			nullInt64(sinceID), nullInt64(untilID), boolInt(r.Private),
		); err != nil {
			return err
		}

		if err := old.drop(tx); err != nil {
			return err
		}

		if err := replaceTextRefList(tx, "relation_notes", "relation_id", string(r.ID), r.Notes); err != nil {
			return err
		}

		if err := replaceSourceLinks(tx, models.TypeRelation, r.ID, r.Sources); err != nil {
			return err
		}

		// у связи нет собственных поисковых полей — индекс только чистится.
		return replaceSearchIndex(tx, "relations", r.ID, nil)
	})
}

// GetRelation читает связь по id; не найдена — models.ErrNotFound.
func (s *Store) GetRelation(ctx context.Context, id models.ID) (*models.Relation, error) {
	var (
		r                    models.Relation
		rawID, kind, relType string
		personA, personB     string
		sinceID, untilID     sql.NullInt64
	)

	err := s.run(ctx).QueryRow(
		`SELECT id, kind, rel_type, person_a, person_b, since_id, until_id, private
		 FROM relations WHERE id = ?`, string(id),
	).Scan(&rawID, &kind, &relType, &personA, &personB, &sinceID, &untilID, &r.Private)
	if notFound(err) {
		return nil, models.ErrNotFound
	}

	if err != nil {
		return nil, err
	}

	r.ID = models.ID(rawID)
	r.Kind, r.RelType = models.RelationKind(kind), models.RelationType(relType)
	r.PersonA, r.PersonB = models.ID(personA), models.ID(personB)

	if r.Since, err = loadDate(s.run(ctx), int64From(sinceID)); err != nil {
		return nil, err
	}

	if r.Until, err = loadDate(s.run(ctx), int64From(untilID)); err != nil {
		return nil, err
	}

	if r.Notes, err = loadTextRefList(s.run(ctx), "relation_notes", "relation_id", string(id)); err != nil {
		return nil, err
	}

	if r.Sources, err = loadSourceLinks(s.run(ctx), models.TypeRelation, id); err != nil {
		return nil, err
	}

	return &r, nil
}

// ListRelations возвращает все связи в порядке вставки.
func (s *Store) ListRelations(ctx context.Context, access models.Access, page models.Page) ([]*models.Relation, error) {
	return listEntities(ctx, s, "relations", access, page, s.GetRelation)
}

// --- Residence ------------------------------------------------------------

// SaveResidence сохраняет проживание персоны в месте.
func (s *Store) SaveResidence(ctx context.Context, r *models.Residence) (err error) {
	defer wrapSave(&err, "residence", r.ID)

	return s.inTx(ctx, func(tx runner) error {
		old, err := collectMainRefs(tx, "residences", string(r.ID),
			[]string{"since_id", "until_id"}, nil, nil)
		if err != nil {
			return err
		}

		sinceID, err := insertDate(tx, r.Since)
		if err != nil {
			return err
		}

		untilID, err := insertDate(tx, r.Until)
		if err != nil {
			return err
		}

		if _, err := tx.Exec(
			`INSERT INTO residences(id, person_id, place_id, since_id, until_id, note, private)
			 VALUES (?, ?, ?, ?, ?, ?, ?)
			 ON CONFLICT(id) DO UPDATE SET person_id = excluded.person_id,
				place_id = excluded.place_id, since_id = excluded.since_id,
				until_id = excluded.until_id, note = excluded.note, private = excluded.private`,
			string(r.ID), string(r.PersonID), string(r.PlaceID),
			nullInt64(sinceID), nullInt64(untilID), r.Note, boolInt(r.Private),
		); err != nil {
			return err
		}

		if err := old.drop(tx); err != nil {
			return err
		}

		if err := replaceSourceLinks(tx, models.TypeResidence, r.ID, r.Sources); err != nil {
			return err
		}

		return replaceSearchIndex(tx, "residences", r.ID, nil)
	})
}

// GetResidence читает проживание по id; не найдено — models.ErrNotFound.
func (s *Store) GetResidence(ctx context.Context, id models.ID) (*models.Residence, error) {
	var (
		r                        models.Residence
		rawID, personID, placeID string
		sinceID, untilID         sql.NullInt64
	)

	err := s.run(ctx).QueryRow(
		`SELECT id, person_id, place_id, since_id, until_id, note, private
		 FROM residences WHERE id = ?`, string(id),
	).Scan(&rawID, &personID, &placeID, &sinceID, &untilID, &r.Note, &r.Private)
	if notFound(err) {
		return nil, models.ErrNotFound
	}

	if err != nil {
		return nil, err
	}

	r.ID, r.PersonID, r.PlaceID = models.ID(rawID), models.ID(personID), models.ID(placeID)

	if r.Since, err = loadDate(s.run(ctx), int64From(sinceID)); err != nil {
		return nil, err
	}

	if r.Until, err = loadDate(s.run(ctx), int64From(untilID)); err != nil {
		return nil, err
	}

	if r.Sources, err = loadSourceLinks(s.run(ctx), models.TypeResidence, id); err != nil {
		return nil, err
	}

	return &r, nil
}

// ListResidences возвращает все проживания в порядке вставки.
func (s *Store) ListResidences(ctx context.Context, access models.Access, page models.Page) ([]*models.Residence, error) {
	return listEntities(ctx, s, "residences", access, page, s.GetResidence)
}

// --- Family ---------------------------------------------------------------

// SaveFamily сохраняет род/линию.
func (s *Store) SaveFamily(ctx context.Context, f *models.Family) (err error) {
	defer wrapSave(&err, "family", f.ID)

	return s.inTx(ctx, func(tx runner) error {
		if _, err := tx.Exec(
			`INSERT INTO families(id, name, private) VALUES (?, ?, ?)
			 ON CONFLICT(id) DO UPDATE SET name = excluded.name, private = excluded.private`,
			string(f.ID), f.Name, boolInt(f.Private),
		); err != nil {
			return err
		}

		if err := replaceTextRefList(tx, "family_members", "family_id", string(f.ID), f.Members); err != nil {
			return err
		}

		if err := replaceTextRefList(tx, "family_notes", "family_id", string(f.ID), f.Notes); err != nil {
			return err
		}

		if err := replaceSourceLinks(tx, models.TypeFamily, f.ID, f.Sources); err != nil {
			return err
		}

		return replaceSearchIndex(tx, "families", f.ID, map[string][]string{"name": {f.Name}})
	})
}

// GetFamily читает род по id; не найден — models.ErrNotFound.
func (s *Store) GetFamily(ctx context.Context, id models.ID) (*models.Family, error) {
	var (
		f     models.Family
		rawID string
	)

	err := s.run(ctx).QueryRow(`SELECT id, name, private FROM families WHERE id = ?`, string(id)).
		Scan(&rawID, &f.Name, &f.Private)
	if notFound(err) {
		return nil, models.ErrNotFound
	}

	if err != nil {
		return nil, err
	}

	f.ID = models.ID(rawID)

	if f.Members, err = loadTextRefList(s.run(ctx), "family_members", "family_id", string(id)); err != nil {
		return nil, err
	}

	if f.Notes, err = loadTextRefList(s.run(ctx), "family_notes", "family_id", string(id)); err != nil {
		return nil, err
	}

	if f.Sources, err = loadSourceLinks(s.run(ctx), models.TypeFamily, id); err != nil {
		return nil, err
	}

	return &f, nil
}

// ListFamilies возвращает все роды в порядке вставки.
func (s *Store) ListFamilies(ctx context.Context, access models.Access, page models.Page) ([]*models.Family, error) {
	return listEntities(ctx, s, "families", access, page, s.GetFamily)
}
