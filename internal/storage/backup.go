package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

const manifestFile = "manifest.json"

// Manifest описывает бандл бэкапа.
type Manifest struct {
	SchemaVersion int            `json:"schema_version"`
	CreatedAt     string         `json:"created_at"`
	AppliedSeq    uint64         `json:"applied_seq"`
	SnapshotFile  string         `json:"snapshot_file"`
	SnapshotSHA   string         `json:"snapshot_sha256"`
	JournalFile   string         `json:"journal_file"`
	JournalSHA    string         `json:"journal_sha256"`
	EntityCounts  map[string]int `json:"entity_counts"`
}

// Backup создаёт в toDir консистентный бандл: снапшот (VACUUM INTO), копию
// журнала и манифест, который пишется последним. Порядок гарантирует: если
// манифест прочитается — значит и снапшот, и журнал уже на месте и целы.
func Backup(s *Storage, toDir string) (Manifest, error) {
	if err := os.MkdirAll(toDir, 0o755); err != nil {
		return Manifest{}, err
	}
	ts := time.Now().UTC().Format("20060102T150405Z")
	snapFile := "snapshot-" + ts + ".db"
	snapPath := filepath.Join(toDir, snapFile)
	if err := s.db.VacuumInto(snapPath); err != nil {
		return Manifest{}, fmt.Errorf("vacuum: %w", err)
	}
	snapSHA, err := fileSHA(snapPath)
	if err != nil {
		return Manifest{}, err
	}

	jrnFile := "journal-" + ts + ".jsonl"
	jrnPath := filepath.Join(toDir, jrnFile)
	if err := s.copyJournal(jrnPath); err != nil {
		return Manifest{}, err
	}
	jrnSHA, err := fileSHA(jrnPath)
	if err != nil {
		return Manifest{}, err
	}

	counts := map[string]int{}
	applied, err := s.db.AppliedSeq()
	if err != nil {
		return Manifest{}, err
	}
	for _, et := range []string{
		"person", "settlement", "church", "parish", "governorate", "district",
		"volost", "archive", "fund", "inventory", "case", "event", "marriage", "source",
	} {
		n, err := s.db.Count(et)
		if err != nil {
			return Manifest{}, err
		}
		if n > 0 {
			counts[et] = n
		}
	}

	m := Manifest{
		SchemaVersion: schemaVersion,
		CreatedAt:     time.Now().UTC().Format(time.RFC3339),
		AppliedSeq:    applied,
		SnapshotFile:  snapFile,
		SnapshotSHA:   snapSHA,
		JournalFile:   jrnFile,
		JournalSHA:    jrnSHA,
		EntityCounts:  counts,
	}
	if err := writeManifest(toDir, m); err != nil {
		return Manifest{}, err
	}
	return m, nil
}

func (s *Storage) copyJournal(dst string) error {
	in, err := os.Open(filepath.Join(s.dir, "db", "journal.jsonl"))
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func fileSHA(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func writeManifest(dir string, m Manifest) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	tmp := filepath.Join(dir, manifestFile+".tmp")
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(dir, manifestFile))
}

// ReadManifest читает манифест бандла.
func ReadManifest(dir string) (*Manifest, error) {
	data, err := os.ReadFile(filepath.Join(dir, manifestFile))
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}
