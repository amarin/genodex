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
	seq := s.LastSeq()
	dst := filepath.Join(t.TempDir(), "backup")
	m, err := Backup(s, dst)
	if err != nil {
		t.Fatal(err)
	}
	if m.AppliedSeq != seq {
		t.Fatalf("AppliedSeq = %d, want %d", m.AppliedSeq, seq)
	}
	if m.SchemaVersion != schemaVersion {
		t.Fatalf("SchemaVersion = %d, want %d", m.SchemaVersion, schemaVersion)
	}
	if _, err := ReadManifest(dst); err != nil {
		t.Fatalf("ReadManifest: %v", err)
	}
}
