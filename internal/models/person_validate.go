package models

// Validate проверяет персону. Все поля, кроме id, необязательны; заданные
// должны быть корректны.
func (p *Person) Validate() error {
	return finish(TypePerson, p.validate())
}

func (p *Person) validate() *ValidationError {
	if err := p.ID.Validate(TypePerson); err != nil {
		return fieldErr("id", "%v", err)
	}
	if p.Gender != "" && !p.Gender.Valid() {
		return fieldErr("gender", "недопустимый пол %q", p.Gender)
	}

	for i := range p.Names {
		if e := p.Names[i].validate(); e != nil {
			return e.within(indexed("names", i))
		}
	}

	if e := validateTextRefs("estates", p.Estates, TypeEstate); e != nil {
		return e
	}
	if e := validateTextRefs("titles", p.Titles, TypeTitle); e != nil {
		return e
	}
	if e := validateTextRefs("nicknames", p.Nicknames, ""); e != nil {
		return e
	}
	if e := validateTextRefs("notes", p.Notes, ""); e != nil {
		return e
	}

	return validateSourceLinks("sources", p.Sources)
}
