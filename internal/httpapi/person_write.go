package httpapi

import (
	"net/http"

	"github.com/amarin/genodex/internal/transport"
)

// handlePersonCreate — POST /api/people: создаёт запись, отвечает
// 201 с созданной записью (id генерирует сценарий). Запись — только для
// вошедшего владельца, см. handleDivisionCreate.
func handlePersonCreate(people PersonService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		var in transport.PersonCreate
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось разобрать тело: " + err.Error()})

			return
		}

		created, err := people.CreatePerson(r.Context(), in.Model())
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusCreated, transport.PersonFromModel(created))
	}
}

// handlePersonUpdate — PUT /api/people/{id}: полная замена
// gender/names/estates/titles/nicknames/notes/sources/private. Читает
// текущую версию, накладывает поля запроса (fetch-then-merge,
// docs/data-model/entity-write.md §3).
func handlePersonUpdate(people PersonService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		id := pathID(r)

		var in transport.PersonUpdate
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось разобрать тело: " + err.Error()})

			return
		}

		cur, err := people.GetPerson(r.Context(), AccessFromContext(r.Context()), id)
		if err != nil {
			writeError(w, err)

			return
		}

		m := in.Model()
		cur.Gender = m.Gender
		cur.Names = m.Names
		cur.Estates = m.Estates
		cur.Titles = m.Titles
		cur.Nicknames = m.Nicknames
		cur.Notes = m.Notes
		cur.Sources = m.Sources
		cur.Private = m.Private

		if err := people.UpdatePerson(r.Context(), cur); err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.PersonFromModel(cur))
	}
}

// handlePersonDelete — DELETE /api/people/{id}: 204 без тела; занятая
// запись — 409 со списком ссылающихся. Запись — только для вошедшего
// владельца, см. handleDivisionCreate.
func handlePersonDelete(people PersonService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		if err := people.DeletePerson(r.Context(), pathID(r)); err != nil {
			writeError(w, err)

			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
