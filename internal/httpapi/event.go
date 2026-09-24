package httpapi

import (
	"net/http"
	"net/url"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/transport"
)

// handleEventList — GET /api/events?person_id=&limit=&offset=. person_id —
// необязательный фильтр по участнику (совпадает с любым из
// Participants[i].PersonID).
func handleEventList(events EventService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q, err := parseEventQuery(r.URL.Query())
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})

			return
		}

		list, err := events.ListEvents(r.Context(), AccessFromContext(r.Context()), q)
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.EventsFromModels(list))
	}
}

// parseEventQuery разбирает параметры запроса: person_id необязателен.
func parseEventQuery(v url.Values) (models.EventQuery, error) {
	var q models.EventQuery

	if pid := v.Get("person_id"); pid != "" {
		id := models.ID(pid)
		q.PersonID = &id
	}

	var err error

	if q.Page.Limit, err = intParam(v, "limit"); err != nil {
		return q, err
	}

	if q.Page.Offset, err = intParam(v, "offset"); err != nil {
		return q, err
	}

	return q, nil
}

// handleEventSearch — GET /api/events/search?q=&limit=&offset=. Ищет ТОЛЬКО
// по началу текста места (place) — единственное индексируемое поле события
// (см. transport.Event).
func handleEventSearch(events EventService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		page, err := parsePage(q)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})

			return
		}

		list, err := events.SearchEvents(r.Context(), AccessFromContext(r.Context()),
			models.SearchQuery{Text: q.Get("q"), Page: page})
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.EventsFromModels(list))
	}
}

// handleEventGet — GET /api/events/{id}.
func handleEventGet(events EventService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		e, err := events.GetEvent(r.Context(), AccessFromContext(r.Context()), pathID(r))
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.EventFromModel(e))
	}
}
