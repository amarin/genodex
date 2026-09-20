package main

import (
	"context"
	"fmt"
	"log"
	"path/filepath"

	"github.com/amarin/genodex/internal/storage"
)

// runBackup создаёт бэкап-бандл каталога данных в --to (по умолчанию <data>/backups).
func runBackup(ctx context.Context, preDataDir string, args []string) error {
	fs := newFlagSet("backup")
	_ = dataDirFlag(fs)
	to := fs.String("to", "", "backup bundle directory (default: <data>/backups)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unknown argument %q", fs.Arg(0))
	}
	dataDirVal := chooseDataDir(fs, preDataDir)

	toDir := *to
	if toDir == "" {
		toDir = filepath.Join(dataDirVal, "backups")
	}
	warnSentinel(dataDirVal, toDir)

	s, err := storage.Open(dataDirVal)
	if err != nil {
		return fmt.Errorf("open data dir: %w", err)
	}
	defer func() { _ = s.Close() }()

	m, err := storage.Backup(s, toDir)
	if err != nil {
		return fmt.Errorf("backup: %w", err)
	}

	log.Printf("backup done: %s", toDir)
	log.Printf("  snapshot: %s (sha256 %s, entity_counts %d)",
		m.SnapshotFile, shortSHA(m.SnapshotSHA), len(m.EntityCounts))
	return nil
}

func shortSHA(sha string) string {
	if len(sha) > 12 {
		return sha[:12]
	}
	return sha
}
