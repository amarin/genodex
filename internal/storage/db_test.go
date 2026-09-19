package storage

import (
	"path/filepath"
	"testing"
)

func openTestDB(t *testing.T) *DB {
	t.Helper()
	db, err := OpenDB(filepath.Join(t.TempDir(), "genealogy.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestDBUpsertGetListDelete(t *testing.T) {
	db := openTestDB(t)
	if err := db.Upsert("person", "blohin", []byte(`{"surname":"Блохин"}`), []byte(Normalize("Блохин"))); err != nil {
		t.Fatal(err)
	}
	got, ok, err := db.Get("person", "blohin")
	if err != nil || !ok {
		t.Fatalf("Get: ok=%v err=%v", ok, err)
	}
	if string(got) != `{"surname":"Блохин"}` {
		t.Fatalf("data = %s", got)
	}
	if n, _ := db.Count("person"); n != 1 {
		t.Fatalf("Count = %d, want 1", n)
	}
	if _, err := db.List("person"); err != nil {
		t.Fatal(err)
	}
	if err := db.Delete("person", "blohin"); err != nil {
		t.Fatal(err)
	}
	_, ok, _ = db.Get("person", "blohin")
	if ok {
		t.Fatal("entity exists after Delete")
	}
}

func TestDBSearchNormalized(t *testing.T) {
	db := openTestDB(t)
	_ = db.Upsert("person", "blohin", []byte(`{}`), []byte(Normalize("Семёнов Иван")))
	_ = db.Upsert("person", "dorozhkin", []byte(`{}`), []byte(Normalize("Дорожкин Пётр")))
	ids, err := db.Search("person", Normalize("семен"))
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 1 || ids[0] != "blohin" {
		t.Fatalf("Search = %v, want [blohin]", ids)
	}
}

func TestDBAppliedSeq(t *testing.T) {
	db := openTestDB(t)
	if seq, _ := db.AppliedSeq(); seq != 0 {
		t.Fatalf("initial AppliedSeq = %d", seq)
	}
	if err := db.SetAppliedSeq(42); err != nil {
		t.Fatal(err)
	}
	if seq, _ := db.AppliedSeq(); seq != 42 {
		t.Fatalf("AppliedSeq = %d, want 42", seq)
	}
}

func TestDBIntegrityAndVacuumInto(t *testing.T) {
	db := openTestDB(t)
	_ = db.Upsert("person", "blohin", []byte(`{}`), []byte("x"))
	if err := db.IntegrityCheck(); err != nil {
		t.Fatalf("IntegrityCheck: %v", err)
	}
	out := filepath.Join(t.TempDir(), "snapshot.db")
	if err := db.VacuumInto(out); err != nil {
		t.Fatalf("VacuumInto: %v", err)
	}
}
