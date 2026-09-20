package storage

import (
	"testing"
)

func savePerson(t *testing.T, s *Storage, id, surname string) {
	t.Helper()
	txt := []byte(`{"surname":"` + surname + `"}`)
	if err := s.Save("person", id, txt, []byte(Normalize(surname))); err != nil {
		t.Fatal(err)
	}
}

func TestStorageSaveGet(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	savePerson(t, s, "blohin", "Блохин")
	got, ok, err := s.Get("person", "blohin")
	if err != nil || !ok || string(got) != `{"surname":"Блохин"}` {
		t.Fatalf("Get: ok=%v data=%s err=%v", ok, got, err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestStoragePersistsOnReopen(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	savePerson(t, s, "blohin", "Блохин")
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	s2, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	got, ok, _ := s2.Get("person", "blohin")
	if !ok || string(got) != `{"surname":"Блохин"}` {
		t.Fatalf("after reopen: ok=%v data=%s", ok, got)
	}
}

func TestStorageDelete(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	savePerson(t, s, "blohin", "Блохин")
	if err := s.Delete("person", "blohin"); err != nil {
		t.Fatal(err)
	}
	_, ok, _ := s.Get("person", "blohin")
	if ok {
		t.Fatal("entity exists after Delete")
	}
}

func TestStorageSearch(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	savePerson(t, s, "blohin", "Семёнов")
	savePerson(t, s, "dorozhkin", "Дорожкин")
	ids, err := s.Search("person", Normalize("семен"))
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 1 || ids[0] != "blohin" {
		t.Fatalf("Search = %v, want [blohin]", ids)
	}
}
