package sqlstore

import (
	"context"
	"database/sql"

	"github.com/amarin/genodex/internal/models"
)

// Эталон для проверки пакетной загрузки: прежние поштучные загрузчики Person и
// AdministrativeDivision (по запросу на каждое значение и каждую связную
// таблицу), сохранённые как есть. Пакетный путь обязан давать те же значения.

// legacyGetPerson читает персону по id поштучно.
func legacyGetPerson(s *Store, ctx context.Context, id models.ID) (*models.Person, error) {
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
		n, err := legacyPersonName(s.run(ctx), nr)
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

// legacyPersonName разрешает value-ссылки сырой строки person_names.
func legacyPersonName(q queryer, nr personNameRow) (models.PersonName, error) {
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

// legacyLoadRenames читает переименования поштучно.
func legacyLoadRenames(q queryer, table, ownerCol, ownerID string) ([]models.NamedPeriod, error) {
	return scanRows(q, func(r *sql.Rows) (models.NamedPeriod, error) {
		var rn models.NamedPeriod
		err := r.Scan(&rn.Text, &rn.Since, &rn.Until)

		return rn, err
	}, `SELECT text, since, until FROM `+table+` WHERE `+ownerCol+` = ? ORDER BY position`, ownerID)
}

// legacyGetDivision читает единицу деления по id поштучно.
func legacyGetDivision(s *Store, ctx context.Context, id models.ID) (*models.AdministrativeDivision, error) {
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

	if a.Renames, err = legacyLoadRenames(s.run(ctx), "ad_renames", "ad_id", string(id)); err != nil {
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
