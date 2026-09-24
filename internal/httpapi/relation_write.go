package httpapi

import (
	"net/http"

	"github.com/amarin/genodex/internal/transport"
)

// handleRelationCreate — POST /api/relations: создаёт ребро, отвечает 201 с
// созданной записью (id генерирует сценарий). Несуществующий person_a/
// person_b — 422 (см. writeError). Запись — только для вошедшего владельца,
// см. handleFamilyCreate.
func handleRelationCreate(relations RelationService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		var in transport.RelationCreate
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось разобрать тело: " + err.Error()})

			return
		}

		created, err := relations.CreateRelation(r.Context(), in.Model())
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusCreated, transport.RelationFromModel(created))
	}
}

// handleRelationUpdate — PUT /api/relations/{id}: полная замена kind/
// rel_type/person_a/person_b/since/until/sources/notes/private
// (fetch-then-merge, docs/data-model/entity-write.md §3).
func handleRelationUpdate(relations RelationService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		id := pathID(r)

		var in transport.RelationUpdate
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось разобрать тело: " + err.Error()})

			return
		}

		cur, err := relations.GetRelation(r.Context(), AccessFromContext(r.Context()), id)
		if err != nil {
			writeError(w, err)

			return
		}

		m := in.Model()
		cur.Kind = m.Kind
		cur.RelType = m.RelType
		cur.PersonA = m.PersonA
		cur.PersonB = m.PersonB
		cur.Since = m.Since
		cur.Until = m.Until
		cur.Sources = m.Sources
		cur.Notes = m.Notes
		cur.Private = m.Private

		if err := relations.UpdateRelation(r.Context(), cur); err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.RelationFromModel(cur))
	}
}

// handleRelationDelete — DELETE /api/relations/{id}: 204 без тела; занятая
// запись — 409 со списком ссылающихся. Запись — только для вошедшего
// владельца, см. handleFamilyCreate.
func handleRelationDelete(relations RelationService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		if err := relations.DeleteRelation(r.Context(), pathID(r)); err != nil {
			writeError(w, err)

			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
