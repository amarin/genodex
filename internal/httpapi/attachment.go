package httpapi

import (
	"net/http"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/transport"
)

// handleAttachmentList — GET /api/attachments?limit=&offset=. Чтение открыто
// анонимному посетителю (приватные вложения скрыты — см. handleAttachmentGet).
func handleAttachmentList(attachments AttachmentService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page, err := parsePage(r.URL.Query())
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})

			return
		}

		list, err := attachments.ListAttachments(r.Context(), AccessFromContext(r.Context()), page)
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.AttachmentsFromModels(list))
	}
}

// handleAttachmentSearch — GET /api/attachments/search?q=&limit=&offset=.
func handleAttachmentSearch(attachments AttachmentService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		page, err := parsePage(q)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})

			return
		}

		list, err := attachments.SearchAttachments(r.Context(), AccessFromContext(r.Context()),
			models.SearchQuery{Text: q.Get("q"), Page: page})
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.AttachmentsFromModels(list))
	}
}

// handleAttachmentGet — GET /api/attachments/{id}. Приватное вложение для
// анонимного или не-владельца — 404.
func handleAttachmentGet(attachments AttachmentService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		a, err := attachments.GetAttachment(r.Context(), AccessFromContext(r.Context()), pathID(r))
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.AttachmentFromModel(a))
	}
}
