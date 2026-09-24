package httpapi

import (
	"net/http"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/transport"
)

// handleChurchList — GET /api/churches?limit=&offset=. Чтение открыто
// анонимному посетителю.
func handleChurchList(churches ChurchService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page, err := parsePage(r.URL.Query())
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})

			return
		}

		list, err := churches.ListChurches(r.Context(), AccessFromContext(r.Context()), page)
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.ChurchesFromModels(list))
	}
}

// handleChurchSearch — GET /api/churches/search?q=&limit=&offset=.
func handleChurchSearch(churches ChurchService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		page, err := parsePage(q)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})

			return
		}

		list, err := churches.SearchChurches(r.Context(), AccessFromContext(r.Context()),
			models.SearchQuery{Text: q.Get("q"), Page: page})
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.ChurchesFromModels(list))
	}
}

// handleChurchGet — GET /api/churches/{id}.
func handleChurchGet(churches ChurchService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, err := churches.GetChurch(r.Context(), AccessFromContext(r.Context()), pathID(r))
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.ChurchFromModel(c))
	}
}
