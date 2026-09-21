package sqlstore

import (
	"context"
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
func (s *Store) saveDictionary(ctx context.Context, spec dictSpec, v dictValue) error {
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

	return s.inTx(ctx, func(tx runner) error {
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
// записи нет — models.ErrNotFound.
func (s *Store) getDictionary(ctx context.Context, spec dictSpec, id models.ID, dest ...*string) (dictValue, error) {
	var v dictValue

	ptrs := make([]any, 0, len(dest))
	for _, d := range dest {
		ptrs = append(ptrs, d)
	}

	err := s.run(ctx).QueryRow(
		`SELECT `+strings.Join(spec.cols, ", ")+` FROM `+spec.table+` WHERE id = ?`, string(id),
	).Scan(ptrs...)
	if notFound(err) {
		return v, models.ErrNotFound
	}

	if err != nil {
		return v, err
	}

	v.id = id
	lists := spec.lists()

	out := [3][]models.TextRef{}

	for i, table := range lists {
		items, err := loadTextRefList(s.run(ctx), table, spec.ownerCol, string(id))
		if err != nil {
			return v, err
		}

		out[i] = items
	}

	v.variants, v.items, v.notes = out[0], out[1], out[2]

	return v, nil
}

// --- Surname --------------------------------------------------------------

// SaveSurname сохраняет словарную запись фамилии.
func (s *Store) SaveSurname(ctx context.Context, v *models.Surname) (err error) {
	defer wrapSave(&err, "surname", v.ID)

	return s.saveDictionary(ctx, surnameSpec, dictValue{
		id: v.ID, vals: []any{v.Canonical},
		variants: v.Variants, items: v.Items, notes: v.Notes,
	})
}

// GetSurname читает фамилию по id; не найдена — models.ErrNotFound.
func (s *Store) GetSurname(ctx context.Context, id models.ID) (*models.Surname, error) {
	var canonical string

	v, err := s.getDictionary(ctx, surnameSpec, id, &canonical)
	if err != nil {
		return nil, err
	}

	return &models.Surname{
		ID: id, Canonical: canonical,
		Variants: v.variants, Items: v.items, Notes: v.notes,
	}, nil
}

// ListSurnames возвращает все фамилии в порядке вставки.
func (s *Store) ListSurnames(ctx context.Context, access models.Access, page models.Page) ([]*models.Surname, error) {
	return listEntities(ctx, s, "surnames", access, page, s.GetSurname)
}

// --- GivenName ------------------------------------------------------------

// SaveGivenName сохраняет словарную запись имени.
func (s *Store) SaveGivenName(ctx context.Context, v *models.GivenName) (err error) {
	defer wrapSave(&err, "given name", v.ID)

	return s.saveDictionary(ctx, givenNameSpec, dictValue{
		id: v.ID, vals: []any{v.Canonical, string(v.Gender)},
		variants: v.Variants, items: v.Items, notes: v.Notes,
	})
}

// GetGivenName читает имя по id; не найдено — models.ErrNotFound.
func (s *Store) GetGivenName(ctx context.Context, id models.ID) (*models.GivenName, error) {
	var canonical, gender string

	v, err := s.getDictionary(ctx, givenNameSpec, id, &canonical, &gender)
	if err != nil {
		return nil, err
	}

	return &models.GivenName{
		ID: id, Canonical: canonical, Gender: models.NameGender(gender),
		Variants: v.variants, Items: v.items, Notes: v.notes,
	}, nil
}

// ListGivenNames возвращает все имена в порядке вставки.
func (s *Store) ListGivenNames(ctx context.Context, access models.Access, page models.Page) ([]*models.GivenName, error) {
	return listEntities(ctx, s, "given_names", access, page, s.GetGivenName)
}

// --- Patronymic -----------------------------------------------------------

// SavePatronymic сохраняет словарную запись отчества.
func (s *Store) SavePatronymic(ctx context.Context, v *models.Patronymic) (err error) {
	defer wrapSave(&err, "patronymic", v.ID)

	return s.saveDictionary(ctx, patronymicSpec, dictValue{
		id: v.ID, vals: []any{v.Canonical},
		variants: v.Variants, items: v.Items, notes: v.Notes,
	})
}

// GetPatronymic читает отчество по id; не найдено — models.ErrNotFound.
func (s *Store) GetPatronymic(ctx context.Context, id models.ID) (*models.Patronymic, error) {
	var canonical string

	v, err := s.getDictionary(ctx, patronymicSpec, id, &canonical)
	if err != nil {
		return nil, err
	}

	return &models.Patronymic{
		ID: id, Canonical: canonical,
		Variants: v.variants, Items: v.items, Notes: v.notes,
	}, nil
}

// ListPatronymics возвращает все отчества в порядке вставки.
func (s *Store) ListPatronymics(ctx context.Context, access models.Access, page models.Page) ([]*models.Patronymic, error) {
	return listEntities(ctx, s, "patronymics", access, page, s.GetPatronymic)
}

// --- Estate ---------------------------------------------------------------

// SaveEstate сохраняет словарную запись сословия.
func (s *Store) SaveEstate(ctx context.Context, v *models.Estate) (err error) {
	defer wrapSave(&err, "estate", v.ID)

	return s.saveDictionary(ctx, estateSpec, dictValue{
		id: v.ID, vals: []any{v.Canonical},
		variants: v.Variants, items: v.Items, notes: v.Notes,
	})
}

// GetEstate читает сословие по id; не найдено — models.ErrNotFound.
func (s *Store) GetEstate(ctx context.Context, id models.ID) (*models.Estate, error) {
	var canonical string

	v, err := s.getDictionary(ctx, estateSpec, id, &canonical)
	if err != nil {
		return nil, err
	}

	return &models.Estate{
		ID: id, Canonical: canonical,
		Variants: v.variants, Items: v.items, Notes: v.notes,
	}, nil
}

// ListEstates возвращает все сословия в порядке вставки.
func (s *Store) ListEstates(ctx context.Context, access models.Access, page models.Page) ([]*models.Estate, error) {
	return listEntities(ctx, s, "estates", access, page, s.GetEstate)
}

// --- Title ----------------------------------------------------------------

// SaveTitle сохраняет словарную запись звания/титула.
func (s *Store) SaveTitle(ctx context.Context, v *models.Title) (err error) {
	defer wrapSave(&err, "title", v.ID)

	return s.saveDictionary(ctx, titleSpec, dictValue{
		id: v.ID, vals: []any{v.Canonical},
		variants: v.Variants, items: v.Items, notes: v.Notes,
	})
}

// GetTitle читает звание по id; не найдено — models.ErrNotFound.
func (s *Store) GetTitle(ctx context.Context, id models.ID) (*models.Title, error) {
	var canonical string

	v, err := s.getDictionary(ctx, titleSpec, id, &canonical)
	if err != nil {
		return nil, err
	}

	return &models.Title{
		ID: id, Canonical: canonical,
		Variants: v.variants, Items: v.items, Notes: v.notes,
	}, nil
}

// ListTitles возвращает все звания в порядке вставки.
func (s *Store) ListTitles(ctx context.Context, access models.Access, page models.Page) ([]*models.Title, error) {
	return listEntities(ctx, s, "titles", access, page, s.GetTitle)
}
