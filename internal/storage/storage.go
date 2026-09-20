package storage

import (
	"os"
	"path/filepath"
)

// Storage — точка входа: SQLite-хранилище с WAL. Все записи идут через Save/Delete.
// Плоская таблица entity (JSON в колонке data) — переходный формат до нормализации;
// план работ: docs/todo.md.
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

// Save пишет полный образ сущности (JSON + нормализованный поисковый текст) в БД.
func (s *Storage) Save(entityType, id string, data, search []byte) error {
	return s.db.Upsert(entityType, id, data, search)
}

func (s *Storage) Delete(entityType, id string) error {
	return s.db.Delete(entityType, id)
}

func (s *Storage) Get(entityType, id string) ([]byte, bool, error) {
	return s.db.Get(entityType, id)
}

func (s *Storage) List(entityType string) ([]entityRow, error) {
	return s.db.List(entityType)
}

func (s *Storage) Search(entityType, query string) ([]string, error) {
	return s.db.Search(entityType, Normalize(query))
}

func (s *Storage) Count(entityType string) (int, error) { return s.db.Count(entityType) }

func (s *Storage) Close() error { return s.db.Close() }

// Dir возвращает каталог данных (нужно backup/restore).
func (s *Storage) Dir() string { return s.dir }
