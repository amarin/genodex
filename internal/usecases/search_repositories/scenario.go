package search_repositories

import (
	"context"
	"errors"
	"strings"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «поиск записей хранилищ».
type Scenario struct {
	repositories RepositoryRepo
}

// New создаёт сценарий.
func New(repositories RepositoryRepo) *Scenario {
	return &Scenario{repositories: repositories}
}

// SearchRepositories находит записи, чьё название начинается с текста
// запроса — та же механика, что и search_divisions.SearchDivisions (см. её
// комментарий).
func (s *Scenario) SearchRepositories(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Repository, error) {
	if err := q.Validate(); err != nil {
		return nil, err
	}

	text := strings.TrimSpace(q.Text)
	if text == "" {
		return []models.Repository{}, nil
	}

	page := q.Page.Normalized()
	out := []models.Repository{}
	matched := 0

	for offset := 0; ; offset += models.MaxPageLimit {
		hits, err := s.repositories.Search(ctx, text, access,
			models.Page{Limit: models.MaxPageLimit, Offset: offset})
		if err != nil {
			return nil, err
		}

		if len(hits) == 0 {
			return out, nil
		}

		for _, h := range hits {
			if h.Type != models.TypeRepository {
				continue
			}

			if matched < page.Offset {
				matched++

				continue
			}

			got, err := s.repositories.GetRepository(ctx, h.ID)
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
