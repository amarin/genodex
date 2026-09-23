package httpapi

import (
	"net/http"

	"github.com/amarin/genodex/internal/transport"
)

// handleRepositoryCreate — POST /api/repositories: создаёт запись, отвечает
// 201 с созданной записью (id генерирует сценарий). Запись — только для
// вошедшего владельца, см. handleDivisionCreate.
func handleRepositoryCreate(repositories RepositoryService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		var in transport.RepositoryCreate
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось разобрать тело: " + err.Error()})

			return
		}

		created, err := repositories.CreateRepository(r.Context(), in.Model())
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusCreated, transport.RepositoryFromModel(created))
	}
}

// handleRepositoryUpdate — PUT /api/repositories/{id}: полная замена
// name/type/address/urls/notes/private. Читает текущую версию, накладывает
// поля запроса (fetch-then-merge, docs/data-model/entity-write.md §3).
// Sources не в DTO — read-only в v1 (internal/transport/source_link.go),
// текущее значение cur.Sources не трогается, остаётся как было.
func handleRepositoryUpdate(repositories RepositoryService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		id := pathID(r)

		var in transport.RepositoryUpdate
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось разобрать тело: " + err.Error()})

			return
		}

		cur, err := repositories.GetRepository(r.Context(), AccessFromContext(r.Context()), id)
		if err != nil {
			writeError(w, err)

			return
		}

		m := in.Model()
		cur.Name = m.Name
		cur.Type = m.Type
		cur.Address = m.Address
		cur.URLs = m.URLs
		cur.Notes = m.Notes
		cur.Private = m.Private

		if err := repositories.UpdateRepository(r.Context(), cur); err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.RepositoryFromModel(cur))
	}
}

// handleRepositoryDelete — DELETE /api/repositories/{id}: 204 без тела;
// занятая запись — 409 со списком ссылающихся. Запись — только для вошедшего
// владельца, см. handleDivisionCreate.
func handleRepositoryDelete(repositories RepositoryService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		if err := repositories.DeleteRepository(r.Context(), pathID(r)); err != nil {
			writeError(w, err)

			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
