package models

// Validate проверяет проживание: персона и место (единица административного
// деления) — корректные ссылки, период корректен.
func (r *Residence) Validate() error {
	return finish(TypeResidence, r.validate())
}

func (r *Residence) validate() *ValidationError {
	if err := r.ID.Validate(TypeResidence); err != nil {
		return fieldErr("id", "%v", err)
	}
	if err := r.PersonID.Validate(TypePerson); err != nil {
		return fieldErr("person_id", "%v", err)
	}
	if err := r.PlaceID.Validate(TypeAdministrativeDivision); err != nil {
		return fieldErr("place_id", "%v", err)
	}

	if e := validatePeriod(r.Since, r.Until); e != nil {
		return e
	}

	return validateSourceLinks("sources", r.Sources)
}
