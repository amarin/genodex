package httpapi

import (
	"net/http"

	"github.com/amarin/genodex/internal/transport"
)

// handleSourceCreate — POST /api/sources: создаёт запись, отвечает 201 с
// созданной записью (id генерирует сценарий). Несуществующее repository_id —
// 422, по образцу handleArchiveCreate. Запись — только для вошедшего
// владельца, см. handleDivisionCreate.
func handleSourceCreate(sources SourceService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		var in transport.SourceCreate
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось разобрать тело: " + err.Error()})

			return
		}

		created, err := sources.CreateSource(r.Context(), in.Model())
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusCreated, transport.SourceFromModel(created))
	}
}

// handleSourceUpdate — PUT /api/sources/{id}: полная замена
// kind/title/author/date/reliability/repository_id/notes/private. Читает
// текущую версию, накладывает поля запроса (fetch-then-merge).
func handleSourceUpdate(sources SourceService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		id := pathID(r)

		var in transport.SourceUpdate
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось разобрать тело: " + err.Error()})

			return
		}

		cur, err := sources.GetSource(r.Context(), AccessFromContext(r.Context()), id)
		if err != nil {
			writeError(w, err)

			return
		}

		m := in.Model()
		cur.Kind = m.Kind
		cur.Title = m.Title
		cur.Author = m.Author
		cur.Date = m.Date
		cur.Reliability = m.Reliability
		cur.RepositoryID = m.RepositoryID
		cur.Notes = m.Notes
		cur.Private = m.Private

		if err := sources.UpdateSource(r.Context(), cur); err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.SourceFromModel(cur))
	}
}

// handleSourceDelete — DELETE /api/sources/{id}: 204 без тела; занятая
// запись — 409 со списком ссылающихся. Запись — только для вошедшего
// владельца, см. handleDivisionCreate.
func handleSourceDelete(sources SourceService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		if err := sources.DeleteSource(r.Context(), pathID(r)); err != nil {
			writeError(w, err)

			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
