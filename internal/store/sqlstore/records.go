package sqlstore

import (
	"strings"

	"github.com/amarin/genodex/internal/entity"
)

// GetEvent получает событие по ID.
func (s *Store) GetEvent(id string) (*entity.Event, error) {
	return getJSON[entity.Event](s, string(entity.TypeEvent), id)
}

// SaveEvent сохраняет событие; поисковый индекс — по типу, дате и месту.
func (s *Store) SaveEvent(e *entity.Event) error {
	return s.saveJSON(string(entity.TypeEvent), e.ID, e, strings.Join([]string{e.EventType, e.Date, e.Place}, " "))
}

// ListEvents возвращает все события.
func (s *Store) ListEvents() ([]*entity.Event, error) {
	return listJSON[entity.Event](s, string(entity.TypeEvent))
}

// GetSource получает источник по ID.
func (s *Store) GetSource(id string) (*entity.Source, error) {
	return getJSON[entity.Source](s, string(entity.TypeSource), id)
}

// SaveSource сохраняет источник; поисковый индекс — по названию.
func (s *Store) SaveSource(src *entity.Source) error {
	return s.saveJSON(string(entity.TypeSource), src.ID, src, src.Title)
}

// ListSources возвращает все источники.
func (s *Store) ListSources() ([]*entity.Source, error) {
	return listJSON[entity.Source](s, string(entity.TypeSource))
}
