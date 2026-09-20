package sqlstore

import "github.com/amarin/genodex/internal/entity"

// GetChurch получает церковь по ID.
func (s *Store) GetChurch(id string) (*entity.Church, error) {
	return getJSON[entity.Church](s, string(entity.TypeChurch), id)
}

// SaveChurch сохраняет церковь; поисковый индекс — по названию.
func (s *Store) SaveChurch(church *entity.Church) error {
	return s.saveJSON(string(entity.TypeChurch), church.ID, church, church.Name)
}

// ListChurches возвращает все церкви.
func (s *Store) ListChurches() ([]*entity.Church, error) {
	return listJSON[entity.Church](s, string(entity.TypeChurch))
}

// GetParish получает приход по ID.
func (s *Store) GetParish(id string) (*entity.Parish, error) {
	return getJSON[entity.Parish](s, string(entity.TypeParish), id)
}

// SaveParish сохраняет приход; поисковый индекс — по названию.
func (s *Store) SaveParish(parish *entity.Parish) error {
	return s.saveJSON(string(entity.TypeParish), parish.ID, parish, parish.Name)
}

// ListParishes возвращает все приходы.
func (s *Store) ListParishes() ([]*entity.Parish, error) {
	return listJSON[entity.Parish](s, string(entity.TypeParish))
}
