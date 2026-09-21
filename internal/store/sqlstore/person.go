package sqlstore

import (
	"context"
	"database/sql"

	"github.com/amarin/genodex/internal/models"
)

// personTextRefLists — связные таблицы персоны со списками TextRef.
var personTextRefLists = []struct {
	table string
	get   func(*models.Person) []models.TextRef
	set   func(*models.Person, []models.TextRef)
}{
	{"person_estates", func(p *models.Person) []models.TextRef { return p.Estates },
		func(p *models.Person, v []models.TextRef) { p.Estates = v }},
	{"person_titles", func(p *models.Person) []models.TextRef { return p.Titles },
		func(p *models.Person, v []models.TextRef) { p.Titles = v }},
	{"person_nicknames", func(p *models.Person) []models.TextRef { return p.Nicknames },
		func(p *models.Person, v []models.TextRef) { p.Nicknames = v }},
	{"person_notes", func(p *models.Person) []models.TextRef { return p.Notes },
		func(p *models.Person, v []models.TextRef) { p.Notes = v }},
}

// SavePerson сохраняет персону: главная строка (upsert) + перезапись имён,
// списков TextRef, доказательств и поискового индекса.
func (s *Store) SavePerson(ctx context.Context, p *models.Person) (err error) {
	defer wrapSave(&err, "person", p.ID)

	return s.inTx(ctx, func(tx runner) error {
		if _, err := tx.Exec(
			`INSERT INTO persons(id, gender, private) VALUES (?, ?, ?)
			 ON CONFLICT(id) DO UPDATE SET gender = excluded.gender, private = excluded.private`,
			string(p.ID), string(p.Gender), boolInt(p.Private),
		); err != nil {
			return err
		}

		if err := clearChildren(tx, "person_names", "person_id", string(p.ID),
			valueRefs{table: "text_refs", cols: []string{"surname_id", "given_id", "patronymic_id"}},
			valueRefs{table: "dates", cols: []string{"since_id", "until_id"}},
		); err != nil {
			return err
		}

		for _, n := range p.Names {
			if err := insertPersonName(tx, p.ID, n); err != nil {
				return err
			}
		}

		for _, l := range personTextRefLists {
			if err := replaceTextRefList(tx, l.table, "person_id", string(p.ID), l.get(p)); err != nil {
				return err
			}
		}

		if err := replaceSourceLinks(tx, models.TypePerson, p.ID, p.Sources); err != nil {
			return err
		}

		return replaceSearchIndex(tx, "persons", p.ID, map[string][]string{"name": personTerms(p)})
	})
}

// insertPersonName пишет одно имя персоны: три TextRef (колонки NOT NULL,
// поэтому строка text_refs создаётся даже для пустого значения) и период.
func insertPersonName(tx runner, personID models.ID, n models.PersonName) error {
	surID, err := insertTextRefRow(tx, n.Surname)
	if err != nil {
		return err
	}

	givID, err := insertTextRefRow(tx, n.Given)
	if err != nil {
		return err
	}

	patID, err := insertTextRefRow(tx, n.Patronymic)
	if err != nil {
		return err
	}

	sinceID, err := insertDate(tx, n.Since)
	if err != nil {
		return err
	}

	untilID, err := insertDate(tx, n.Until)
	if err != nil {
		return err
	}

	_, err = tx.Exec(
		`INSERT INTO person_names(person_id, type, surname_id, given_id, patronymic_id,
			prefix, suffix, since_id, until_id)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		string(personID), string(n.Type), surID, givID, patID,
		n.Prefix, n.Suffix, nullInt64(sinceID), nullInt64(untilID))

	return err
}

// personTerms собирает поисковые термины персоны — все части всех имён.
func personTerms(p *models.Person) []string {
	terms := make([]string, 0, len(p.Names)*3)

	for _, n := range p.Names {
		terms = append(terms, n.Surname.Text, n.Given.Text, n.Patronymic.Text)
	}

	return terms
}

// personNameRow — сырая строка person_names до разрешения value-ссылок.
type personNameRow struct {
	typ, prefix, suffix string
	surID, givID, patID int64
	sinceID, untilID    sql.NullInt64
}

// GetPerson читает персону по id; не найдена — models.ErrNotFound.
func (s *Store) GetPerson(ctx context.Context, id models.ID) (*models.Person, error) {
	var (
		p      models.Person
		rawID  string
		gender string
	)

	err := s.run(ctx).QueryRow(`SELECT id, gender, private FROM persons WHERE id = ?`, string(id)).
		Scan(&rawID, &gender, &p.Private)
	if notFound(err) {
		return nil, models.ErrNotFound
	}

	if err != nil {
		return nil, err
	}

	p.ID, p.Gender = models.ID(rawID), models.PersonGender(gender)

	nameRows, err := scanRows(s.run(ctx), func(r *sql.Rows) (personNameRow, error) {
		var nr personNameRow
		err := r.Scan(&nr.typ, &nr.surID, &nr.givID, &nr.patID,
			&nr.prefix, &nr.suffix, &nr.sinceID, &nr.untilID)

		return nr, err
	},
		`SELECT type, surname_id, given_id, patronymic_id, prefix, suffix, since_id, until_id
		 FROM person_names WHERE person_id = ? ORDER BY id`, string(id))
	if err != nil {
		return nil, err
	}

	for _, nr := range nameRows {
		n, err := loadPersonName(s.run(ctx), nr)
		if err != nil {
			return nil, err
		}

		p.Names = append(p.Names, n)
	}

	for _, l := range personTextRefLists {
		items, err := loadTextRefList(s.run(ctx), l.table, "person_id", string(id))
		if err != nil {
			return nil, err
		}

		l.set(&p, items)
	}

	if p.Sources, err = loadSourceLinks(s.run(ctx), models.TypePerson, id); err != nil {
		return nil, err
	}

	return &p, nil
}

// loadPersonName разрешает value-ссылки сырой строки person_names.
func loadPersonName(q queryer, nr personNameRow) (models.PersonName, error) {
	n := models.PersonName{
		Type:   models.PersonNameType(nr.typ),
		Prefix: nr.prefix,
		Suffix: nr.suffix,
	}

	var err error
	if n.Surname, err = loadTextRef(q, nr.surID); err != nil {
		return n, err
	}

	if n.Given, err = loadTextRef(q, nr.givID); err != nil {
		return n, err
	}

	if n.Patronymic, err = loadTextRef(q, nr.patID); err != nil {
		return n, err
	}

	if n.Since, err = loadDate(q, int64From(nr.sinceID)); err != nil {
		return n, err
	}

	if n.Until, err = loadDate(q, int64From(nr.untilID)); err != nil {
		return n, err
	}

	return n, nil
}

// ListPeople возвращает всех персон в порядке вставки.
func (s *Store) ListPeople(ctx context.Context) ([]*models.Person, error) {
	return listEntities(ctx, s, "persons", s.GetPerson)
}
