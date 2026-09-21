package models

// Validate проверяет архив: название обязательно; система иерархии (если
// задана) — только текст с именем системы, без ссылки (определение системы —
// built-in данные, а не сущность); хранилище (если задано) — repository.
func (a *Archive) Validate() error {
	return finish(TypeArchive, a.validate())
}

func (a *Archive) validate() *ValidationError {
	if e := idErr("id", a.ID, TypeArchive); e != nil {
		return e
	}
	if e := requireText("name", a.Name); e != nil {
		return e
	}

	if a.System != nil {
		if a.System.Ref != "" || a.System.Type != "" {
			return fieldErr("system.ref", "система иерархии задаётся именем, ссылка на сущность не допускается")
		}
		if e := requireText("system.text", a.System.Text); e != nil {
			return e
		}
	}

	if e := validateOptionalID("repository_id", a.RepositoryID, TypeRepository); e != nil {
		return e
	}
	if e := validateTextRefs("notes", a.Notes, ""); e != nil {
		return e
	}

	return validateSourceLinks("sources", a.Sources)
}
