package search_people

import (
	"context"
	"errors"
	"strings"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «поиск персон».
type Scenario struct {
	people PersonRepo
}

// New создаёт сценарий.
func New(people PersonRepo) *Scenario {
	return &Scenario{people: people}
}

// SearchPeople находит записи, у которых фамилия/имя/отчество из ЛЮБОГО
// элемента Names начинается с текста запроса — единое поисковое поле "name"
// индексирует все имена персоны, не только основное (см.
// internal/store/sqlstore/person.go:personTerms). Та же механика окна, что
// и search_divisions.SearchDivisions (см. её комментарий).
func (s *Scenario) SearchPeople(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Person, error) {
	if err := q.Validate(); err != nil {
		return nil, err
	}

	text := strings.TrimSpace(q.Text)
	if text == "" {
		return []models.Person{}, nil
	}

	page := q.Page.Normalized()
	out := []models.Person{}
	matched := 0

	for offset := 0; ; offset += models.MaxPageLimit {
		hits, err := s.people.Search(ctx, text, access,
			models.Page{Limit: models.MaxPageLimit, Offset: offset})
		if err != nil {
			return nil, err
		}

		if len(hits) == 0 {
			return out, nil
		}

		for _, h := range hits {
			if h.Type != models.TypePerson {
				continue
			}

			if matched < page.Offset {
				matched++

				continue
			}

			got, err := s.people.GetPerson(ctx, h.ID)
			if err != nil {
				if errors.Is(err, models.ErrNotFound) {
					matched++
					continue
				}

				return nil, err
			}

			out = append(out, *got)

			if len(out) == page.Limit {
				return out, nil
			}

			matched++
		}
	}
}
