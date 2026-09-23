package search_patronymics

import (
	"context"
	"errors"
	"strings"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «поиск словарных записей отчеств».
type Scenario struct {
	patronymics PatronymicRepo
}

// New создаёт сценарий.
func New(patronymics PatronymicRepo) *Scenario {
	return &Scenario{patronymics: patronymics}
}

// SearchPatronymics находит записи, чья каноническая форма (или вариант)
// начинается с текста запроса — та же механика, что и
// search_divisions.SearchDivisions (см. её комментарий).
func (s *Scenario) SearchPatronymics(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Patronymic, error) {
	if err := q.Validate(); err != nil {
		return nil, err
	}

	text := strings.TrimSpace(q.Text)
	if text == "" {
		return []models.Patronymic{}, nil
	}

	page := q.Page.Normalized()
	out := []models.Patronymic{}
	matched := 0

	for offset := 0; ; offset += models.MaxPageLimit {
		hits, err := s.patronymics.Search(ctx, text, access,
			models.Page{Limit: models.MaxPageLimit, Offset: offset})
		if err != nil {
			return nil, err
		}

		if len(hits) == 0 {
			return out, nil
		}

		for _, h := range hits {
			if h.Type != models.TypePatronymic {
				continue
			}

			if matched < page.Offset {
				matched++

				continue
			}

			got, err := s.patronymics.GetPatronymic(ctx, h.ID)
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
