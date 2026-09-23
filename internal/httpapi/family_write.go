package httpapi

import (
	"net/http"

	"github.com/amarin/genodex/internal/transport"
)

// handleFamilyCreate — POST /api/families: создаёт запись, отвечает
// 201 с созданной записью (id генерирует сценарий). Запись — только для
// вошедшего владельца, см. handleDivisionCreate.
func handleFamilyCreate(families FamilyService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		var in transport.FamilyCreate
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось разобрать тело: " + err.Error()})

			return
		}

		created, err := families.CreateFamily(r.Context(), in.Model())
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusCreated, transport.FamilyFromModel(created))
	}
}

// handleFamilyUpdate — PUT /api/families/{id}: полная замена
// name/members/notes/sources/private. Читает текущую версию, накладывает
// поля запроса (fetch-then-merge, docs/data-model/entity-write.md §3).
func handleFamilyUpdate(families FamilyService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		id := pathID(r)

		var in transport.FamilyUpdate
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось разобрать тело: " + err.Error()})

			return
		}

		cur, err := families.GetFamily(r.Context(), AccessFromContext(r.Context()), id)
		if err != nil {
			writeError(w, err)

			return
		}

		m := in.Model()
		cur.Name = m.Name
		cur.Members = m.Members
		cur.Notes = m.Notes
		cur.Sources = m.Sources
		cur.Private = m.Private

		if err := families.UpdateFamily(r.Context(), cur); err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.FamilyFromModel(cur))
	}
}

// handleFamilyDelete — DELETE /api/families/{id}: 204 без тела; занятая
// запись — 409 со списком ссылающихся. Запись — только для вошедшего
// владельца, см. handleDivisionCreate.
func handleFamilyDelete(families FamilyService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		if err := families.DeleteFamily(r.Context(), pathID(r)); err != nil {
			writeError(w, err)

			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
