package httpapi

import (
	"net/http"
	"net/url"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/transport"
)

// handleSurnameList — GET /api/surnames?limit=&offset=. Чтение открыто
// анонимному посетителю (как и деления — auth.md §6, Access здесь не
// проверяется).
func handleSurnameList(surnames SurnameService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page, err := parsePage(r.URL.Query())
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})

			return
		}

		list, err := surnames.ListSurnames(r.Context(), AccessFromContext(r.Context()), page)
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.SurnamesFromModels(list))
	}
}

// handleSurnameSearch — GET /api/surnames/search?q=&limit=&offset=.
func handleSurnameSearch(surnames SurnameService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		page, err := parsePage(q)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})

			return
		}

		list, err := surnames.SearchSurnames(r.Context(), AccessFromContext(r.Context()),
			models.SearchQuery{Text: q.Get("q"), Page: page})
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.SurnamesFromModels(list))
	}
}

// handleSurnameGet — GET /api/surnames/{id}.
func handleSurnameGet(surnames SurnameService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sn, err := surnames.GetSurname(r.Context(), pathID(r))
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.SurnameFromModel(sn))
	}
}

// parsePage разбирает limit/offset из query-параметров.
func parsePage(v url.Values) (models.Page, error) {
	var page models.Page

	var err error

	if page.Limit, err = intParam(v, "limit"); err != nil {
		return page, err
	}

	if page.Offset, err = intParam(v, "offset"); err != nil {
		return page, err
	}

	return page, nil
}
