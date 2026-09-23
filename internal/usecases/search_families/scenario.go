package search_families

import (
	"context"
	"errors"
	"strings"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «поиск записей родов».
type Scenario struct {
	families FamilyRepo
}

// New создаёт сценарий.
func New(families FamilyRepo) *Scenario {
	return &Scenario{families: families}
}

// SearchFamilies находит записи, чьё название начинается с текста
// запроса — та же механика, что и search_divisions.SearchDivisions (см. её
// комментарий).
func (s *Scenario) SearchFamilies(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Family, error) {
	if err := q.Validate(); err != nil {
		return nil, err
	}

	text := strings.TrimSpace(q.Text)
	if text == "" {
		return []models.Family{}, nil
	}

	page := q.Page.Normalized()
	out := []models.Family{}
	matched := 0

	for offset := 0; ; offset += models.MaxPageLimit {
		hits, err := s.families.Search(ctx, text, access,
			models.Page{Limit: models.MaxPageLimit, Offset: offset})
		if err != nil {
			return nil, err
		}

		if len(hits) == 0 {
			return out, nil
		}

		for _, h := range hits {
			if h.Type != models.TypeFamily {
				continue
			}

			if matched < page.Offset {
				matched++

				continue
			}

			got, err := s.families.GetFamily(ctx, h.ID)
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
