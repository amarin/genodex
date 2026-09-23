package httpapi

import (
	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/transport"
	"net/http"
)

// handlePatronymicList — GET /api/patronymics?limit=&offset=. Чтение открыто
// анонимному посетителю (как и деления — auth.md §6, Access здесь не
// проверяется).
func handlePatronymicList(patronymics PatronymicService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page, err := parsePage(r.URL.Query())
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})

			return
		}

		list, err := patronymics.ListPatronymics(r.Context(), AccessFromContext(r.Context()), page)
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.PatronymicsFromModels(list))
	}
}

// handlePatronymicSearch — GET /api/patronymics/search?q=&limit=&offset=.
func handlePatronymicSearch(patronymics PatronymicService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		page, err := parsePage(q)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})

			return
		}

		list, err := patronymics.SearchPatronymics(r.Context(), AccessFromContext(r.Context()),
			models.SearchQuery{Text: q.Get("q"), Page: page})
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.PatronymicsFromModels(list))
	}
}

// handlePatronymicGet — GET /api/patronymics/{id}.
func handlePatronymicGet(patronymics PatronymicService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sn, err := patronymics.GetPatronymic(r.Context(), pathID(r))
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.PatronymicFromModel(sn))
	}
}
