package models

// Validate проверяет источник: вид, название и общая достоверность обязательны
// (вид и достоверность — закрытые enum'ы); автор и заметки необязательны; дата
// корректна; хранилище (если задано) — repository.
func (s *Source) Validate() error {
	return finish(TypeSource, s.validate())
}

func (s *Source) validate() *ValidationError {
	if e := idErr("id", s.ID, TypeSource); e != nil {
		return e
	}
	if !s.Kind.Valid() {
		return fieldErr("kind", "недопустимый вид источника %q", s.Kind)
	}
	if e := requireText("title", s.Title); e != nil {
		return e
	}

	if s.Date != nil {
		if e := s.Date.validate(); e != nil {
			return e.within("date")
		}
	}
	if !s.Reliability.Valid() {
		return fieldErr("reliability", "недопустимая достоверность %q", s.Reliability)
	}
	if e := validateOptionalID("repository_id", s.RepositoryID, TypeRepository); e != nil {
		return e
	}

	return validateTextRefs("notes", s.Notes, "")
}
