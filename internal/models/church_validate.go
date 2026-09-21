package models

// Validate проверяет церковь: название обязательно; приход — ссылка на parish
// (или текст); населённые пункты — ссылки на деления; варианты — непустые строки.
func (c *Church) Validate() error {
	return finish(TypeChurch, c.validate())
}

func (c *Church) validate() *ValidationError {
	if e := idErr("id", c.ID, TypeChurch); e != nil {
		return e
	}
	if e := requireText("name", c.Name); e != nil {
		return e
	}

	if e := validateOptionalTextRefPtr("parish", c.Parish, TypeParish); e != nil {
		return e
	}
	if e := validateTextRefs("settlements", c.Settlements, TypeAdministrativeDivision); e != nil {
		return e
	}
	if e := validateStrings("variants", c.Variants); e != nil {
		return e
	}
	if e := validateTextRefs("notes", c.Notes, ""); e != nil {
		return e
	}

	return validateSourceLinks("sources", c.Sources)
}
