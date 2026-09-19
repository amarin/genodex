package main

import (
	"context"
	"fmt"
	"log"

	"github.com/amarin/genodex/internal/storage"
)

// runVerify проверяет целостность журнала и БД и, если задан --backup,
// свежесть бэкап-бандла (снапшот+журнал против манифеста).
func runVerify(ctx context.Context, preDataDir string, args []string) error {
	fs := newFlagSet("verify")
	_ = dataDirFlag(fs)
	backup := fs.String("backup", "", "backup bundle directory to check (optional)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unknown argument %q", fs.Arg(0))
	}

	s, err := storage.Open(chooseDataDir(fs, preDataDir))
	if err != nil {
		return fmt.Errorf("open data dir: %w", err)
	}
	defer func() { _ = s.Close() }()

	res, err := storage.Verify(s, *backup)
	if err != nil {
		return fmt.Errorf("verify: %w", err)
	}
	log.Print(res.String())
	if !res.OK {
		return fmt.Errorf("verify failed")
	}
	return nil
}
