package models

import "testing"

func TestArchiveNodeFullForm(t *testing.T) {
	n := ArchiveNode{
		ID:        ID("n-1"),
		Type:      ArchiveNodeType("case"),
		ArchiveID: ID("a-1"),
		Label:     "дело №264",
		Parish:    &TextRef{Text: "Николаевская ц."},
	}
	if n.EntityType() != TypeArchiveNode {
		t.Fatalf("EntityType() = %q", n.EntityType())
	}
	if n.ArchiveID != ID("a-1") {
		t.Fatalf("strict link lost")
	}
}

func TestRepositoryFullForm(t *testing.T) {
	r := Repository{
		ID:      ID("r-1"),
		Name:    "ГАВО",
		Type:    RepositoryTypeArchive,
		Address: "г. Владимир",
		URLs:    []TextRef{{Text: "https://example.gov.ru/funds"}},
		Private: true,
	}
	if r.EntityType() != TypeRepository {
		t.Fatalf("EntityType() = %q", r.EntityType())
	}
	if len(r.URLs) != 1 || r.Private != true {
		t.Fatalf("urls/private не прочитаны")
	}
}
