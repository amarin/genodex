package models

// Validate проверяет приход: название обязательно; церковь — ссылка на church
// (или текст); населённые пункты — ссылки на деления; период корректен.
func (p *Parish) Validate() error {
	return finish(TypeParish, p.validate())
}

func (p *Parish) validate() *ValidationError {
	if e := idErr("id", p.ID, TypeParish); e != nil {
		return e
	}
	if e := requireText("name", p.Name); e != nil {
		return e
	}

	if e := validateOptionalTextRefPtr("church", p.Church, TypeChurch); e != nil {
		return e
	}
	if e := validateTextRefs("settlements", p.Settlements, TypeAdministrativeDivision); e != nil {
		return e
	}
	if e := validatePeriod(p.Since, p.Until); e != nil {
		return e
	}
	if e := validateTextRefs("notes", p.Notes, ""); e != nil {
		return e
	}

	return validateSourceLinks("sources", p.Sources)
}
