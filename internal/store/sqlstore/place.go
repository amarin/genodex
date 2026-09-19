package sqlstore

import "github.com/amarin/genodex/internal/entity"

// GetGovernorate получает губернию по ID.
func (s *Store) GetGovernorate(id string) (*entity.Governorate, error) {
	return getJSON[entity.Governorate](s, string(entity.TypeGovernorate), id)
}

// SaveGovernorate сохраняет губернию; поисковый индекс — по названию.
func (s *Store) SaveGovernorate(g *entity.Governorate) error {
	return s.saveJSON(string(entity.TypeGovernorate), g.ID, g, g.Name)
}

// ListGovernorates возвращает все губернии.
func (s *Store) ListGovernorates() ([]*entity.Governorate, error) {
	return listJSON[entity.Governorate](s, string(entity.TypeGovernorate))
}

// GetDistrict получает уезд по ID.
func (s *Store) GetDistrict(id string) (*entity.District, error) {
	return getJSON[entity.District](s, string(entity.TypeDistrict), id)
}

// SaveDistrict сохраняет уезд; поисковый индекс — по названию.
func (s *Store) SaveDistrict(d *entity.District) error {
	return s.saveJSON(string(entity.TypeDistrict), d.ID, d, d.Name)
}

// ListDistricts возвращает все уезды.
func (s *Store) ListDistricts() ([]*entity.District, error) {
	return listJSON[entity.District](s, string(entity.TypeDistrict))
}

// GetVolost получает волость по ID.
func (s *Store) GetVolost(id string) (*entity.Volost, error) {
	return getJSON[entity.Volost](s, string(entity.TypeVolost), id)
}

// SaveVolost сохраняет волость; поисковый индекс — по названию.
func (s *Store) SaveVolost(v *entity.Volost) error {
	return s.saveJSON(string(entity.TypeVolost), v.ID, v, v.Name)
}

// ListVolosts возвращает все волости.
func (s *Store) ListVolosts() ([]*entity.Volost, error) {
	return listJSON[entity.Volost](s, string(entity.TypeVolost))
}

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
