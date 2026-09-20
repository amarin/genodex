package sqlstore

import "github.com/amarin/genodex/internal/entity"

// GetArchive получает архив по ID.
func (s *Store) GetArchive(id string) (*entity.Archive, error) {
	return getJSON[entity.Archive](s, string(entity.TypeArchive), id)
}

// SaveArchive сохраняет архив; поисковый индекс — по названию.
func (s *Store) SaveArchive(a *entity.Archive) error {
	return s.saveJSON(string(entity.TypeArchive), a.ID, a, a.Name)
}

// ListArchives возвращает все архивы.
func (s *Store) ListArchives() ([]*entity.Archive, error) {
	return listJSON[entity.Archive](s, string(entity.TypeArchive))
}
