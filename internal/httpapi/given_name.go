package httpapi

import (
	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/transport"
	"net/http"
)

// handleGivenNameList — GET /api/given-names?limit=&offset=. Чтение открыто
// анонимному посетителю (как и деления — auth.md §6, Access здесь не
// проверяется).
func handleGivenNameList(givenNames GivenNameService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page, err := parsePage(r.URL.Query())
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})

			return
		}

		list, err := givenNames.ListGivenNames(r.Context(), AccessFromContext(r.Context()), page)
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.GivenNamesFromModels(list))
	}
}

// handleGivenNameSearch — GET /api/given-names/search?q=&limit=&offset=.
func handleGivenNameSearch(givenNames GivenNameService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		page, err := parsePage(q)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})

			return
		}

		list, err := givenNames.SearchGivenNames(r.Context(), AccessFromContext(r.Context()),
			models.SearchQuery{Text: q.Get("q"), Page: page})
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.GivenNamesFromModels(list))
	}
}

// handleGivenNameGet — GET /api/given-names/{id}.
func handleGivenNameGet(givenNames GivenNameService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sn, err := givenNames.GetGivenName(r.Context(), pathID(r))
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.GivenNameFromModel(sn))
	}
}
