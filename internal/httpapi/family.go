package httpapi

import (
	"net/http"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/transport"
)

// handleFamilyList — GET /api/families?limit=&offset=. Чтение открыто
// анонимному посетителю.
func handleFamilyList(families FamilyService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page, err := parsePage(r.URL.Query())
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})

			return
		}

		list, err := families.ListFamilies(r.Context(), AccessFromContext(r.Context()), page)
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.FamiliesFromModels(list))
	}
}

// handleFamilySearch — GET /api/families/search?q=&limit=&offset=.
func handleFamilySearch(families FamilyService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		page, err := parsePage(q)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})

			return
		}

		list, err := families.SearchFamilies(r.Context(), AccessFromContext(r.Context()),
			models.SearchQuery{Text: q.Get("q"), Page: page})
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.FamiliesFromModels(list))
	}
}

// handleFamilyGet — GET /api/families/{id}.
func handleFamilyGet(families FamilyService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		f, err := families.GetFamily(r.Context(), AccessFromContext(r.Context()), pathID(r))
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.FamilyFromModel(f))
	}
}
