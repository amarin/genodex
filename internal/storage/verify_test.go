package storage

import (
	"path/filepath"
	"testing"
)

func TestVerifyHappyPath(t *testing.T) {
	dir := t.TempDir()
	s, _ := Open(dir)
	savePerson(t, s, "blohin", "Блохин")
	bkp := filepath.Join(t.TempDir(), "backup")
	if _, err := Backup(s, bkp); err != nil {
		t.Fatal(err)
	}
	res, err := Verify(s, bkp)
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK {
		t.Fatalf("Verify: %+v", res)
	}
	if res.ManifestMatches != true {
		t.Fatal("manifest должен совпадать")
	}
}

func TestVerifyDetectsCorruptionInSnapshot(t *testing.T) {
	dir := t.TempDir()
	s, _ := Open(dir)
	savePerson(t, s, "blohin", "Блохин")
	bkp := filepath.Join(t.TempDir(), "backup")
	if _, err := Backup(s, bkp); err != nil {
		t.Fatal(err)
	}
	// портим снапшот в бандле
	m, _ := ReadManifest(bkp)
	corrupt := m.SnapshotFile
	if err := writeGarbage(filepath.Join(bkp, corrupt), 512); err != nil {
		t.Fatal(err)
	}
	res, err := Verify(s, bkp)
	if err == nil && res.ManifestMatches {
		t.Fatal("подпорченный снапшот не должен проходить verify")
	}
}
