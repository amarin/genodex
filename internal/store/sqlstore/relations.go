package sqlstore

import (
	"database/sql"

	"github.com/amarin/genodex/internal/models"
)

// --- Relation -------------------------------------------------------------

// SaveRelation сохраняет ребро графа родства.
func (s *Store) SaveRelation(r *models.Relation) (err error) {
	defer wrapSave(&err, "relation", r.ID)

	return s.db.Tx(func(tx *sql.Tx) error {
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

// GetRelation читает связь по id; не найдена — (nil, nil).
func (s *Store) GetRelation(id models.ID) (*models.Relation, error) {
	var (
		r                    models.Relation
		rawID, kind, relType string
		personA, personB     string
		sinceID, untilID     sql.NullInt64
	)

	err := s.db.QueryRow(
		`SELECT id, kind, rel_type, person_a, person_b, since_id, until_id, private
		 FROM relations WHERE id = ?`, string(id),
	).Scan(&rawID, &kind, &relType, &personA, &personB, &sinceID, &untilID, &r.Private)
	if notFound(err) {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	r.ID = models.ID(rawID)
	r.Kind, r.RelType = models.RelationKind(kind), models.RelationType(relType)
	r.PersonA, r.PersonB = models.ID(personA), models.ID(personB)

	if r.Since, err = loadDate(s.db, int64From(sinceID)); err != nil {
		return nil, err
	}

	if r.Until, err = loadDate(s.db, int64From(untilID)); err != nil {
		return nil, err
	}

	if r.Notes, err = loadTextRefList(s.db, "relation_notes", "relation_id", string(id)); err != nil {
		return nil, err
	}

	if r.Sources, err = loadSourceLinks(s.db, models.TypeRelation, id); err != nil {
		return nil, err
	}

	return &r, nil
}

// ListRelations возвращает все связи в порядке вставки.
func (s *Store) ListRelations() ([]*models.Relation, error) {
	return listEntities(s, "relations", s.GetRelation)
}

// --- Residence ------------------------------------------------------------

// SaveResidence сохраняет проживание персоны в месте.
func (s *Store) SaveResidence(r *models.Residence) (err error) {
	defer wrapSave(&err, "residence", r.ID)

	return s.db.Tx(func(tx *sql.Tx) error {
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

// GetResidence читает проживание по id; не найдено — (nil, nil).
func (s *Store) GetResidence(id models.ID) (*models.Residence, error) {
	var (
		r                        models.Residence
		rawID, personID, placeID string
		sinceID, untilID         sql.NullInt64
	)

	err := s.db.QueryRow(
		`SELECT id, person_id, place_id, since_id, until_id, note, private
		 FROM residences WHERE id = ?`, string(id),
	).Scan(&rawID, &personID, &placeID, &sinceID, &untilID, &r.Note, &r.Private)
	if notFound(err) {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	r.ID, r.PersonID, r.PlaceID = models.ID(rawID), models.ID(personID), models.ID(placeID)

	if r.Since, err = loadDate(s.db, int64From(sinceID)); err != nil {
		return nil, err
	}

	if r.Until, err = loadDate(s.db, int64From(untilID)); err != nil {
		return nil, err
	}

	if r.Sources, err = loadSourceLinks(s.db, models.TypeResidence, id); err != nil {
		return nil, err
	}

	return &r, nil
}

// ListResidences возвращает все проживания в порядке вставки.
func (s *Store) ListResidences() ([]*models.Residence, error) {
	return listEntities(s, "residences", s.GetResidence)
}

// --- Family ---------------------------------------------------------------

// SaveFamily сохраняет род/линию.
func (s *Store) SaveFamily(f *models.Family) (err error) {
	defer wrapSave(&err, "family", f.ID)

	return s.db.Tx(func(tx *sql.Tx) error {
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

// GetFamily читает род по id; не найден — (nil, nil).
func (s *Store) GetFamily(id models.ID) (*models.Family, error) {
	var (
		f     models.Family
		rawID string
	)

	err := s.db.QueryRow(`SELECT id, name, private FROM families WHERE id = ?`, string(id)).
		Scan(&rawID, &f.Name, &f.Private)
	if notFound(err) {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	f.ID = models.ID(rawID)

	if f.Members, err = loadTextRefList(s.db, "family_members", "family_id", string(id)); err != nil {
		return nil, err
	}

	if f.Notes, err = loadTextRefList(s.db, "family_notes", "family_id", string(id)); err != nil {
		return nil, err
	}

	if f.Sources, err = loadSourceLinks(s.db, models.TypeFamily, id); err != nil {
		return nil, err
	}

	return &f, nil
}

// ListFamilies возвращает все роды в порядке вставки.
func (s *Store) ListFamilies() ([]*models.Family, error) {
	return listEntities(s, "families", s.GetFamily)
}
