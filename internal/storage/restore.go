package storage

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Restore создаёт новое хранилище в toDir из бандла srcDir:
// снапшот копируется как genodex.db, затем БД сверяется целостностью
// и счётчиками манифеста.
func Restore(srcDir, toDir string) (*Storage, error) {
	m, err := ReadManifest(srcDir)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}

	dbDir := filepath.Join(toDir, "db")
	if err := os.MkdirAll(dbDir, 0o755); err != nil {
		return nil, err
	}
	// не пишем поверх живой БД: если genodex.db уже существует — отказ
	dbPath := filepath.Join(dbDir, "genodex.db")
	if _, err := os.Stat(dbPath); err == nil {
		return nil, fmt.Errorf("target already contains %s", dbPath)
	}

	// 1. снапшот → genodex.db
	if err := copyFile(filepath.Join(srcDir, m.SnapshotFile), dbPath); err != nil {
		return nil, fmt.Errorf("copy snapshot: %w", err)
	}

	// 2. открываем как обычное хранилище
	s, err := Open(toDir)
	if err != nil {
		return nil, fmt.Errorf("open restored: %w", err)
	}

	// 3. сверка целостности и счётчиков (ключи — имена таблиц сущностей)
	if err := s.db.IntegrityCheck(); err != nil {
		s.Close()
		return nil, fmt.Errorf("integrity of restored db: %w", err)
	}
	for et, want := range m.EntityCounts {
		got, err := s.db.Count(et)
		if err != nil {
			s.Close()
			return nil, err
		}
		if got != want {
			s.Close()
			return nil, fmt.Errorf("count %s: got %d want %d", et, got, want)
		}
	}
	return s, nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
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
