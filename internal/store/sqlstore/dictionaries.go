package sqlstore

import (
	"database/sql"
	"strings"

	"github.com/amarin/genodex/internal/models"
)

// Словарные записи (Surname, GivenName, Patronymic, Estate, Title) устроены
// одинаково: главная строка с canonical (у given_names ещё gender) и три
// связных списка TextRef — variants/items/notes. Поля Sources у них в
// internal/models нет, поэтому source_links для них не пишутся.

// dictSpec описывает словарную таблицу и её связные списки.
type dictSpec struct {
	table    string   // surnames
	ownerCol string   // surname_id
	cols     []string // скалярные колонки помимо id
}

// prefix возвращает префикс связных таблиц (surname_id → surname).
func (d dictSpec) prefix() string { return strings.TrimSuffix(d.ownerCol, "_id") }

// lists возвращает связные таблицы в порядке variants/items/notes.
func (d dictSpec) lists() [3]string {
	p := d.prefix()

	return [3]string{p + "_variants", p + "_items", p + "_notes"}
}

var (
	surnameSpec    = dictSpec{table: "surnames", ownerCol: "surname_id", cols: []string{"canonical"}}
	givenNameSpec  = dictSpec{table: "given_names", ownerCol: "given_name_id", cols: []string{"canonical", "gender"}}
	patronymicSpec = dictSpec{table: "patronymics", ownerCol: "patronymic_id", cols: []string{"canonical"}}
	estateSpec     = dictSpec{table: "estates", ownerCol: "estate_id", cols: []string{"canonical"}}
	titleSpec      = dictSpec{table: "titles", ownerCol: "title_id", cols: []string{"canonical"}}
)

// dictValue — значения словарной записи для обобщённого Save/Get.
type dictValue struct {
	id                     models.ID
	vals                   []any // значения spec.cols
	variants, items, notes []models.TextRef
}

// saveDictionary пишет словарную запись: upsert главной строки, перезапись
// трёх списков и поискового индекса (canonical + тексты вариантов).
func (s *Store) saveDictionary(spec dictSpec, v dictValue) error {
	cols := append([]string{"id"}, spec.cols...)
	set := make([]string, 0, len(spec.cols))

	for _, c := range spec.cols {
		set = append(set, c+" = excluded."+c)
	}

	placeholders := strings.TrimSuffix(strings.Repeat("?, ", len(cols)), ", ")
	args := append([]any{string(v.id)}, v.vals...)
	lists := spec.lists()

	terms := make([]string, 0, len(v.variants)+1)
	if len(v.vals) > 0 {
		if canonical, ok := v.vals[0].(string); ok {
			terms = append(terms, canonical)
		}
	}

	for _, variant := range v.variants {
		terms = append(terms, variant.Text)
	}

	return s.db.Tx(func(tx *sql.Tx) error {
		if _, err := tx.Exec(
			`INSERT INTO `+spec.table+`(`+strings.Join(cols, ", ")+`) VALUES (`+placeholders+`)
			 ON CONFLICT(id) DO UPDATE SET `+strings.Join(set, ", "), args...,
		); err != nil {
			return err
		}

		for i, items := range [3][]models.TextRef{v.variants, v.items, v.notes} {
			if err := replaceTextRefList(tx, lists[i], spec.ownerCol, string(v.id), items); err != nil {
				return err
			}
		}

		return replaceSearchIndex(tx, spec.table, v.id, map[string][]string{"name": terms})
	})
}

// getDictionary читает скалярные колонки словарной записи и три списка;
// found=false — записи нет.
func (s *Store) getDictionary(spec dictSpec, id models.ID, dest ...*string) (dictValue, bool, error) {
	var v dictValue

	ptrs := make([]any, 0, len(dest))
	for _, d := range dest {
		ptrs = append(ptrs, d)
	}

	err := s.db.QueryRow(
		`SELECT `+strings.Join(spec.cols, ", ")+` FROM `+spec.table+` WHERE id = ?`, string(id),
	).Scan(ptrs...)
	if notFound(err) {
		return v, false, nil
	}

	if err != nil {
		return v, false, err
	}

	v.id = id
	lists := spec.lists()

	out := [3][]models.TextRef{}

	for i, table := range lists {
		items, err := loadTextRefList(s.db, table, spec.ownerCol, string(id))
		if err != nil {
			return v, false, err
		}

		out[i] = items
	}

	v.variants, v.items, v.notes = out[0], out[1], out[2]

	return v, true, nil
}

// --- Surname --------------------------------------------------------------

// SaveSurname сохраняет словарную запись фамилии.
func (s *Store) SaveSurname(v *models.Surname) error {
	return s.saveDictionary(surnameSpec, dictValue{
		id: v.ID, vals: []any{v.Canonical},
		variants: v.Variants, items: v.Items, notes: v.Notes,
	})
}

// GetSurname читает фамилию по id; не найдена — (nil, nil).
func (s *Store) GetSurname(id models.ID) (*models.Surname, error) {
	var canonical string

	v, ok, err := s.getDictionary(surnameSpec, id, &canonical)
	if err != nil || !ok {
		return nil, err
	}

	return &models.Surname{
		ID: id, Canonical: canonical,
		Variants: v.variants, Items: v.items, Notes: v.notes,
	}, nil
}

// ListSurnames возвращает все фамилии в порядке вставки.
func (s *Store) ListSurnames() ([]*models.Surname, error) {
	return listEntities(s, "surnames", s.GetSurname)
}

// --- GivenName ------------------------------------------------------------

// SaveGivenName сохраняет словарную запись имени.
func (s *Store) SaveGivenName(v *models.GivenName) error {
	return s.saveDictionary(givenNameSpec, dictValue{
		id: v.ID, vals: []any{v.Canonical, string(v.Gender)},
		variants: v.Variants, items: v.Items, notes: v.Notes,
	})
}

// GetGivenName читает имя по id; не найдено — (nil, nil).
func (s *Store) GetGivenName(id models.ID) (*models.GivenName, error) {
	var canonical, gender string

	v, ok, err := s.getDictionary(givenNameSpec, id, &canonical, &gender)
	if err != nil || !ok {
		return nil, err
	}

	return &models.GivenName{
		ID: id, Canonical: canonical, Gender: models.NameGender(gender),
		Variants: v.variants, Items: v.items, Notes: v.notes,
	}, nil
}

// ListGivenNames возвращает все имена в порядке вставки.
func (s *Store) ListGivenNames() ([]*models.GivenName, error) {
	return listEntities(s, "given_names", s.GetGivenName)
}

// --- Patronymic -----------------------------------------------------------

// SavePatronymic сохраняет словарную запись отчества.
func (s *Store) SavePatronymic(v *models.Patronymic) error {
	return s.saveDictionary(patronymicSpec, dictValue{
		id: v.ID, vals: []any{v.Canonical},
		variants: v.Variants, items: v.Items, notes: v.Notes,
	})
}

// GetPatronymic читает отчество по id; не найдено — (nil, nil).
func (s *Store) GetPatronymic(id models.ID) (*models.Patronymic, error) {
	var canonical string

	v, ok, err := s.getDictionary(patronymicSpec, id, &canonical)
	if err != nil || !ok {
		return nil, err
	}

	return &models.Patronymic{
		ID: id, Canonical: canonical,
		Variants: v.variants, Items: v.items, Notes: v.notes,
	}, nil
}

// ListPatronymics возвращает все отчества в порядке вставки.
func (s *Store) ListPatronymics() ([]*models.Patronymic, error) {
	return listEntities(s, "patronymics", s.GetPatronymic)
}

// --- Estate ---------------------------------------------------------------

// SaveEstate сохраняет словарную запись сословия.
func (s *Store) SaveEstate(v *models.Estate) error {
	return s.saveDictionary(estateSpec, dictValue{
		id: v.ID, vals: []any{v.Canonical},
		variants: v.Variants, items: v.Items, notes: v.Notes,
	})
}

// GetEstate читает сословие по id; не найдено — (nil, nil).
func (s *Store) GetEstate(id models.ID) (*models.Estate, error) {
	var canonical string

	v, ok, err := s.getDictionary(estateSpec, id, &canonical)
	if err != nil || !ok {
		return nil, err
	}

	return &models.Estate{
		ID: id, Canonical: canonical,
		Variants: v.variants, Items: v.items, Notes: v.notes,
	}, nil
}

// ListEstates возвращает все сословия в порядке вставки.
func (s *Store) ListEstates() ([]*models.Estate, error) {
	return listEntities(s, "estates", s.GetEstate)
}

// --- Title ----------------------------------------------------------------

// SaveTitle сохраняет словарную запись звания/титула.
func (s *Store) SaveTitle(v *models.Title) error {
	return s.saveDictionary(titleSpec, dictValue{
		id: v.ID, vals: []any{v.Canonical},
		variants: v.Variants, items: v.Items, notes: v.Notes,
	})
}

// GetTitle читает звание по id; не найдено — (nil, nil).
func (s *Store) GetTitle(id models.ID) (*models.Title, error) {
	var canonical string

	v, ok, err := s.getDictionary(titleSpec, id, &canonical)
	if err != nil || !ok {
		return nil, err
	}

	return &models.Title{
		ID: id, Canonical: canonical,
		Variants: v.variants, Items: v.items, Notes: v.notes,
	}, nil
}

// ListTitles возвращает все звания в порядке вставки.
func (s *Store) ListTitles() ([]*models.Title, error) {
	return listEntities(s, "titles", s.GetTitle)
}
