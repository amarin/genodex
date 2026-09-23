package httpapi

import (
	"net/http"

	"github.com/amarin/genodex/internal/transport"
)

// handleChurchCreate — POST /api/churches: создаёт запись, отвечает 201 с
// созданной записью (id генерирует сценарий). Запись — только для вошедшего
// владельца, см. handleDivisionCreate.
func handleChurchCreate(churches ChurchService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		var in transport.ChurchCreate
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось разобрать тело: " + err.Error()})

			return
		}

		created, err := churches.CreateChurch(r.Context(), in.Model())
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusCreated, transport.ChurchFromModel(created))
	}
}

// handleChurchUpdate — PUT /api/churches/{id}: полная замена
// name/parish/settlements/variants/notes/sources. Читает текущую версию,
// накладывает поля запроса (fetch-then-merge).
func handleChurchUpdate(churches ChurchService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		id := pathID(r)

		var in transport.ChurchUpdate
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось разобрать тело: " + err.Error()})

			return
		}

		cur, err := churches.GetChurch(r.Context(), id)
		if err != nil {
			writeError(w, err)

			return
		}

		m := in.Model()
		cur.Name = m.Name
		cur.Parish = m.Parish
		cur.Settlements = m.Settlements
		cur.Variants = m.Variants
		cur.Notes = m.Notes
		cur.Sources = m.Sources

		if err := churches.UpdateChurch(r.Context(), cur); err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.ChurchFromModel(cur))
	}
}

// handleChurchDelete — DELETE /api/churches/{id}: 204 без тела; занятая
// запись — 409 со списком ссылающихся. Запись — только для вошедшего
// владельца, см. handleDivisionCreate.
func handleChurchDelete(churches ChurchService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		if err := churches.DeleteChurch(r.Context(), pathID(r)); err != nil {
			writeError(w, err)

			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
