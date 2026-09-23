package httpapi

import (
	"net/http"

	"github.com/amarin/genodex/internal/transport"
)

// handleAttachmentCreate — POST /api/attachments: создаёт запись, отвечает
// 201 с созданной записью (id генерирует сценарий). Несуществующий node_id
// или document_id — 422. Запись — только для вошедшего владельца, см.
// handleDivisionCreate.
func handleAttachmentCreate(attachments AttachmentService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		var in transport.AttachmentCreate
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось разобрать тело: " + err.Error()})

			return
		}

		created, err := attachments.CreateAttachment(r.Context(), in.Model())
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusCreated, transport.AttachmentFromModel(created))
	}
}

// handleAttachmentUpdate — PUT /api/attachments/{id}: полная замена
// kind/uri/filename/mime/page/node_id/document_id/note/private. Читает
// текущую версию, накладывает поля запроса (fetch-then-merge).
func handleAttachmentUpdate(attachments AttachmentService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		id := pathID(r)

		var in transport.AttachmentUpdate
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось разобрать тело: " + err.Error()})

			return
		}

		cur, err := attachments.GetAttachment(r.Context(), AccessFromContext(r.Context()), id)
		if err != nil {
			writeError(w, err)

			return
		}

		m := in.Model()
		cur.Kind = m.Kind
		cur.URI = m.URI
		cur.Filename = m.Filename
		cur.MIME = m.MIME
		cur.Page = m.Page
		cur.NodeID = m.NodeID
		cur.DocumentID = m.DocumentID
		cur.Note = m.Note
		cur.Private = m.Private

		if err := attachments.UpdateAttachment(r.Context(), cur); err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.AttachmentFromModel(cur))
	}
}

// handleAttachmentDelete — DELETE /api/attachments/{id}: 204 без тела;
// занятая запись — 409 со списком ссылающихся. Запись — только для
// вошедшего владельца, см. handleDivisionCreate.
func handleAttachmentDelete(attachments AttachmentService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		if err := attachments.DeleteAttachment(r.Context(), pathID(r)); err != nil {
			writeError(w, err)

			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
