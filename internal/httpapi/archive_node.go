package httpapi

import (
	"errors"
	"net/http"
	"net/url"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/transport"
)

// errMissingArchiveID — archive_id обязателен для списка узлов дерева
// (у узла нет смысла вне архива, docs/data-model/entity-write.md §3.4).
var errMissingArchiveID = errors.New("параметр archive_id обязателен")

// handleArchiveNodeList — GET /api/archive-nodes?archive_id=&parent_id=&limit=&offset=.
// archive_id обязателен (у узла нет смысла вне архива) — отсутствующий или
// синтаксически неверный параметр — 400 до вызова сценария; неверное
// значение (несуществующий формат archive_id/parent_id, отрицательное
// окно) — 422 (*models.ValidationError из сценария).
func handleArchiveNodeList(archiveNodes ArchiveNodeService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q, err := parseArchiveNodeQuery(r.URL.Query())
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})

			return
		}

		list, err := archiveNodes.ListArchiveNodes(r.Context(), AccessFromContext(r.Context()), q)
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.ArchiveNodesFromModels(list))
	}
}

// parseArchiveNodeQuery разбирает параметры запроса; archive_id обязателен —
// отсутствующий это 400 (не 422 из сценария, чтобы не тратить обращение к
// сценарию на заведомо неполный запрос).
func parseArchiveNodeQuery(v url.Values) (models.ArchiveNodeQuery, error) {
	raw := v.Get("archive_id")
	if raw == "" {
		return models.ArchiveNodeQuery{}, errMissingArchiveID
	}

	q := models.ArchiveNodeQuery{ArchiveID: models.ID(raw)}

	if pid := v.Get("parent_id"); pid != "" {
		id := models.ID(pid)
		q.ParentID = &id
	}

	var err error

	if q.Page.Limit, err = intParam(v, "limit"); err != nil {
		return q, err
	}

	if q.Page.Offset, err = intParam(v, "offset"); err != nil {
		return q, err
	}

	return q, nil
}

// handleArchiveNodeSearch — GET /api/archive-nodes/search?q=&limit=&offset=.
// Глобальный поиск по всем архивам (не сужен по archive_id — сужение, если
// понадобится, веб-сторона делает сама, docs/data-model/entity-write.md §3.4).
func handleArchiveNodeSearch(archiveNodes ArchiveNodeService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		page, err := parsePage(q)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})

			return
		}

		list, err := archiveNodes.SearchArchiveNodes(r.Context(), AccessFromContext(r.Context()),
			models.SearchQuery{Text: q.Get("q"), Page: page})
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.ArchiveNodesFromModels(list))
	}
}

// handleArchiveNodeGet — GET /api/archive-nodes/{id}.
func handleArchiveNodeGet(archiveNodes ArchiveNodeService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		n, err := archiveNodes.GetArchiveNode(r.Context(), AccessFromContext(r.Context()), pathID(r))
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.ArchiveNodeFromModel(n))
	}
}
