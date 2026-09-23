package httpapi

import (
	"net/http"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/transport"
)

// handleParishList — GET /api/parishes?limit=&offset=. Чтение открыто
// анонимному посетителю.
func handleParishList(parishes ParishService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page, err := parsePage(r.URL.Query())
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})

			return
		}

		list, err := parishes.ListParishes(r.Context(), AccessFromContext(r.Context()), page)
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.ParishesFromModels(list))
	}
}

// handleParishSearch — GET /api/parishes/search?q=&limit=&offset=.
func handleParishSearch(parishes ParishService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		page, err := parsePage(q)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})

			return
		}

		list, err := parishes.SearchParishes(r.Context(), AccessFromContext(r.Context()),
			models.SearchQuery{Text: q.Get("q"), Page: page})
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.ParishesFromModels(list))
	}
}

// handleParishGet — GET /api/parishes/{id}.
func handleParishGet(parishes ParishService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, err := parishes.GetParish(r.Context(), pathID(r))
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.ParishFromModel(p))
	}
}
