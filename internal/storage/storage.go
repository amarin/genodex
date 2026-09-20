package storage

import (
	"os"
	"path/filepath"
)

// Storage — точка входа: SQLite-хранилище с WAL и колоночной схемой.
// Доступ к данным — через Storage.DB() (Exec/Query/QueryRow/Tx), которым
// пользуется internal/store/sqlstore. Само хранилище о модели не знает.
type Storage struct {
	dir string
	db  *DB
}

// Open открывает хранилище в dataDir (структура: db/genodex.db).
func Open(dataDir string) (*Storage, error) {
	dbDir := filepath.Join(dataDir, "db")
	if err := os.MkdirAll(dbDir, 0o755); err != nil {
		return nil, err
	}
	db, err := OpenDB(filepath.Join(dbDir, "genodex.db"))
	if err != nil {
		return nil, err
	}
	return &Storage{dir: dataDir, db: db}, nil
}

// DB возвращает доступ к соединению.
func (s *Storage) DB() *DB { return s.db }

// Close закрывает хранилище.
func (s *Storage) Close() error { return s.db.Close() }

// Dir возвращает каталог данных (нужно backup/restore).
func (s *Storage) Dir() string { return s.dir }
