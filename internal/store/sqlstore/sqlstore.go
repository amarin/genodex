package sqlstore

import (
	"encoding/json"
	"fmt"

	"github.com/amarin/genodex/internal/storage"
	"github.com/amarin/genodex/internal/store"
)

// Store — реализация порта store.Store поверх internal/storage:
// маппинг сущности ↔ (entity_type, id, JSON-блоб, поисковый индекс).
type Store struct {
	st *storage.Storage
}

var _ store.Store = (*Store)(nil)

// Open открывает адаптер в dataDir.
func Open(dataDir string) (*Store, error) {
	st, err := storage.Open(dataDir)
	if err != nil {
		return nil, fmt.Errorf("open storage: %w", err)
	}
	return &Store{st: st}, nil
}

// Close закрывает хранилище.
func (s *Store) Close() error {
	return s.st.Close()
}

// saveJSON пишет сущность: JSON-блоб + нормализованный поисковый индекс.
func (s *Store) saveJSON(entityType, id string, v any, search string) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal %s: %w", entityType, err)
	}
	return s.st.Save(entityType, id, data, []byte(storage.Normalize(search)))
}

// getJSON читает сущность по id; (nil, nil) — если не найдена.
func getJSON[T any](s *Store, entityType, id string) (*T, error) {
	data, ok, err := s.st.Get(entityType, id)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}
	var v T
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, fmt.Errorf("unmarshal %s %s: %w", entityType, id, err)
	}
	return &v, nil
}

// listJSON читает все сущности типа.
func listJSON[T any](s *Store, entityType string) ([]*T, error) {
	rows, err := s.st.List(entityType)
	if err != nil {
		return nil, err
	}
	out := make([]*T, 0, len(rows))
	for _, row := range rows {
		var v T
		if err := json.Unmarshal(row.Data, &v); err != nil {
			return nil, fmt.Errorf("unmarshal %s %s: %w", entityType, row.ID, err)
		}
		out = append(out, &v)
	}
	return out, nil
}
