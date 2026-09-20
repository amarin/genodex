package models

// Validate проверяет проживание: персона и место (единица административного
// деления) — корректные ссылки, период корректен.
func (r *Residence) Validate() error {
	return finish(TypeResidence, r.validate())
}

func (r *Residence) validate() *ValidationError {
	if e := idErr("id", r.ID, TypeResidence); e != nil {
		return e
	}
	if e := idErr("person_id", r.PersonID, TypePerson); e != nil {
		return e
	}
	if e := idErr("place_id", r.PlaceID, TypeAdministrativeDivision); e != nil {
		return e
	}

	if e := validatePeriod(r.Since, r.Until); e != nil {
		return e
	}

	return validateSourceLinks("sources", r.Sources)
}
