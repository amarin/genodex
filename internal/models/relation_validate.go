package models

// Validate проверяет ребро графа: вид обязателен и допустим; rel_type — только
// для kind=associate (обязателен и в формате открытого enum'а), у остальных
// видов пуст; стороны — персоны и не совпадают; период корректен.
func (r *Relation) Validate() error {
	return finish(TypeRelation, r.validate())
}

func (r *Relation) validate() *ValidationError {
	if err := r.ID.Validate(TypeRelation); err != nil {
		return fieldErr("id", "%v", err)
	}
	if !r.Kind.Valid() {
		return fieldErr("kind", "недопустимый вид связи %q", r.Kind)
	}

	if r.Kind == RelationKindAssociate {
		if r.RelType == "" {
			return fieldErr("rel_type", "обязателен при kind=associate")
		}
		if !validOpenEnum(string(r.RelType)) {
			return fieldErr("rel_type", "недопустимый формат %q: ожидается [a-z][a-z0-9_-]*", r.RelType)
		}
	} else if r.RelType != "" {
		return fieldErr("rel_type", "допустим только при kind=associate")
	}

	if err := r.PersonA.Validate(TypePerson); err != nil {
		return fieldErr("person_a", "%v", err)
	}
	if err := r.PersonB.Validate(TypePerson); err != nil {
		return fieldErr("person_b", "%v", err)
	}
	if r.PersonA == r.PersonB {
		return fieldErr("person_b", "связь персоны с самой собой")
	}

	if e := validatePeriod(r.Since, r.Until); e != nil {
		return e
	}
	if e := validateSourceLinks("sources", r.Sources); e != nil {
		return e
	}

	return validateTextRefs("notes", r.Notes, "")
}
