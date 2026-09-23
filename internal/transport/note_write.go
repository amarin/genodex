package transport

import "github.com/amarin/genodex/internal/models"

// NoteCreate — тело POST /api/notes и аргументы тула note_create.
// Идентификатор генерирует сценарий. ParentID — просто id (пустая строка —
// без родителя); сценарий проверяет существование и отсутствие циклов, по
// образцу create_division.
type NoteCreate struct {
	Kind     string `json:"kind"`
	Title    string `json:"title,omitempty"`
	Text     string `json:"text,omitempty"`
	ParentID string `json:"parent_id,omitempty"`
	Private  bool   `json:"private"`
}

// Model возвращает доменную запись с пустым ID.
func (n NoteCreate) Model() models.Note {
	var parentID *models.ID
	if n.ParentID != "" {
		id := models.ID(n.ParentID)
		parentID = &id
	}

	return models.Note{
		Kind:     models.NoteKind(n.Kind),
		Title:    n.Title,
		Text:     n.Text,
		ParentID: parentID,
		Private:  n.Private,
	}
}

// NoteUpdate — тело PUT /api/notes/{id} и аргументы тула note_update:
// полная замена kind/title/text/parent_id/private.
type NoteUpdate struct {
	Kind     string `json:"kind"`
	Title    string `json:"title,omitempty"`
	Text     string `json:"text,omitempty"`
	ParentID string `json:"parent_id,omitempty"`
	Private  bool   `json:"private"`
}

// Model возвращает доменную запись с пустым ID.
func (n NoteUpdate) Model() models.Note {
	var parentID *models.ID
	if n.ParentID != "" {
		id := models.ID(n.ParentID)
		parentID = &id
	}

	return models.Note{
		Kind:     models.NoteKind(n.Kind),
		Title:    n.Title,
		Text:     n.Text,
		ParentID: parentID,
		Private:  n.Private,
	}
}
