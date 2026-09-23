package httpapi

import (
	"net/http"

	"github.com/amarin/genodex/internal/transport"
)

// handleCitationCreate — POST /api/citations: создаёт запись, отвечает 201 с
// созданной записью (id генерирует сценарий). Несуществующий source_id или
// ссылка внутри anchor — 422. Запись — только для вошедшего владельца, см.
// handleDivisionCreate.
func handleCitationCreate(citations CitationService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		var in transport.CitationCreate
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось разобрать тело: " + err.Error()})

			return
		}

		created, err := citations.CreateCitation(r.Context(), in.Model())
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusCreated, transport.CitationFromModel(created))
	}
}

// handleCitationUpdate — PUT /api/citations/{id}: полная замена
// source_id/anchor/text/note/private. Читает текущую версию, накладывает
// поля запроса (fetch-then-merge).
func handleCitationUpdate(citations CitationService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		id := pathID(r)

		var in transport.CitationUpdate
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось разобрать тело: " + err.Error()})

			return
		}

		cur, err := citations.GetCitation(r.Context(), AccessFromContext(r.Context()), id)
		if err != nil {
			writeError(w, err)

			return
		}

		m := in.Model()
		cur.SourceID = m.SourceID
		cur.Anchor = m.Anchor
		cur.Text = m.Text
		cur.Note = m.Note
		cur.Private = m.Private

		if err := citations.UpdateCitation(r.Context(), cur); err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.CitationFromModel(cur))
	}
}

// handleCitationDelete — DELETE /api/citations/{id}: 204 без тела; занятая
// запись (есть SourceLink на неё) — 409 со списком ссылающихся. Запись —
// только для вошедшего владельца, см. handleDivisionCreate.
func handleCitationDelete(citations CitationService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		if err := citations.DeleteCitation(r.Context(), pathID(r)); err != nil {
			writeError(w, err)

			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
