package transport

import "github.com/amarin/genodex/internal/models"

// Attachment — контракт файлового вложения (GET /api/attachments, MCP-тул
// attachment_list). NodeID — обязательная строгая ссылка на архивный узел
// (просто id — существование проверяется в сценарии через generic-хранилище,
// читающее любую сущность по id независимо от готовности её
// orchestration-слоя; ArchiveNode/ArchiveDocument теперь имеют собственный
// CRUD-слой тоже). DocumentID — необязательная мягкая ссылка на архивный
// документ (ON DELETE SET NULL в схеме — при удалении документа поле
// обнуляется автоматически, но существование при создании/изменении
// сценарий всё равно проверяет, как и для NodeID). Sources у Attachment нет
// (в отличие от Repository/Church/Parish/Archive/Note).
type Attachment struct {
	ID         models.ID `json:"id"`
	Kind       string    `json:"kind"`
	URI        string    `json:"uri,omitempty"`
	Filename   string    `json:"filename,omitempty"`
	MIME       string    `json:"mime,omitempty"`
	Page       int       `json:"page,omitempty"`
	NodeID     string    `json:"node_id"`
	DocumentID string    `json:"document_id,omitempty"`
	Note       string    `json:"note,omitempty"`
	Private    bool      `json:"private"`
}

// AttachmentFromModel конвертирует запись в контракт.
func AttachmentFromModel(a models.Attachment) Attachment {
	var documentID string
	if a.DocumentID != nil {
		documentID = string(*a.DocumentID)
	}

	return Attachment{
		ID:         a.ID,
		Kind:       string(a.Kind),
		URI:        a.URI,
		Filename:   a.Filename,
		MIME:       a.MIME,
		Page:       a.Page,
		NodeID:     string(a.NodeID),
		DocumentID: documentID,
		Note:       a.Note,
		Private:    a.Private,
	}
}

// AttachmentsFromModels конвертирует список; пустой вход даёт пустой срез, а не nil.
func AttachmentsFromModels(as []models.Attachment) []Attachment {
	out := make([]Attachment, 0, len(as))
	for _, a := range as {
		out = append(out, AttachmentFromModel(a))
	}

	return out
}
