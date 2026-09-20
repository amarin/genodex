package models

// Note — заметка как самостоятельная сущность (markdown-текст с иерархией «книга → главы»).
type Note struct {
	ID       ID
	Kind     NoteKind
	Title    string
	Text     string
	ParentID *ID
	Sources  []SourceLink
	Private  bool
}

// EntityType возвращает тип сущности.
func (n *Note) EntityType() Type { return TypeNote }
