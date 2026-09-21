package models

// Validate проверяет хранилище-контейнер источников: название и тип
// обязательны (тип — открытый enum); адрес свободный; ссылки и заметки —
// корректные TextRef.
func (r *Repository) Validate() error {
	return finish(TypeRepository, r.validate())
}

func (r *Repository) validate() *ValidationError {
	if e := idErr("id", r.ID, TypeRepository); e != nil {
		return e
	}
	if e := requireText("name", r.Name); e != nil {
		return e
	}
	if !validOpenEnum(string(r.Type)) {
		return fieldErr("type", "недопустимый тип хранилища %q: ожидается [a-z][a-z0-9_-]*", r.Type)
	}

	if e := validateTextRefs("urls", r.URLs, ""); e != nil {
		return e
	}
	if e := validateTextRefs("notes", r.Notes, ""); e != nil {
		return e
	}

	return validateSourceLinks("sources", r.Sources)
}
