package models

// Validate проверяет единицу административного деления: название и вид
// (закрытый enum) обязательны; родитель — деление, не совпадающее с самой
// единицей (циклы длиннее проверяют сценарии); составляющие и преемники
// ссылаются на деления; варианты — непустые строки; переименования и период
// корректны.
func (a *AdministrativeDivision) Validate() error {
	return finish(TypeAdministrativeDivision, a.validate())
}

func (a *AdministrativeDivision) validate() *ValidationError {
	if e := idErr("id", a.ID, TypeAdministrativeDivision); e != nil {
		return e
	}
	if e := requireText("name", a.Name); e != nil {
		return e
	}
	if !a.Type.Valid() {
		return fieldErr("type", "недопустимый тип единицы деления %q", a.Type)
	}

	if e := validateOptionalIDPtr("parent_id", a.ParentID, TypeAdministrativeDivision); e != nil {
		return e
	}
	if e := validateNotSelf(a.ID, a.ParentID); e != nil {
		return e
	}

	if e := validateTextRefs("items", a.Items, TypeAdministrativeDivision); e != nil {
		return e
	}
	if e := validateStrings("variants", a.Variants); e != nil {
		return e
	}
	if e := validateRenames("renames", a.Renames); e != nil {
		return e
	}
	if e := validateTextRefs("successors", a.Successors, TypeAdministrativeDivision); e != nil {
		return e
	}

	if e := validatePeriod(a.Since, a.Until); e != nil {
		return e
	}
	if e := validateTextRefs("notes", a.Notes, ""); e != nil {
		return e
	}

	return validateSourceLinks("sources", a.Sources)
}
