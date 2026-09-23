package httpapi

import (
	"net/http"

	"github.com/amarin/genodex/internal/transport"
)

// handleNoteCreate — POST /api/notes: создаёт запись, отвечает 201 с
// созданной записью (id генерирует сценарий). Несуществующий parent_id —
// 422 (та же механика, что и у делений/архивов). Запись — только для
// вошедшего владельца, см. handleDivisionCreate.
func handleNoteCreate(notes NoteService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		var in transport.NoteCreate
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось разобрать тело: " + err.Error()})

			return
		}

		created, err := notes.CreateNote(r.Context(), in.Model())
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusCreated, transport.NoteFromModel(created))
	}
}

// handleNoteUpdate — PUT /api/notes/{id}: полная замена
// kind/title/text/parent_id/sources. Читает текущую версию, накладывает поля
// запроса (fetch-then-merge). Владелец всегда видит запись при fetch
// (requireFull даёт полный доступ).
func handleNoteUpdate(notes NoteService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		id := pathID(r)

		var in transport.NoteUpdate
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось разобрать тело: " + err.Error()})

			return
		}

		cur, err := notes.GetNote(r.Context(), AccessFromContext(r.Context()), id)
		if err != nil {
			writeError(w, err)

			return
		}

		m := in.Model()
		cur.Kind = m.Kind
		cur.Title = m.Title
		cur.Text = m.Text
		cur.ParentID = m.ParentID
		cur.Sources = m.Sources
		cur.Private = m.Private

		if err := notes.UpdateNote(r.Context(), cur); err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.NoteFromModel(cur))
	}
}

// handleNoteDelete — DELETE /api/notes/{id}: 204 без тела; занятая запись
// (есть дочерние заметки или другие ссылки) — 409 со списком ссылающихся.
// Запись — только для вошедшего владельца, см. handleDivisionCreate.
func handleNoteDelete(notes NoteService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		if err := notes.DeleteNote(r.Context(), pathID(r)); err != nil {
			writeError(w, err)

			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
