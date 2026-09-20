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
	SnapshotFile  string         `json:"snapshot_file"`
	SnapshotSHA   string         `json:"snapshot_sha256"`
	EntityCounts  map[string]int `json:"entity_counts"`
}

// Backup создаёт в toDir консистентный бандл: снапшот (VACUUM INTO) и манифест,
// который пишется последним. Порядок гарантирует: если манифест прочитался —
// значит, снапшот уже на месте и цел. Бэкап — стандартная функция обслуживания;
// для защиты от отказа диска цель должна лежать на другом устройстве (см. warnSentinel).
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

	counts := map[string]int{}
	for _, et := range []string{
		"person", "settlement", "church", "parish", "administrative_division",
		"archive", "event", "source",
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
		SnapshotFile:  snapFile,
		SnapshotSHA:   snapSHA,
		EntityCounts:  counts,
	}
	if err := writeManifest(toDir, m); err != nil {
		return Manifest{}, err
	}
	return m, nil
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
