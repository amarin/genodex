package httpapi

import (
	"net/http"
	"net/url"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/transport"
)

// handleRelationList — GET /api/relations?person_id=&limit=&offset=.
// person_id — необязательный фильтр (ребро проходит, если совпадает с
// person_a ИЛИ person_b); без него список плоский. Замена search_relations —
// см. registerRelationRoutes.
func handleRelationList(relations RelationService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q, err := parseRelationQuery(r.URL.Query())
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})

			return
		}

		list, err := relations.ListRelations(r.Context(), AccessFromContext(r.Context()), q)
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.RelationsFromModels(list))
	}
}

// parseRelationQuery разбирает параметры запроса: person_id необязателен.
func parseRelationQuery(v url.Values) (models.RelationQuery, error) {
	var q models.RelationQuery

	if pid := v.Get("person_id"); pid != "" {
		id := models.ID(pid)
		q.PersonID = &id
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

// handleRelationGet — GET /api/relations/{id}.
func handleRelationGet(relations RelationService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rel, err := relations.GetRelation(r.Context(), AccessFromContext(r.Context()), pathID(r))
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.RelationFromModel(rel))
	}
}
