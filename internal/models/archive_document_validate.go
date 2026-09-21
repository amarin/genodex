package models

// Validate проверяет документ внутри единицы учёта: единица — archive_node,
// название обязательно; вид — свободный необязательный текст; период, приход и
// населённые пункты корректны.
func (d *ArchiveDocument) Validate() error {
	return finish(TypeArchiveDocument, d.validate())
}

func (d *ArchiveDocument) validate() *ValidationError {
	if e := idErr("id", d.ID, TypeArchiveDocument); e != nil {
		return e
	}
	if e := idErr("unit_id", d.UnitID, TypeArchiveNode); e != nil {
		return e
	}
	if e := requireText("title", d.Title); e != nil {
		return e
	}

	if e := validatePeriod(d.Since, d.Until); e != nil {
		return e
	}
	if e := validateOptionalTextRefPtr("parish", d.Parish, TypeParish); e != nil {
		return e
	}
	if e := validateTextRefs("settlements", d.Settlements, TypeAdministrativeDivision); e != nil {
		return e
	}
	if e := validateTextRefs("notes", d.Notes, ""); e != nil {
		return e
	}

	return validateSourceLinks("sources", d.Sources)
}
