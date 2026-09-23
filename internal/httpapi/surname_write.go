package httpapi

import (
	"net/http"

	"github.com/amarin/genodex/internal/transport"
)

// handleSurnameCreate — POST /api/surnames: создаёт запись, отвечает 201 с
// созданной записью (id генерирует сценарий). Запись — только для вошедшего
// владельца (auth.md §6, решение 9), см. handleDivisionCreate.
func handleSurnameCreate(surnames SurnameService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		var in transport.SurnameCreate
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось разобрать тело: " + err.Error()})

			return
		}

		created, err := surnames.CreateSurname(r.Context(), in.Model())
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusCreated, transport.SurnameFromModel(created))
	}
}

// handleSurnameUpdate — PUT /api/surnames/{id}: полная замена
// canonical/variants/items/notes. Читает текущую версию, накладывает поля
// запроса (fetch-then-merge — как handleDivisionUpdate; у Surname сейчас DTO
// покрывает всю модель, но конвенция общая для будущих сущностей, чей DTO
// v1 не покрывает всех полей модели, docs/data-model/entity-write.md §3).
func handleSurnameUpdate(surnames SurnameService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		id := pathID(r)

		var in transport.SurnameUpdate
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось разобрать тело: " + err.Error()})

			return
		}

		cur, err := surnames.GetSurname(r.Context(), id)
		if err != nil {
			writeError(w, err)

			return
		}

		m := in.Model()
		cur.Canonical = m.Canonical
		cur.Variants = m.Variants
		cur.Items = m.Items
		cur.Notes = m.Notes

		if err := surnames.UpdateSurname(r.Context(), cur); err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.SurnameFromModel(cur))
	}
}

// handleSurnameDelete — DELETE /api/surnames/{id}: 204 без тела; занятая
// запись — 409 со списком ссылающихся. Запись — только для вошедшего
// владельца, см. handleDivisionCreate.
func handleSurnameDelete(surnames SurnameService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		if err := surnames.DeleteSurname(r.Context(), pathID(r)); err != nil {
			writeError(w, err)

			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
