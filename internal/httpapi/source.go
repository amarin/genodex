package httpapi

import (
	"net/http"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/transport"
)

// handleSourceList — GET /api/sources?limit=&offset=. Чтение открыто
// анонимному посетителю.
func handleSourceList(sources SourceService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page, err := parsePage(r.URL.Query())
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})

			return
		}

		list, err := sources.ListSources(r.Context(), AccessFromContext(r.Context()), page)
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.SourcesFromModels(list))
	}
}

// handleSourceSearch — GET /api/sources/search?q=&limit=&offset=.
func handleSourceSearch(sources SourceService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		page, err := parsePage(q)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})

			return
		}

		list, err := sources.SearchSources(r.Context(), AccessFromContext(r.Context()),
			models.SearchQuery{Text: q.Get("q"), Page: page})
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.SourcesFromModels(list))
	}
}

// handleSourceGet — GET /api/sources/{id}.
func handleSourceGet(sources SourceService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		src, err := sources.GetSource(r.Context(), AccessFromContext(r.Context()), pathID(r))
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.SourceFromModel(src))
	}
}
