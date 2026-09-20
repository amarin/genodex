package storage

import (
	"path/filepath"
	"testing"
)

func TestBackupCreatesBundle(t *testing.T) {
	src := t.TempDir()
	s, err := Open(src)
	if err != nil {
		t.Fatal(err)
	}
	savePerson(t, s, "blohin", "Блохин")
	savePerson(t, s, "dorozhkin", "Дорожкин")
	dst := filepath.Join(t.TempDir(), "backup")
	m, err := Backup(s, dst)
	if err != nil {
		t.Fatal(err)
	}
	if m.SchemaVersion != schemaVersion {
		t.Fatalf("SchemaVersion = %d, want %d", m.SchemaVersion, schemaVersion)
	}
	if n := m.EntityCounts["persons"]; n != 2 {
		t.Fatalf("EntityCounts[persons] = %d, want 2", n)
	}
	if _, err := ReadManifest(dst); err != nil {
		t.Fatalf("ReadManifest: %v", err)
	}
}
