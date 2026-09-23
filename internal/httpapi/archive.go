package httpapi

import (
	"net/http"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/transport"
)

// handleArchiveList — GET /api/archives?limit=&offset=. Чтение открыто
// анонимному посетителю.
func handleArchiveList(archives ArchiveService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page, err := parsePage(r.URL.Query())
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})

			return
		}

		list, err := archives.ListArchives(r.Context(), AccessFromContext(r.Context()), page)
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.ArchivesFromModels(list))
	}
}

// handleArchiveSearch — GET /api/archives/search?q=&limit=&offset=.
func handleArchiveSearch(archives ArchiveService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		page, err := parsePage(q)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})

			return
		}

		list, err := archives.SearchArchives(r.Context(), AccessFromContext(r.Context()),
			models.SearchQuery{Text: q.Get("q"), Page: page})
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.ArchivesFromModels(list))
	}
}

// handleArchiveGet — GET /api/archives/{id}.
func handleArchiveGet(archives ArchiveService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		a, err := archives.GetArchive(r.Context(), AccessFromContext(r.Context()), pathID(r))
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.ArchiveFromModel(a))
	}
}
