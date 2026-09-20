package models

import "testing"

func TestNote(t *testing.T) {
	parent := ID("n-book")
	n := Note{
		ID:       ID("n-1"),
		Kind:     NoteKindChapter,
		Title:    "Глава 1",
		Text:     "# Глава 1",
		ParentID: &parent,
	}
	if n.EntityType() != TypeNote {
		t.Fatalf("EntityType() = %q, want %q", n.EntityType(), TypeNote)
	}
	if n.ParentID == nil || *n.ParentID != parent {
		t.Fatalf("ParentID не установлен через *ID")
	}
	if n.Kind != NoteKindChapter {
		t.Fatalf("Kind = %q, want %q", n.Kind, NoteKindChapter)
	}
}
