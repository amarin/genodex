package models

// Validate проверяет доказательство: цитата обязательна и имеет тип citation;
// достоверность необязательна, но если задана — одна из констант. TargetType и
// TargetID выставляет адаптер из владельца при записи, здесь не проверяются.
func (l SourceLink) Validate() error {
	return finish("", l.validate())
}

func (l SourceLink) validate() *ValidationError {
	if e := idErr("citation_id", l.CitationID, TypeCitation); e != nil {
		return e
	}
	if l.Reliability != "" && !l.Reliability.Valid() {
		return fieldErr("reliability", "недопустимая достоверность %q", l.Reliability)
	}

	return nil
}

// validateSourceLinks проверяет список доказательств; путь ошибки — field[i].….
func validateSourceLinks(field string, links []SourceLink) *ValidationError {
	for i, l := range links {
		if e := l.validate(); e != nil {
			return e.within(indexed(field, i))
		}
	}

	return nil
}
