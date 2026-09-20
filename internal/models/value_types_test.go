package models

import "testing"

// Ссылки value-типов — строгие ID/Type, а не произвольные строки.
func TestValueTypesUseStrictRefs(t *testing.T) {
	r := TextRef{Text: "Давыдово", Ref: ID("ad-1"), Type: TypeAdministrativeDivision}
	if r.Ref != ID("ad-1") || r.Type != TypeAdministrativeDivision {
		t.Fatalf("TextRef: %+v", r)
	}
	a := &ArchiveAnchor{NodeID: ID("n-1"), DocumentID: ID("d-1"), Page: 3}
	if a.Kind() != AnchorArchive {
		t.Fatalf("Kind() = %q, want %q", a.Kind(), AnchorArchive)
	}
	f := &FileAnchor{AttachmentID: ID("att-1")}
	if f.Kind() != AnchorFile {
		t.Fatalf("Kind() = %q, want %q", f.Kind(), AnchorFile)
	}
}
