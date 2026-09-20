package storage

import (
	"testing"
)

// savePerson кладёт персону в колоночную схему: строка persons, основное имя
// в person_names (через text_refs) и термин в search_index.
// Полноценный маппинг сущностей — задача internal/store/sqlstore.
func savePerson(t *testing.T, s *Storage, id, surname string) {
	t.Helper()
	db := s.DB()
	if _, err := db.Exec(
		`INSERT OR REPLACE INTO persons(id, gender, private) VALUES (?, 'unknown', 0)`, id,
	); err != nil {
		t.Fatal(err)
	}
	res, err := db.Exec(`INSERT INTO text_refs(text) VALUES (?)`, surname)
	if err != nil {
		t.Fatal(err)
	}
	ref, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(
		`INSERT INTO person_names(person_id, type, surname_id, given_id, patronymic_id)
		 VALUES (?, 'main', ?, ?, ?)`, id, ref, ref, ref,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(
		`INSERT OR REPLACE INTO search_index(entity_table, entity_id, field, term)
		 VALUES ('persons', ?, 'surname', ?)`, id, Normalize(surname),
	); err != nil {
		t.Fatal(err)
	}
}

// personSurname читает каноническую фамилию основного имени персоны.
func personSurname(t *testing.T, s *Storage, id string) (string, bool) {
	t.Helper()
	var surname string
	err := s.DB().QueryRow(
		`SELECT tr.text FROM person_names pn
		 JOIN text_refs tr ON tr.id = pn.surname_id
		 WHERE pn.person_id = ?`, id,
	).Scan(&surname)
	if err != nil {
		return "", false
	}
	return surname, true
}

func TestStorageSaveGet(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	savePerson(t, s, "blohin", "Блохин")
	got, ok := personSurname(t, s, "blohin")
	if !ok || got != "Блохин" {
		t.Fatalf("personSurname: ok=%v surname=%q", ok, got)
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
	got, ok := personSurname(t, s2, "blohin")
	if !ok || got != "Блохин" {
		t.Fatalf("after reopen: ok=%v surname=%q", ok, got)
	}
}

func TestStorageDeleteCascades(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	savePerson(t, s, "blohin", "Блохин")
	if _, err := s.DB().Exec(`DELETE FROM persons WHERE id = 'blohin'`); err != nil {
		t.Fatal(err)
	}
	if _, ok := personSurname(t, s, "blohin"); ok {
		t.Fatal("person_names остались после удаления персоны")
	}
	n, err := s.DB().Count("persons")
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("Count(persons) = %d, want 0", n)
	}
}
