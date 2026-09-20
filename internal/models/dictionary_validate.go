package models

import "strings"

// Validate проверяет запись словаря фамилий.
func (s *Surname) Validate() error {
	return finish(TypeSurname, validateDictionary(TypeSurname, s.ID, s.Canonical, s.Variants, s.Items, s.Notes))
}

// Validate проверяет запись словаря имён; признак пола обязателен.
func (g *GivenName) Validate() error {
	if e := validateDictionary(TypeGivenName, g.ID, g.Canonical, g.Variants, g.Items, g.Notes); e != nil {
		return finish(TypeGivenName, e)
	}
	if !g.Gender.Valid() {
		return finish(TypeGivenName, fieldErr("gender", "обязателен допустимый пол имени, получено %q", g.Gender))
	}

	return nil
}

// Validate проверяет запись словаря отчеств.
func (p *Patronymic) Validate() error {
	return finish(TypePatronymic, validateDictionary(TypePatronymic, p.ID, p.Canonical, p.Variants, p.Items, p.Notes))
}

// Validate проверяет запись словаря сословий.
func (e *Estate) Validate() error {
	return finish(TypeEstate, validateDictionary(TypeEstate, e.ID, e.Canonical, e.Variants, e.Items, e.Notes))
}

// Validate проверяет запись словаря титулов.
func (t *Title) Validate() error {
	return finish(TypeTitle, validateDictionary(TypeTitle, t.ID, t.Canonical, t.Variants, t.Items, t.Notes))
}

// validateDictionary — общие правила словарной записи: id нужного типа,
// непустая каноническая форма, варианты — того же типа, что и словарь,
// носители и заметки — любые корректные TextRef.
func validateDictionary(t Type, id ID, canonical string, variants, items, notes []TextRef) *ValidationError {
	if err := id.Validate(t); err != nil {
		return fieldErr("id", "%v", err)
	}
	if strings.TrimSpace(canonical) == "" {
		return fieldErr("canonical", "каноническая форма обязательна")
	}
	if e := validateTextRefs("variants", variants, t); e != nil {
		return e
	}
	if e := validateTextRefs("items", items, ""); e != nil {
		return e
	}

	return validateTextRefs("notes", notes, "")
}
