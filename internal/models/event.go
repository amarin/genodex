package models

// Event — событие жизненного факта. Private — приватность (решение #24).
type Event struct {
	ID           ID
	Type         EventType
	Date         *FactDate
	Place        *PlaceRef
	Participants []EventParticipant
	Sources      []SourceLink
	Notes        []TextRef
	Private      bool
}

// EntityType возвращает тип сущности.
func (e *Event) EntityType() Type { return TypeEvent }
