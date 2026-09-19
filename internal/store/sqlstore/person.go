package sqlstore

import (
	"strings"

	"github.com/amarin/genodex/internal/entity"
)

// GetPerson получает персону по ID.
func (s *Store) GetPerson(id string) (*entity.Person, error) {
	return getJSON[entity.Person](s, string(entity.TypePerson), id)
}

// SavePerson сохраняет персону; поисковый индекс — по ФИО.
func (s *Store) SavePerson(p *entity.Person) error {
	return s.saveJSON(string(entity.TypePerson), p.ID, p, fio(p))
}

// ListPeople возвращает всех персон.
func (s *Store) ListPeople() ([]*entity.Person, error) {
	return listJSON[entity.Person](s, string(entity.TypePerson))
}

func fio(p *entity.Person) string {
	return strings.Join([]string{p.Surname, p.FirstName, p.Patronymic}, " ")
}
