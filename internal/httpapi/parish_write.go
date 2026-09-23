package httpapi

import (
	"net/http"

	"github.com/amarin/genodex/internal/transport"
)

// handleParishCreate — POST /api/parishes: создаёт запись, отвечает 201 с
// созданной записью (id генерирует сценарий). Запись — только для вошедшего
// владельца, см. handleDivisionCreate.
func handleParishCreate(parishes ParishService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		var in transport.ParishCreate
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось разобрать тело: " + err.Error()})

			return
		}

		created, err := parishes.CreateParish(r.Context(), in.Model())
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusCreated, transport.ParishFromModel(created))
	}
}

// handleParishUpdate — PUT /api/parishes/{id}: полная замена
// name/church/settlements/since/until/notes. Читает текущую версию,
// накладывает поля запроса (fetch-then-merge). Sources не в DTO — read-only
// в v1.
func handleParishUpdate(parishes ParishService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		id := pathID(r)

		var in transport.ParishUpdate
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось разобрать тело: " + err.Error()})

			return
		}

		cur, err := parishes.GetParish(r.Context(), id)
		if err != nil {
			writeError(w, err)

			return
		}

		m := in.Model()
		cur.Name = m.Name
		cur.Church = m.Church
		cur.Settlements = m.Settlements
		cur.Since = m.Since
		cur.Until = m.Until
		cur.Notes = m.Notes

		if err := parishes.UpdateParish(r.Context(), cur); err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.ParishFromModel(cur))
	}
}

// handleParishDelete — DELETE /api/parishes/{id}: 204 без тела; занятая
// запись — 409 со списком ссылающихся. Запись — только для вошедшего
// владельца, см. handleDivisionCreate.
func handleParishDelete(parishes ParishService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		if err := parishes.DeleteParish(r.Context(), pathID(r)); err != nil {
			writeError(w, err)

			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
