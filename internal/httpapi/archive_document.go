package httpapi

import (
	"net/http"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/transport"
)

// handleArchiveDocumentList — GET /api/archive-documents?limit=&offset=.
// Плоская сущность (в отличие от archive-nodes) — без обязательных фильтров.
func handleArchiveDocumentList(archiveDocuments ArchiveDocumentService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page, err := parsePage(r.URL.Query())
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})

			return
		}

		list, err := archiveDocuments.ListArchiveDocuments(r.Context(), AccessFromContext(r.Context()), page)
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.ArchiveDocumentsFromModels(list))
	}
}

// handleArchiveDocumentSearch — GET /api/archive-documents/search?q=&limit=&offset=.
func handleArchiveDocumentSearch(archiveDocuments ArchiveDocumentService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		page, err := parsePage(q)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})

			return
		}

		list, err := archiveDocuments.SearchArchiveDocuments(r.Context(), AccessFromContext(r.Context()),
			models.SearchQuery{Text: q.Get("q"), Page: page})
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.ArchiveDocumentsFromModels(list))
	}
}

// handleArchiveDocumentGet — GET /api/archive-documents/{id}.
func handleArchiveDocumentGet(archiveDocuments ArchiveDocumentService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		d, err := archiveDocuments.GetArchiveDocument(r.Context(), AccessFromContext(r.Context()), pathID(r))
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.ArchiveDocumentFromModel(d))
	}
}
