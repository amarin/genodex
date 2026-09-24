package search_churches

import (
	"context"
	"errors"
	"strings"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «поиск записей церквей».
type Scenario struct {
	churches ChurchRepo
}

// New создаёт сценарий.
func New(churches ChurchRepo) *Scenario {
	return &Scenario{churches: churches}
}

// SearchChurches находит записи, чья каноническая форма (или вариант)
// начинается с текста запроса — та же механика, что и
// search_divisions.SearchDivisions (см. её комментарий), включая проверку
// приватной цитаты среди источников (у Church нет своего Private, но
// приватная цитата в источниках прячет запись целиком).
func (s *Scenario) SearchChurches(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Church, error) {
	if err := q.Validate(); err != nil {
		return nil, err
	}

	text := strings.TrimSpace(q.Text)
	if text == "" {
		return []models.Church{}, nil
	}

	page := q.Page.Normalized()
	out := []models.Church{}
	matched := 0
	// cache — мемоизация Private по id цитаты в рамках ОДНОГО вызова
	// SearchChurches (не переживает вызов, не шарится между запросами).
	cache := map[models.ID]bool{}

	for offset := 0; ; offset += models.MaxPageLimit {
		hits, err := s.churches.Search(ctx, text, access,
			models.Page{Limit: models.MaxPageLimit, Offset: offset})
		if err != nil {
			return nil, err
		}

		if len(hits) == 0 {
			return out, nil
		}

		for _, h := range hits {
			if h.Type != models.TypeChurch {
				continue
			}

			got, err := s.churches.GetChurch(ctx, h.ID)
			if err != nil {
				if errors.Is(err, models.ErrNotFound) {
					continue
				}

				return nil, err
			}

			if access != models.AccessFull {
				hidden, err := churchReferencesPrivateCitation(ctx, s.churches, got, cache)
				if err != nil {
					return nil, err
				}

				if hidden {
					continue
				}
			}

			if matched < page.Offset {
				matched++

				continue
			}

			out = append(out, *got)

			if len(out) == page.Limit {
				return out, nil
			}

			matched++
		}
	}
}

// churchReferencesPrivateCitation сообщает, ссылается ли запись (через
// Sources[i].CitationID) хотя бы на одну приватную цитату. Независимая
// копия одноимённой функции list_churches: пакеты сценариев в этом проекте
// самодостаточны и не делятся кодом друг с другом. cache — мемоизация в
// рамках одного вызова SearchChurches.
func churchReferencesPrivateCitation(ctx context.Context, repo ChurchRepo, rec *models.Church, cache map[models.ID]bool) (bool, error) {
	for _, sl := range rec.Sources {
		hidden, err := citationIsPrivate(ctx, repo, cache, sl.CitationID)
		if err != nil {
			return false, err
		}

		if hidden {
			return true, nil
		}
	}

	return false, nil
}

// citationIsPrivate сообщает, приватна ли цитата id; гонка с конкурентным
// удалением цитаты трактуется как приватность, см. одноимённую функцию в
// list_divisions.
func citationIsPrivate(ctx context.Context, repo ChurchRepo, cache map[models.ID]bool, id models.ID) (bool, error) {
	if v, ok := cache[id]; ok {
		return v, nil
	}

	c, err := repo.GetCitation(ctx, id)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			cache[id] = true

			return true, nil
		}

		return false, err
	}

	cache[id] = c.Private

	return c.Private, nil
}
