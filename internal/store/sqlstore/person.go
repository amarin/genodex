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
	people, err := s.getPeople(ctx, []models.ID{id})
	if err != nil {
		return nil, err
	}

	if len(people) == 0 {
		return nil, models.ErrNotFound
	}

	return people[0], nil
}

// getPeople читает персон пачкой — по одному запросу на таблицу, а не на
// персону. Результат — найденные персоны в порядке ids; отсутствующие
// пропускаются. Одиночный GetPerson идёт тем же путём, поэтому список и Get
// не могут разойтись.
func (s *Store) getPeople(ctx context.Context, ids []models.ID) ([]*models.Person, error) {
	q := s.run(ctx)
	owners := idStrings(ids)

	main, err := queryGrouped(q, owners, nil, func(in string) string {
		return `SELECT id, gender, private FROM persons WHERE id IN (` + in + `)`
	}, func(r *sql.Rows) (string, *models.Person, error) {
		var (
			p      models.Person
			id     string
			gender string
		)

		err := r.Scan(&id, &gender, &p.Private)
		p.ID, p.Gender = models.ID(id), models.PersonGender(gender)

		return id, &p, err
	})
	if err != nil {
		return nil, err
	}

	names, err := queryGrouped(q, owners, nil, func(in string) string {
		return `SELECT person_id, type, surname_id, given_id, patronymic_id, prefix, suffix, since_id, until_id
		        FROM person_names WHERE person_id IN (` + in + `) ORDER BY id`
	}, func(r *sql.Rows) (string, personNameRow, error) {
		var (
			owner string
			nr    personNameRow
		)

		err := r.Scan(&owner, &nr.typ, &nr.surID, &nr.givID, &nr.patID,
			&nr.prefix, &nr.suffix, &nr.sinceID, &nr.untilID)

		return owner, nr, err
	})
	if err != nil {
		return nil, err
	}

	var textIDs, dateIDs []int64

	for _, rows := range names {
		for _, nr := range rows {
			textIDs = append(textIDs, nr.surID, nr.givID, nr.patID)
			dateIDs = append(dateIDs, int64From(nr.sinceID), int64From(nr.untilID))
		}
	}

	texts, err := loadTextRefsByID(q, textIDs)
	if err != nil {
		return nil, err
	}

	dates, err := loadDatesByID(q, dateIDs)
	if err != nil {
		return nil, err
	}

	sources, err := loadSourceLinksBatch(q, models.TypePerson, owners)
	if err != nil {
		return nil, err
	}

	lists := make([]map[string][]models.TextRef, len(personTextRefLists))

	for i, l := range personTextRefLists {
		if lists[i], err = loadTextRefListsBatch(q, l.table, "person_id", owners); err != nil {
			return nil, err
		}
	}

	out := make([]*models.Person, 0, len(ids))

	for _, id := range ids {
		found := main[string(id)]
		if len(found) == 0 {
			continue
		}

		p := found[0]

		for _, nr := range names[string(id)] {
			p.Names = append(p.Names, models.PersonName{
				Type:       models.PersonNameType(nr.typ),
				Surname:    texts[nr.surID],
				Given:      texts[nr.givID],
				Patronymic: texts[nr.patID],
				Prefix:     nr.prefix,
				Suffix:     nr.suffix,
				Since:      dates[int64From(nr.sinceID)],
				Until:      dates[int64From(nr.untilID)],
			})
		}

		for i, l := range personTextRefLists {
			l.set(p, lists[i][string(id)])
		}

		p.Sources = sources[string(id)]
		out = append(out, p)
	}

	return out, nil
}

// ListPeople возвращает всех персон в порядке вставки.
func (s *Store) ListPeople(ctx context.Context, access models.Access, page models.Page) ([]*models.Person, error) {
	return listByIDs(ctx, s, "persons", access, page, s.getPeople)
}
