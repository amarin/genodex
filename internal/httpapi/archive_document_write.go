package httpapi

import (
	"net/http"

	"github.com/amarin/genodex/internal/transport"
)

// handleArchiveDocumentCreate — POST /api/archive-documents: создаёт
// документ, отвечает 201 с созданным документом (id генерирует сценарий).
// Несуществующий unit_id — 422 (см. writeError). Запись — только для
// вошедшего владельца, см. handleArchiveCreate.
func handleArchiveDocumentCreate(archiveDocuments ArchiveDocumentService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		var in transport.ArchiveDocumentCreate
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось разобрать тело: " + err.Error()})

			return
		}

		created, err := archiveDocuments.CreateArchiveDocument(r.Context(), in.Model())
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusCreated, transport.ArchiveDocumentFromModel(created))
	}
}

// handleArchiveDocumentUpdate — PUT /api/archive-documents/{id}: полная
// замена всех полей документа (fetch-then-merge, как у ArchiveNode/Archive).
func handleArchiveDocumentUpdate(archiveDocuments ArchiveDocumentService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		id := pathID(r)

		var in transport.ArchiveDocumentUpdate
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось разобрать тело: " + err.Error()})

			return
		}

		cur, err := archiveDocuments.GetArchiveDocument(r.Context(), AccessFromContext(r.Context()), id)
		if err != nil {
			writeError(w, err)

			return
		}

		m := in.Model()
		cur.UnitID = m.UnitID
		cur.Title = m.Title
		cur.Kind = m.Kind
		cur.Since = m.Since
		cur.Until = m.Until
		cur.Parish = m.Parish
		cur.Settlements = m.Settlements
		cur.Notes = m.Notes
		cur.Sources = m.Sources
		cur.Private = m.Private

		if err := archiveDocuments.UpdateArchiveDocument(r.Context(), cur); err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.ArchiveDocumentFromModel(cur))
	}
}

// handleArchiveDocumentDelete — DELETE /api/archive-documents/{id}: 204 без
// тела; занятый документ — 409 со списком ссылающихся.
func handleArchiveDocumentDelete(archiveDocuments ArchiveDocumentService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		if err := archiveDocuments.DeleteArchiveDocument(r.Context(), pathID(r)); err != nil {
			writeError(w, err)

			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
