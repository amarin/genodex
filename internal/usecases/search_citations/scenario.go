package search_citations

import (
	"context"
	"errors"
	"strings"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «поиск цитат».
type Scenario struct {
	citations CitationRepo
}

// New создаёт сценарий.
func New(citations CitationRepo) *Scenario {
	return &Scenario{citations: citations}
}

// SearchCitations находит цитаты, чей текст начинается с текста запроса
// (sqlstore.SaveCitation индексирует только text) — та же механика, что и
// search_divisions.SearchDivisions (см. её комментарий).
func (s *Scenario) SearchCitations(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Citation, error) {
	if err := q.Validate(); err != nil {
		return nil, err
	}

	text := strings.TrimSpace(q.Text)
	if text == "" {
		return []models.Citation{}, nil
	}

	page := q.Page.Normalized()
	out := []models.Citation{}
	matched := 0

	for offset := 0; ; offset += models.MaxPageLimit {
		hits, err := s.citations.Search(ctx, text, access,
			models.Page{Limit: models.MaxPageLimit, Offset: offset})
		if err != nil {
			return nil, err
		}

		if len(hits) == 0 {
			return out, nil
		}

		for _, h := range hits {
			if h.Type != models.TypeCitation {
				continue
			}

			if matched < page.Offset {
				matched++

				continue
			}

			got, err := s.citations.GetCitation(ctx, h.ID)
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
