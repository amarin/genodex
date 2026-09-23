package httpapi

import (
	"net/http"

	"github.com/amarin/genodex/internal/transport"
)

// handleArchiveCreate — POST /api/archives: создаёт запись, отвечает 201 с
// созданной записью (id генерирует сценарий). Несуществующее repository_id —
// 422 (см. writeError, та же механика, что и parent_id у делений). Запись —
// только для вошедшего владельца, см. handleDivisionCreate.
func handleArchiveCreate(archives ArchiveService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		var in transport.ArchiveCreate
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось разобрать тело: " + err.Error()})

			return
		}

		created, err := archives.CreateArchive(r.Context(), in.Model())
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusCreated, transport.ArchiveFromModel(created))
	}
}

// handleArchiveUpdate — PUT /api/archives/{id}: полная замена
// name/system/repository_id/notes/private. Читает текущую версию, накладывает
// поля запроса (fetch-then-merge). Sources не в DTO — read-only в v1.
func handleArchiveUpdate(archives ArchiveService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		id := pathID(r)

		var in transport.ArchiveUpdate
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось разобрать тело: " + err.Error()})

			return
		}

		cur, err := archives.GetArchive(r.Context(), AccessFromContext(r.Context()), id)
		if err != nil {
			writeError(w, err)

			return
		}

		m := in.Model()
		cur.Name = m.Name
		cur.System = m.System
		cur.RepositoryID = m.RepositoryID
		cur.Notes = m.Notes
		cur.Private = m.Private

		if err := archives.UpdateArchive(r.Context(), cur); err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.ArchiveFromModel(cur))
	}
}

// handleArchiveDelete — DELETE /api/archives/{id}: 204 без тела; занятая
// запись — 409 со списком ссылающихся. Запись — только для вошедшего
// владельца, см. handleDivisionCreate.
func handleArchiveDelete(archives ArchiveService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		if err := archives.DeleteArchive(r.Context(), pathID(r)); err != nil {
			writeError(w, err)

			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
