package search_given_names

import (
	"context"
	"errors"
	"strings"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «поиск словарных записей фамилий».
type Scenario struct {
	givenNames GivenNameRepo
}

// New создаёт сценарий.
func New(givenNames GivenNameRepo) *Scenario {
	return &Scenario{givenNames: givenNames}
}

// SearchGivenNames находит записи, чья каноническая форма (или вариант)
// начинается с текста запроса — та же механика, что и
// search_divisions.SearchDivisions (см. её комментарий).
func (s *Scenario) SearchGivenNames(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.GivenName, error) {
	if err := q.Validate(); err != nil {
		return nil, err
	}

	text := strings.TrimSpace(q.Text)
	if text == "" {
		return []models.GivenName{}, nil
	}

	page := q.Page.Normalized()
	out := []models.GivenName{}
	matched := 0

	for offset := 0; ; offset += models.MaxPageLimit {
		hits, err := s.givenNames.Search(ctx, text, access,
			models.Page{Limit: models.MaxPageLimit, Offset: offset})
		if err != nil {
			return nil, err
		}

		if len(hits) == 0 {
			return out, nil
		}

		for _, h := range hits {
			if h.Type != models.TypeGivenName {
				continue
			}

			if matched < page.Offset {
				matched++

				continue
			}

			got, err := s.givenNames.GetGivenName(ctx, h.ID)
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
