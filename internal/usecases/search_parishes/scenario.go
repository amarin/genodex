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
// search_divisions.SearchDivisions (см. её комментарий), включая проверку
// приватной цитаты среди источников (у Parish нет своего Private, но
// приватная цитата в источниках прячет запись целиком).
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
	// cache — мемоизация Private по id цитаты в рамках ОДНОГО вызова
	// SearchParishes (не переживает вызов, не шарится между запросами).
	cache := map[models.ID]bool{}

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

			got, err := s.parishes.GetParish(ctx, h.ID)
			if err != nil {
				if errors.Is(err, models.ErrNotFound) {
					continue
				}

				return nil, err
			}

			if access != models.AccessFull {
				hidden, err := parishReferencesPrivateCitation(ctx, s.parishes, got, cache)
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

// parishReferencesPrivateCitation сообщает, ссылается ли запись (через
// Sources[i].CitationID) хотя бы на одну приватную цитату. Независимая
// копия одноимённой функции list_parishes: пакеты сценариев в этом проекте
// самодостаточны и не делятся кодом друг с другом. cache — мемоизация в
// рамках одного вызова SearchParishes.
func parishReferencesPrivateCitation(ctx context.Context, repo ParishRepo, rec *models.Parish, cache map[models.ID]bool) (bool, error) {
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
func citationIsPrivate(ctx context.Context, repo ParishRepo, cache map[models.ID]bool, id models.ID) (bool, error) {
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
