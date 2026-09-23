package search_parishes

import (
	"context"
	"errors"
	"strings"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «поиск записей приходов».
type Scenario struct {
	parishes ParishRepo
}

// New создаёт сценарий.
func New(parishes ParishRepo) *Scenario {
	return &Scenario{parishes: parishes}
}

// SearchParishes находит записи, чья каноническая форма (или вариант)
// начинается с текста запроса — та же механика, что и
// search_divisions.SearchDivisions (см. её комментарий).
func (s *Scenario) SearchParishes(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Parish, error) {
	if err := q.Validate(); err != nil {
		return nil, err
	}

	text := strings.TrimSpace(q.Text)
	if text == "" {
		return []models.Parish{}, nil
	}

	page := q.Page.Normalized()
	out := []models.Parish{}
	matched := 0

	for offset := 0; ; offset += models.MaxPageLimit {
		hits, err := s.parishes.Search(ctx, text, access,
			models.Page{Limit: models.MaxPageLimit, Offset: offset})
		if err != nil {
			return nil, err
		}

		if len(hits) == 0 {
			return out, nil
		}

		for _, h := range hits {
			if h.Type != models.TypeParish {
				continue
			}

			if matched < page.Offset {
				matched++

				continue
			}

			got, err := s.parishes.GetParish(ctx, h.ID)
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
