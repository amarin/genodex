package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Storage — точка входа: журнал (appendix, durability) + SQLite (запросы,
// индексы). Все записи идут через Save/Delete, которые сначала пишут журнал.
type Storage struct {
	dir     string
	db      *DB
	journal *Journal
}

// Open открывает хранилище в dataDir (структура: db/, journal лежит в db/).
// При старте replay: восстанавливает пропущенные операции (seq > applied_seq)
// из журнала в БД — так поддерживается инвариант «журнал не младше БД».
func Open(dataDir string) (*Storage, error) {
	dbDir := filepath.Join(dataDir, "db")
	if err := os.MkdirAll(dbDir, 0o755); err != nil {
		return nil, err
	}
	db, err := OpenDB(filepath.Join(dbDir, "genodex.db"))
	if err != nil {
		return nil, err
	}
	j, err := OpenJournal(filepath.Join(dbDir, "journal.jsonl"))
	if err != nil {
		db.Close()
		return nil, err
	}
	s := &Storage{dir: dataDir, db: db, journal: j}
	if err := s.replay(); err != nil {
		s.Close()
		return nil, err
	}
	return s, nil
}

// replay применяет к БД записи журнала с seq > applied_seq.
func (s *Storage) replay() error {
	applied, err := s.db.AppliedSeq()
	if err != nil {
		return err
	}
	return s.journal.ReplayAll(func(e JournalEntry) error {
		if e.Seq <= applied {
			return nil
		}
		switch e.Op {
		case "save":
			if err := s.db.Upsert(e.EntityType, e.ID, e.Payload, e.Search); err != nil {
				return err
			}
		case "delete":
			if err := s.db.Delete(e.EntityType, e.ID); err != nil {
				return err
			}
		}
		return s.db.SetAppliedSeq(e.Seq)
	})
}

// Save пишет полный образ в журнал (fsync), затем в БД и обновляет applied_seq.
// search — нормализованный поисковый текст; он идёт и в журнал, чтобы replay
// восстановил search-индекс после потери БД.
func (s *Storage) Save(entityType, id string, data, search []byte) error {
	if _, err := s.journal.Append("save", entityType, id, json.RawMessage(data), search); err != nil {
		return err
	}
	if err := s.db.Upsert(entityType, id, data, search); err != nil {
		return err
	}
	return s.db.SetAppliedSeq(s.journal.LastSeq())
}

func (s *Storage) Delete(entityType, id string) error {
	if _, err := s.journal.Append("delete", entityType, id, nil, nil); err != nil {
		return err
	}
	if err := s.db.Delete(entityType, id); err != nil {
		return err
	}
	return s.db.SetAppliedSeq(s.journal.LastSeq())
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

func (s *Storage) AppliedSeq() (uint64, error) { return s.db.AppliedSeq() }

func (s *Storage) LastSeq() uint64 { return s.journal.LastSeq() }

func (s *Storage) Close() error {
	if err := s.journal.Close(); err != nil {
		s.db.Close()
		return err
	}
	return s.db.Close()
}

// Dir возвращает каталог данных (нужно backup/restore).
func (s *Storage) Dir() string { return s.dir }
