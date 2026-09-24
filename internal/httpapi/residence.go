package httpapi

import (
	"net/http"
	"net/url"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/transport"
)

// handleResidenceList — GET /api/residences?person_id=&place_id=&limit=&offset=.
// Оба фильтра необязательны и пересекаются, если заданы вместе. Замена
// search_residences — см. registerResidenceRoutes.
func handleResidenceList(residences ResidenceService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q, err := parseResidenceQuery(r.URL.Query())
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})

			return
		}

		list, err := residences.ListResidences(r.Context(), AccessFromContext(r.Context()), q)
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.ResidencesFromModels(list))
	}
}

// parseResidenceQuery разбирает параметры запроса: person_id/place_id необязательны.
func parseResidenceQuery(v url.Values) (models.ResidenceQuery, error) {
	var q models.ResidenceQuery

	if pid := v.Get("person_id"); pid != "" {
		id := models.ID(pid)
		q.PersonID = &id
	}

	if plid := v.Get("place_id"); plid != "" {
		id := models.ID(plid)
		q.PlaceID = &id
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

// handleResidenceGet — GET /api/residences/{id}.
func handleResidenceGet(residences ResidenceService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		res, err := residences.GetResidence(r.Context(), AccessFromContext(r.Context()), pathID(r))
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.ResidenceFromModel(res))
	}
}
