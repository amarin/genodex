package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/amarin/genodex/internal/storage"
)

// runRestore восстанавливает бандл --from в новый каталог --to.
// По умолчанию storage.Restore отказывается писать поверх живой БД; флаг
// --force разрешает повторное восстановление (удаляет db/genodex.db в цели).
func runRestore(ctx context.Context, preDataDir string, args []string) error {
	fs := newFlagSet("restore")
	_ = dataDirFlag(fs)
	from := fs.String("from", "", "backup bundle directory (required)")
	to := fs.String("to", "", "target data directory (required)")
	force := fs.Bool("force", false, "allow restore into a directory with existing database")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unknown argument %q", fs.Arg(0))
	}
	if *from == "" {
		return fmt.Errorf("--from is required")
	}
	toDir := *to
	if toDir == "" {
		toDir = chooseDataDir(fs, preDataDir)
	}

	if *force {
		for _, name := range []string{"db/genodex.db", "db/journal.jsonl"} {
			p := filepath.Join(toDir, name)
			if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("--force: remove %s: %w", p, err)
			}
		}
	}

	s, err := storage.Restore(*from, toDir)
	if err != nil {
		return fmt.Errorf("restore: %w", err)
	}
	defer func() { _ = s.Close() }()

	log.Printf("restore done: %s -> %s", *from, toDir)
	return nil
}
