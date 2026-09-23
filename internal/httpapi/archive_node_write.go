package httpapi

import (
	"net/http"

	"github.com/amarin/genodex/internal/transport"
)

// handleArchiveNodeCreate — POST /api/archive-nodes: создаёт узел, отвечает
// 201 с созданным узлом (id генерирует сценарий). Несуществующий archive_id
// или parent_id, либо parent_id из другого архива — 422 (см. writeError).
// Запись — только для вошедшего владельца, см. handleArchiveCreate.
func handleArchiveNodeCreate(archiveNodes ArchiveNodeService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		var in transport.ArchiveNodeCreate
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось разобрать тело: " + err.Error()})

			return
		}

		created, err := archiveNodes.CreateArchiveNode(r.Context(), in.Model())
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusCreated, transport.ArchiveNodeFromModel(created))
	}
}

// handleArchiveNodeUpdate — PUT /api/archive-nodes/{id}: полная замена всех
// полей узла (fetch-then-merge: читает текущую версию, накладывает поля
// запроса, сохраняет — тот же приём, что и у Archive, даже при DTO,
// покрывающем сейчас все поля модели, docs/data-model/entity-write.md §3).
func handleArchiveNodeUpdate(archiveNodes ArchiveNodeService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		id := pathID(r)

		var in transport.ArchiveNodeUpdate
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось разобрать тело: " + err.Error()})

			return
		}

		cur, err := archiveNodes.GetArchiveNode(r.Context(), AccessFromContext(r.Context()), id)
		if err != nil {
			writeError(w, err)

			return
		}

		m := in.Model()
		cur.Type = m.Type
		cur.ArchiveID = m.ArchiveID
		cur.ParentID = m.ParentID
		cur.Label = m.Label
		cur.Name = m.Name
		cur.Since = m.Since
		cur.Until = m.Until
		cur.Parish = m.Parish
		cur.Settlements = m.Settlements
		cur.Notes = m.Notes
		cur.Sources = m.Sources
		cur.Private = m.Private

		if err := archiveNodes.UpdateArchiveNode(r.Context(), cur); err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.ArchiveNodeFromModel(cur))
	}
}

// handleArchiveNodeDelete — DELETE /api/archive-nodes/{id}: 204 без тела;
// занятый узел (дочерние узлы или документы) — 409 со списком ссылающихся.
// Запись — только для вошедшего владельца, см. handleArchiveCreate.
func handleArchiveNodeDelete(archiveNodes ArchiveNodeService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		if err := archiveNodes.DeleteArchiveNode(r.Context(), pathID(r)); err != nil {
			writeError(w, err)

			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
