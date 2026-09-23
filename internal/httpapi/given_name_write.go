package httpapi

import (
	"net/http"

	"github.com/amarin/genodex/internal/transport"
)

// handleGivenNameCreate — POST /api/given-names: создаёт запись, отвечает 201 с
// созданной записью (id генерирует сценарий). Запись — только для вошедшего
// владельца (auth.md §6, решение 9), см. handleDivisionCreate.
func handleGivenNameCreate(givenNames GivenNameService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		var in transport.GivenNameCreate
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось разобрать тело: " + err.Error()})

			return
		}

		created, err := givenNames.CreateGivenName(r.Context(), in.Model())
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusCreated, transport.GivenNameFromModel(created))
	}
}

// handleGivenNameUpdate — PUT /api/given-names/{id}: полная замена
// canonical/variants/items/notes. Читает текущую версию, накладывает поля
// запроса (fetch-then-merge — как handleDivisionUpdate; у GivenName сейчас DTO
// покрывает всю модель, но конвенция общая для будущих сущностей, чей DTO
// v1 не покрывает всех полей модели, docs/data-model/entity-write.md §3).
func handleGivenNameUpdate(givenNames GivenNameService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		id := pathID(r)

		var in transport.GivenNameUpdate
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось разобрать тело: " + err.Error()})

			return
		}

		cur, err := givenNames.GetGivenName(r.Context(), id)
		if err != nil {
			writeError(w, err)

			return
		}

		m := in.Model()
		cur.Canonical = m.Canonical
		cur.Gender = m.Gender
		cur.Variants = m.Variants
		cur.Items = m.Items
		cur.Notes = m.Notes

		if err := givenNames.UpdateGivenName(r.Context(), cur); err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.GivenNameFromModel(cur))
	}
}

// handleGivenNameDelete — DELETE /api/given-names/{id}: 204 без тела; занятая
// запись — 409 со списком ссылающихся. Запись — только для вошедшего
// владельца, см. handleDivisionCreate.
func handleGivenNameDelete(givenNames GivenNameService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		if err := givenNames.DeleteGivenName(r.Context(), pathID(r)); err != nil {
			writeError(w, err)

			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
