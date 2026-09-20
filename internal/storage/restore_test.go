package storage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRestoreRoundTrip(t *testing.T) {
	srcData := t.TempDir()
	s, err := Open(srcData)
	if err != nil {
		t.Fatal(err)
	}
	savePerson(t, s, "blohin", "Блохин")
	savePerson(t, s, "dorozhkin", "Дорожкин")
	bkpDir := filepath.Join(t.TempDir(), "backup")
	if _, err := Backup(s, bkpDir); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	restored := filepath.Join(t.TempDir(), "restored")
	rs, err := Restore(bkpDir, restored)
	if err != nil {
		t.Fatalf("Restore: %v", err)
	}
	defer rs.Close()
	got, ok := personSurname(t, rs, "blohin")
	if !ok || got != "Блохин" {
		t.Fatalf("restored blohin: ok=%v surname=%q", ok, got)
	}
	n, _ := rs.DB().Count("persons")
	if n != 2 {
		t.Fatalf("restored Count = %d, want 2", n)
	}
}

func TestRestoreRefusesNonEmptyTarget(t *testing.T) {
	srcData := t.TempDir()
	s, _ := Open(srcData)
	savePerson(t, s, "blohin", "Блохин")
	bkpDir := filepath.Join(t.TempDir(), "backup")
	if _, err := Backup(s, bkpDir); err != nil {
		t.Fatal(err)
	}
	s.Close()

	evil := filepath.Join(t.TempDir(), "occupied")
	if err := os.MkdirAll(filepath.Join(evil, "db"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evil, "db", "genodex.db"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Restore(bkpDir, evil); err == nil {
		t.Fatal("Restore должен отказаться писать в непустую директорию")
	}
}

// makeBundle делает бандл с двумя персонами и даёт правку манифеста.
func makeBundle(t *testing.T, mutate func(m *Manifest)) string {
	t.Helper()
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	savePerson(t, s, "blohin", "Блохин")
	bkpDir := filepath.Join(t.TempDir(), "backup")
	m, err := Backup(s, bkpDir)
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	mutate(&m)
	if err := writeManifest(bkpDir, m); err != nil {
		t.Fatal(err)
	}
	return bkpDir
}

func TestRestoreRejectsSchemaVersionMismatch(t *testing.T) {
	bkpDir := makeBundle(t, func(m *Manifest) { m.SchemaVersion = schemaVersion + 7 })
	target := filepath.Join(t.TempDir(), "restored")
	_, err := Restore(bkpDir, target)
	if err == nil {
		t.Fatal("Restore должен отказать при другой версии схемы")
	}
	if !strings.Contains(err.Error(), "версия схемы бандла") {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(target, "db", "genodex.db")); !os.IsNotExist(statErr) {
		t.Fatalf("db/genodex.db не должен появиться: %v", statErr)
	}
}

func TestRestoreCleansUpTargetOnFailure(t *testing.T) {
	bkpDir := makeBundle(t, func(m *Manifest) { m.EntityCounts = map[string]int{"persons": 99} })
	target := filepath.Join(t.TempDir(), "restored")
	if _, err := Restore(bkpDir, target); err == nil {
		t.Fatal("Restore должен упасть на несовпадении счётчиков")
	}
	for _, suf := range []string{"", "-wal", "-shm"} {
		if _, err := os.Stat(filepath.Join(target, "db", "genodex.db"+suf)); !os.IsNotExist(err) {
			t.Fatalf("db/genodex.db%s остался после неудачи: %v", suf, err)
		}
	}
	// повторная попытка с исправным манифестом не должна упираться в «target already contains»
	good := makeBundle(t, func(m *Manifest) {})
	rs, err := Restore(good, target)
	if err != nil {
		t.Fatalf("повторный Restore: %v", err)
	}
	rs.Close()
}

func TestRestoreCleansUpTargetOnUnknownTable(t *testing.T) {
	bkpDir := makeBundle(t, func(m *Manifest) { m.EntityCounts = map[string]int{"no_such_table": 1} })
	target := filepath.Join(t.TempDir(), "restored")
	if _, err := Restore(bkpDir, target); err == nil {
		t.Fatal("Restore должен упасть на неизвестной таблице")
	}
	if _, err := os.Stat(filepath.Join(target, "db", "genodex.db")); !os.IsNotExist(err) {
		t.Fatalf("db/genodex.db остался после неудачи: %v", err)
	}
}
