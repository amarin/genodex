package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/amarin/genodex/internal/app"
	"github.com/amarin/genodex/web"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}

// run разбирает глобальный флаг --data и подкоманду. По умолчанию — serve
// (совместимость: genodex -p 9000). --data можно указывать и до, и после
// подкоманды: genodex --data X backup --to Y == genodex backup --data X --to Y.
func run(ctx context.Context, args []string) error {
	preDataDir, rest := peelDataFlag(args)

	cmd := ""
	if len(rest) > 0 {
		switch rest[0] {
		case "serve", "backup", "restore", "verify":
			cmd = rest[0]
			rest = rest[1:]
		}
	}

	switch cmd {
	case "backup":
		return runBackup(ctx, preDataDir, rest)
	case "restore":
		return runRestore(ctx, preDataDir, rest)
	case "verify":
		return runVerify(ctx, preDataDir, rest)
	case "serve":
		return runServe(ctx, preDataDir, rest)
	default:
		return runServe(ctx, preDataDir, rest)
	}
}

// peelDataFlag вынимает ведущий --data <value> (или --data=<value>) и
// возвращает значение и оставшиеся аргументы.
func peelDataFlag(args []string) (string, []string) {
	if len(args) == 0 {
		return "", args
	}
	if args[0] == "--data" && len(args) >= 2 {
		return args[1], args[2:]
	}
	if strings.HasPrefix(args[0], "--data=") {
		return strings.TrimPrefix(args[0], "--data="), args[1:]
	}
	return "", args
}

// runServe запускает HTTP-сервер (MCP + API + web) поверх хранилища в --data.
func runServe(ctx context.Context, preDataDir string, args []string) error {
	fs := newFlagSet("serve")
	_ = dataDirFlag(fs)
	port := fs.Int("p", 9000, "HTTP server port")
	webMode := fs.String("web", web.ModeProd, "web assets mode: prod (embedded) or dev (from disk)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unknown argument %q", fs.Arg(0))
	}

	a, err := app.New(app.Config{
		DataDir: chooseDataDir(fs, preDataDir),
		Port:    *port,
		WebMode: *webMode,
	})
	if err != nil {
		return err
	}
	return a.Run(ctx)
}

// chooseDataDir отдаёт каталог данных: явный --data в аргументах команды,
// иначе --data перед подкомандой, иначе дефолт (GENODEX_DATA или ".").
func chooseDataDir(fs *flag.FlagSet, pre string) string {
	explicit := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "data" {
			explicit = true
		}
	})
	if explicit {
		return fs.Lookup("data").Value.String()
	}
	if pre != "" {
		return pre
	}
	return fs.Lookup("data").Value.String()
}

func newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	return fs
}
