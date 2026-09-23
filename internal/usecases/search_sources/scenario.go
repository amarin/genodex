package search_sources

import (
	"context"
	"errors"
	"strings"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «поиск записей источников».
type Scenario struct {
	sources SourceRepo
}

// New создаёт сценарий.
func New(sources SourceRepo) *Scenario {
	return &Scenario{sources: sources}
}

// SearchSources находит записи, чьи название или автор начинаются с текста
// запроса (sqlstore.SaveSource индексирует оба поля) — та же механика, что и
// search_divisions.SearchDivisions (см. её комментарий).
func (s *Scenario) SearchSources(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Source, error) {
	if err := q.Validate(); err != nil {
		return nil, err
	}

	text := strings.TrimSpace(q.Text)
	if text == "" {
		return []models.Source{}, nil
	}

	page := q.Page.Normalized()
	out := []models.Source{}
	matched := 0

	for offset := 0; ; offset += models.MaxPageLimit {
		hits, err := s.sources.Search(ctx, text, access,
			models.Page{Limit: models.MaxPageLimit, Offset: offset})
		if err != nil {
			return nil, err
		}

		if len(hits) == 0 {
			return out, nil
		}

		for _, h := range hits {
			if h.Type != models.TypeSource {
				continue
			}

			if matched < page.Offset {
				matched++

				continue
			}

			got, err := s.sources.GetSource(ctx, h.ID)
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
