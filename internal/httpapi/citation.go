package httpapi

import (
	"net/http"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/transport"
)

// handleCitationList — GET /api/citations?limit=&offset=. Чтение открыто
// анонимному посетителю.
func handleCitationList(citations CitationService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page, err := parsePage(r.URL.Query())
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})

			return
		}

		list, err := citations.ListCitations(r.Context(), AccessFromContext(r.Context()), page)
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.CitationsFromModels(list))
	}
}

// handleCitationSearch — GET /api/citations/search?q=&limit=&offset=.
func handleCitationSearch(citations CitationService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		page, err := parsePage(q)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})

			return
		}

		list, err := citations.SearchCitations(r.Context(), AccessFromContext(r.Context()),
			models.SearchQuery{Text: q.Get("q"), Page: page})
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.CitationsFromModels(list))
	}
}

// handleCitationGet — GET /api/citations/{id}.
func handleCitationGet(citations CitationService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, err := citations.GetCitation(r.Context(), AccessFromContext(r.Context()), pathID(r))
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.CitationFromModel(c))
	}
}
