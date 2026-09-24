package httpapi

import (
	"net/http"

	"github.com/amarin/genodex/internal/transport"
)

// handleEventCreate — POST /api/events: создаёт событие, отвечает 201 с
// созданной записью (id генерирует сценарий). Несуществующий
// participants[i].person_id — 422 (см. writeError); Place — мягкая ссылка,
// НИКОГДА не проверяется на существование. Запись — только для вошедшего
// владельца, см. handleFamilyCreate.
func handleEventCreate(events EventService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		var in transport.EventCreate
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось разобрать тело: " + err.Error()})

			return
		}

		created, err := events.CreateEvent(r.Context(), in.Model())
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusCreated, transport.EventFromModel(created))
	}
}

// handleEventUpdate — PUT /api/events/{id}: полная замена type/date/place/
// participants/sources/notes/private (fetch-then-merge, docs/data-model/
// entity-write.md §3).
func handleEventUpdate(events EventService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		id := pathID(r)

		var in transport.EventUpdate
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось разобрать тело: " + err.Error()})

			return
		}

		cur, err := events.GetEvent(r.Context(), AccessFromContext(r.Context()), id)
		if err != nil {
			writeError(w, err)

			return
		}

		m := in.Model()
		cur.Type = m.Type
		cur.Date = m.Date
		cur.Place = m.Place
		cur.Participants = m.Participants
		cur.Sources = m.Sources
		cur.Notes = m.Notes
		cur.Private = m.Private

		if err := events.UpdateEvent(r.Context(), cur); err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.EventFromModel(cur))
	}
}

// handleEventDelete — DELETE /api/events/{id}: 204 без тела; занятая запись —
// 409 со списком ссылающихся. Запись — только для вошедшего владельца, см.
// handleFamilyCreate.
func handleEventDelete(events EventService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		if err := events.DeleteEvent(r.Context(), pathID(r)); err != nil {
			writeError(w, err)

			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
