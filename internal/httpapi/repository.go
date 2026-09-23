package httpapi

import (
	"net/http"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/transport"
)

// handleRepositoryList — GET /api/repositories?limit=&offset=. Чтение открыто
// анонимному посетителю.
func handleRepositoryList(repositories RepositoryService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page, err := parsePage(r.URL.Query())
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})

			return
		}

		list, err := repositories.ListRepositories(r.Context(), AccessFromContext(r.Context()), page)
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.RepositoriesFromModels(list))
	}
}

// handleRepositorySearch — GET /api/repositories/search?q=&limit=&offset=.
func handleRepositorySearch(repositories RepositoryService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		page, err := parsePage(q)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})

			return
		}

		list, err := repositories.SearchRepositories(r.Context(), AccessFromContext(r.Context()),
			models.SearchQuery{Text: q.Get("q"), Page: page})
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.RepositoriesFromModels(list))
	}
}

// handleRepositoryGet — GET /api/repositories/{id}.
func handleRepositoryGet(repositories RepositoryService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rep, err := repositories.GetRepository(r.Context(), pathID(r))
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.RepositoryFromModel(rep))
	}
}
