package models

// Validate проверяет событие: вид обязателен (открытый enum); дата, место
// (PlaceRef) и участники необязательны; у участника — персона и роль.
func (e *Event) Validate() error {
	return finish(TypeEvent, e.validate())
}

func (e *Event) validate() *ValidationError {
	if err := idErr("id", e.ID, TypeEvent); err != nil {
		return err
	}
	if !validOpenEnum(string(e.Type)) {
		return fieldErr("type", "недопустимый вид события %q: ожидается [a-z][a-z0-9_-]*", e.Type)
	}

	if e.Date != nil {
		if err := e.Date.validate(); err != nil {
			return err.within("date")
		}
	}
	if e.Place != nil {
		if err := e.Place.validate(); err != nil {
			return err.within("place")
		}
	}

	for i, p := range e.Participants {
		if err := p.validate(); err != nil {
			return err.within(indexed("participants", i))
		}
	}

	if err := validateSourceLinks("sources", e.Sources); err != nil {
		return err
	}

	return validateTextRefs("notes", e.Notes, "")
}

// validate проверяет участника: персона обязательна, роль — непустая.
func (p EventParticipant) validate() *ValidationError {
	if e := idErr("person_id", p.PersonID, TypePerson); e != nil {
		return e
	}

	return requireText("role", p.Role)
}
