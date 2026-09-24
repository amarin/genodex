package httpapi

import (
	"net/http"

	"github.com/amarin/genodex/internal/transport"
)

// handleResidenceCreate — POST /api/residences: создаёт запись, отвечает
// 201 с созданной записью (id генерирует сценарий). Несуществующий
// person_id/place_id — 422 (см. writeError). Запись — только для вошедшего
// владельца, см. handleFamilyCreate.
func handleResidenceCreate(residences ResidenceService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		var in transport.ResidenceCreate
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось разобрать тело: " + err.Error()})

			return
		}

		created, err := residences.CreateResidence(r.Context(), in.Model())
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusCreated, transport.ResidenceFromModel(created))
	}
}

// handleResidenceUpdate — PUT /api/residences/{id}: полная замена
// person_id/place_id/since/until/sources/note/private (fetch-then-merge,
// docs/data-model/entity-write.md §3).
func handleResidenceUpdate(residences ResidenceService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		id := pathID(r)

		var in transport.ResidenceUpdate
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось разобрать тело: " + err.Error()})

			return
		}

		cur, err := residences.GetResidence(r.Context(), AccessFromContext(r.Context()), id)
		if err != nil {
			writeError(w, err)

			return
		}

		m := in.Model()
		cur.PersonID = m.PersonID
		cur.PlaceID = m.PlaceID
		cur.Since = m.Since
		cur.Until = m.Until
		cur.Sources = m.Sources
		cur.Note = m.Note
		cur.Private = m.Private

		if err := residences.UpdateResidence(r.Context(), cur); err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.ResidenceFromModel(cur))
	}
}

// handleResidenceDelete — DELETE /api/residences/{id}: 204 без тела; занятая
// запись — 409 со списком ссылающихся. Запись — только для вошедшего
// владельца, см. handleFamilyCreate.
func handleResidenceDelete(residences ResidenceService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		if err := residences.DeleteResidence(r.Context(), pathID(r)); err != nil {
			writeError(w, err)

			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
