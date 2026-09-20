package models

// NoteKind — тип заметки как самостоятельной сущности (решение #22).
// Значения свободные и расширяемые.
type NoteKind string

const (
	NoteKindNote    NoteKind = "note"
	NoteKindArticle NoteKind = "article"
	NoteKindBook    NoteKind = "book"
	NoteKindChapter NoteKind = "chapter"
)
