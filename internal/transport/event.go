package transport

import "github.com/amarin/genodex/internal/models"

// Event — контракт события жизненного факта (GET /api/events, MCP-тул
// event_list). Sources редактируется с рождения контракта. Поиск
// (event_search, GET /api/events/search) ищет ТОЛЬКО по началу текста места
// (place) — единственное индексируемое поле события
// (internal/store/sqlstore/records.go:SaveEvent); type/date/участники
// поиском не охвачены.
type Event struct {
	ID           models.ID          `json:"id"`
	Type         string             `json:"type"`
	Date         *FactDate          `json:"date,omitempty"`
	Place        *PlaceRef          `json:"place,omitempty"`
	Participants []EventParticipant `json:"participants"`
	Sources      []SourceLink       `json:"sources"`
	Notes        []TextRef          `json:"notes"`
	Private      bool               `json:"private"`
}

// EventFromModel конвертирует запись в контракт.
func EventFromModel(e models.Event) Event {
	return Event{
		ID:           e.ID,
		Type:         string(e.Type),
		Date:         FactDateFromModel(e.Date),
		Place:        PlaceRefFromModel(e.Place),
		Participants: EventParticipantsFromModel(e.Participants),
		Sources:      SourceLinksFromModel(e.Sources),
		Notes:        TextRefsFromModel(e.Notes),
		Private:      e.Private,
	}
}

// EventsFromModels конвертирует список; пустой вход даёт пустой срез, а не nil.
func EventsFromModels(es []models.Event) []Event {
	out := make([]Event, 0, len(es))
	for _, e := range es {
		out = append(out, EventFromModel(e))
	}

	return out
}
