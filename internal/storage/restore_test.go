package storage

import (
	"os"
	"path/filepath"
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
