package transport

import "github.com/amarin/genodex/internal/models"

// AttachmentCreate — тело POST /api/attachments и аргументы тула
// attachment_create. Идентификатор генерирует сценарий. NodeID обязателен;
// DocumentID — просто id, пустая строка — не задан. Сценарий проверяет
// существование обоих (если заданы).
type AttachmentCreate struct {
	Kind       string `json:"kind"`
	URI        string `json:"uri,omitempty"`
	Filename   string `json:"filename,omitempty"`
	MIME       string `json:"mime,omitempty"`
	Page       int    `json:"page,omitempty"`
	NodeID     string `json:"node_id"`
	DocumentID string `json:"document_id,omitempty"`
	Note       string `json:"note,omitempty"`
	Private    bool   `json:"private"`
}

// Model возвращает доменную запись с пустым ID.
func (a AttachmentCreate) Model() models.Attachment {
	var documentID *models.ID
	if a.DocumentID != "" {
		id := models.ID(a.DocumentID)
		documentID = &id
	}

	return models.Attachment{
		Kind:       models.AttachmentKind(a.Kind),
		URI:        a.URI,
		Filename:   a.Filename,
		MIME:       a.MIME,
		Page:       a.Page,
		NodeID:     models.ID(a.NodeID),
		DocumentID: documentID,
		Note:       a.Note,
		Private:    a.Private,
	}
}

// AttachmentUpdate — тело PUT /api/attachments/{id} и аргументы тула
// attachment_update: полная замена kind/uri/filename/mime/page/node_id/
// document_id/note/private.
type AttachmentUpdate struct {
	Kind       string `json:"kind"`
	URI        string `json:"uri,omitempty"`
	Filename   string `json:"filename,omitempty"`
	MIME       string `json:"mime,omitempty"`
	Page       int    `json:"page,omitempty"`
	NodeID     string `json:"node_id"`
	DocumentID string `json:"document_id,omitempty"`
	Note       string `json:"note,omitempty"`
	Private    bool   `json:"private"`
}

// Model возвращает доменную запись с пустым ID.
func (a AttachmentUpdate) Model() models.Attachment {
	var documentID *models.ID
	if a.DocumentID != "" {
		id := models.ID(a.DocumentID)
		documentID = &id
	}

	return models.Attachment{
		Kind:       models.AttachmentKind(a.Kind),
		URI:        a.URI,
		Filename:   a.Filename,
		MIME:       a.MIME,
		Page:       a.Page,
		NodeID:     models.ID(a.NodeID),
		DocumentID: documentID,
		Note:       a.Note,
		Private:    a.Private,
	}
}
