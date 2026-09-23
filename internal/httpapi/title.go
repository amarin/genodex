package httpapi

import (
	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/transport"
	"net/http"
)

// handleTitleList — GET /api/titles?limit=&offset=. Чтение открыто
// анонимному посетителю (как и деления — auth.md §6, Access здесь не
// проверяется).
func handleTitleList(titles TitleService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page, err := parsePage(r.URL.Query())
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})

			return
		}

		list, err := titles.ListTitles(r.Context(), AccessFromContext(r.Context()), page)
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.TitlesFromModels(list))
	}
}

// handleTitleSearch — GET /api/titles/search?q=&limit=&offset=.
func handleTitleSearch(titles TitleService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		page, err := parsePage(q)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})

			return
		}

		list, err := titles.SearchTitles(r.Context(), AccessFromContext(r.Context()),
			models.SearchQuery{Text: q.Get("q"), Page: page})
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.TitlesFromModels(list))
	}
}

// handleTitleGet — GET /api/titles/{id}.
func handleTitleGet(titles TitleService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sn, err := titles.GetTitle(r.Context(), pathID(r))
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.TitleFromModel(sn))
	}
}
