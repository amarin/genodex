package main

import (
	"flag"
	"log"
	"os"
	"syscall"
)

// defaultDataDir возвращает каталог данных: значение GENODEX_DATA или ".data".
func defaultDataDir() string {
	if v := os.Getenv("GENODEX_DATA"); v != "" {
		return v
	}
	return ".data"
}

// dataDirFlag регистрирует флаг --data с дефолтом из GENODEX_DATA.
func dataDirFlag(fs *flag.FlagSet) *string {
	return fs.String("data", defaultDataDir(), "data directory")
}

// sameDevice возвращает true, если a и b лежат на одном устройстве.
func sameDevice(a, b string) bool {
	sa, err := os.Stat(a)
	if err != nil {
		return false
	}
	sb, err := os.Stat(b)
	if err != nil {
		return false
	}
	sta, ok1 := sa.Sys().(*syscall.Stat_t)
	stb, ok2 := sb.Sys().(*syscall.Stat_t)
	return ok1 && ok2 && sta.Dev == stb.Dev
}

// warnSentinel предупреждает, если каталог данных и бэкапа лежат на одном
// устройстве: такой бэкап не переживёт отказ диска. Предупреждение в stderr,
// не ошибка (решение за пользователем); смотри Decision Points плана.
func warnSentinel(dataDir, backupDir string) {
	if sameDevice(dataDir, backupDir) {
		log.Printf("warning: каталог данных и бэкапа на одном устройстве (%s / %s) — бэкап не защитит от отказа диска", dataDir, backupDir)
	}
}
