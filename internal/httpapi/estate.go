package httpapi

import (
	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/transport"
	"net/http"
)

// handleEstateList — GET /api/estates?limit=&offset=. Чтение открыто
// анонимному посетителю (как и деления — auth.md §6, Access здесь не
// проверяется).
func handleEstateList(estates EstateService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page, err := parsePage(r.URL.Query())
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})

			return
		}

		list, err := estates.ListEstates(r.Context(), AccessFromContext(r.Context()), page)
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.EstatesFromModels(list))
	}
}

// handleEstateSearch — GET /api/estates/search?q=&limit=&offset=.
func handleEstateSearch(estates EstateService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		page, err := parsePage(q)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})

			return
		}

		list, err := estates.SearchEstates(r.Context(), AccessFromContext(r.Context()),
			models.SearchQuery{Text: q.Get("q"), Page: page})
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.EstatesFromModels(list))
	}
}

// handleEstateGet — GET /api/estates/{id}.
func handleEstateGet(estates EstateService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sn, err := estates.GetEstate(r.Context(), pathID(r))
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.EstateFromModel(sn))
	}
}
