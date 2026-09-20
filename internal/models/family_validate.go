package models

import "strings"

// Validate проверяет род: название обязательно (непустое после обрезки
// пробелов); члены — «текст или ссылка» (ссылка на персону либо строка-имя).
func (f *Family) Validate() error {
	return finish(TypeFamily, f.validate())
}

func (f *Family) validate() *ValidationError {
	if e := idErr("id", f.ID, TypeFamily); e != nil {
		return e
	}
	if strings.TrimSpace(f.Name) == "" {
		return fieldErr("name", "название рода обязательно")
	}

	if e := validateTextRefs("members", f.Members, TypePerson); e != nil {
		return e
	}
	if e := validateTextRefs("notes", f.Notes, ""); e != nil {
		return e
	}

	return validateSourceLinks("sources", f.Sources)
}
