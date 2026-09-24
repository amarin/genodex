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
// комментарий), с дополнительной проверкой: запись, ссылающаяся (через
// Sources[i].CitationID) на приватную цитату, для вызывающего без полного
// доступа прячется, даже если сама она не приватна (см. комментарий
// get_repository.GetRepository). Порядок проверок — как в search_events:
// сначала загрузка записи (пропуск хита без учёта offset-бюджета при
// ErrNotFound), затем проверка приватной цитаты (тот же пропуск), и только
// после этого — учёт offset; иначе скрытый хит перед запрошенным offset
// "съедает" часть offset-бюджета вслепую, и соседние страницы
// дублируют/теряют записи.
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
	// cache — мемоизация Private по id цитаты в рамках ОДНОГО вызова
	// SearchRepositories (не переживает вызов, не шарится между
	// запросами).
	cache := map[models.ID]bool{}

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

			got, err := s.repositories.GetRepository(ctx, h.ID)
			if err != nil {
				if errors.Is(err, models.ErrNotFound) {
					continue
				}

				return nil, err
			}

			if access != models.AccessFull {
				hidden, err := repositoryReferencesPrivateCitation(ctx, s.repositories, got, cache)
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

// repositoryReferencesPrivateCitation сообщает, ссылается ли запись (через
// Sources[i].CitationID) хотя бы на одну приватную цитату. Независимая
// копия одноимённой функции get_repository/list_repositories: пакеты
// сценариев в этом проекте самодостаточны и не делятся кодом друг с
// другом. cache — мемоизация в рамках одного вызова SearchRepositories.
func repositoryReferencesPrivateCitation(ctx context.Context, repo RepositoryRepo, rec *models.Repository, cache map[models.ID]bool) (bool, error) {
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

// citationIsPrivate сообщает, приватна ли цитата id — с точки зрения
// сканирующего List*/Search*-сценария сюда же относится и гонка с
// конкурентным удалением цитаты (models.ErrNotFound от GetCitation): такая
// цитата трактуется как приватная, т.е. запись, ссылающаяся на неё, тоже
// прячется, а не проваливает весь вызов ошибкой. cache — мемоизация в
// рамках одного вызова.
func citationIsPrivate(ctx context.Context, repo RepositoryRepo, cache map[models.ID]bool, id models.ID) (bool, error) {
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
