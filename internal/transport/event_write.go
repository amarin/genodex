package transport

import "github.com/amarin/genodex/internal/models"

// EventCreate — тело POST /api/events и аргументы тула event_create.
// Идентификатор генерирует сценарий.
type EventCreate struct {
	Type         string             `json:"type"`
	Date         *FactDate          `json:"date,omitempty"`
	Place        *PlaceRef          `json:"place,omitempty"`
	Participants []EventParticipant `json:"participants"`
	Sources      []SourceLink       `json:"sources"`
	Notes        []TextRef          `json:"notes"`
	Private      bool               `json:"private"`
}

// Model возвращает доменную запись с пустым ID.
func (e EventCreate) Model() models.Event {
	return models.Event{
		Type:         models.EventType(e.Type),
		Date:         e.Date.Model(),
		Place:        e.Place.Model(),
		Participants: EventParticipantsToModel(e.Participants),
		Sources:      SourceLinksToModel(e.Sources),
		Notes:        TextRefsToModel(e.Notes),
		Private:      e.Private,
	}
}

// EventUpdate — тело PUT /api/events/{id} и аргументы тула event_update:
// полная замена type/date/place/participants/sources/notes/private.
type EventUpdate struct {
	Type         string             `json:"type"`
	Date         *FactDate          `json:"date,omitempty"`
	Place        *PlaceRef          `json:"place,omitempty"`
	Participants []EventParticipant `json:"participants"`
	Sources      []SourceLink       `json:"sources"`
	Notes        []TextRef          `json:"notes"`
	Private      bool               `json:"private"`
}

// Model возвращает доменную запись с пустым ID.
func (e EventUpdate) Model() models.Event {
	return models.Event{
		Type:         models.EventType(e.Type),
		Date:         e.Date.Model(),
		Place:        e.Place.Model(),
		Participants: EventParticipantsToModel(e.Participants),
		Sources:      SourceLinksToModel(e.Sources),
		Notes:        TextRefsToModel(e.Notes),
		Private:      e.Private,
	}
}
