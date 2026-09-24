package httpapi

import (
	"net/http"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/transport"
)

// handlePersonList — GET /api/people?limit=&offset=. Чтение открыто
// анонимному посетителю.
func handlePersonList(people PersonService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page, err := parsePage(r.URL.Query())
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})

			return
		}

		list, err := people.ListPeople(r.Context(), AccessFromContext(r.Context()), page)
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.PeopleFromModels(list))
	}
}

// handlePersonSearch — GET /api/people/search?q=&limit=&offset=. Ищет по
// началу фамилии/имени/отчества из ЛЮБОГО из имён персоны (не только
// основного) — см. transport.Person.
func handlePersonSearch(people PersonService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		page, err := parsePage(q)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})

			return
		}

		list, err := people.SearchPeople(r.Context(), AccessFromContext(r.Context()),
			models.SearchQuery{Text: q.Get("q"), Page: page})
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.PeopleFromModels(list))
	}
}

// handlePersonGet — GET /api/people/{id}.
func handlePersonGet(people PersonService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, err := people.GetPerson(r.Context(), AccessFromContext(r.Context()), pathID(r))
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.PersonFromModel(p))
	}
}
