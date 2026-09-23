package transport

import "github.com/amarin/genodex/internal/models"

// Note — контракт заметки как самостоятельной сущности (markdown-текст с
// иерархией «книга → главы», GET /api/notes, MCP-тул note_list). ParentID —
// просто id родительской заметки (не TextRef — строгая self-ref ссылка, как
// Archive.RepositoryID; пустая строка — без родителя). Sources редактируется
// с подпроекта 5 (см. internal/transport/source_link.go).
type Note struct {
	ID       models.ID    `json:"id"`
	Kind     string       `json:"kind"`
	Title    string       `json:"title,omitempty"`
	Text     string       `json:"text,omitempty"`
	ParentID string       `json:"parent_id,omitempty"`
	Sources  []SourceLink `json:"sources"`
	Private  bool         `json:"private"`
}

// NoteFromModel конвертирует запись в контракт.
func NoteFromModel(n models.Note) Note {
	var parentID string
	if n.ParentID != nil {
		parentID = string(*n.ParentID)
	}

	return Note{
		ID:       n.ID,
		Kind:     string(n.Kind),
		Title:    n.Title,
		Text:     n.Text,
		ParentID: parentID,
		Sources:  SourceLinksFromModel(n.Sources),
		Private:  n.Private,
	}
}

// NotesFromModels конвертирует список; пустой вход даёт пустой срез, а не nil.
func NotesFromModels(ns []models.Note) []Note {
	out := make([]Note, 0, len(ns))
	for _, n := range ns {
		out = append(out, NoteFromModel(n))
	}

	return out
}
