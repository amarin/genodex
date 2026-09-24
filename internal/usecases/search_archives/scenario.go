package search_archives

import (
	"context"
	"errors"
	"strings"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «поиск записей архивов».
type Scenario struct {
	archives ArchiveRepo
}

// New создаёт сценарий.
func New(archives ArchiveRepo) *Scenario {
	return &Scenario{archives: archives}
}

// SearchArchives находит записи, чьё название начинается с текста запроса —
// та же механика, что и search_divisions.SearchDivisions (см. её
// комментарий), с дополнительной проверкой: запись, ссылающаяся (через
// Sources[i].CitationID) на приватную цитату, для вызывающего без полного
// доступа прячется, даже если сама она не приватна (см. комментарий
// get_archive.GetArchive). Порядок проверок — как в search_events: сначала
// загрузка записи (пропуск хита без учёта offset-бюджета при ErrNotFound),
// затем проверка приватной цитаты (тот же пропуск), и только после этого —
// учёт offset; иначе скрытый хит перед запрошенным offset "съедает" часть
// offset-бюджета вслепую, и соседние страницы дублируют/теряют записи.
func (s *Scenario) SearchArchives(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Archive, error) {
	if err := q.Validate(); err != nil {
		return nil, err
	}

	text := strings.TrimSpace(q.Text)
	if text == "" {
		return []models.Archive{}, nil
	}

	page := q.Page.Normalized()
	out := []models.Archive{}
	matched := 0
	// cache — мемоизация Private по id цитаты в рамках ОДНОГО вызова
	// SearchArchives (не переживает вызов, не шарится между запросами).
	cache := map[models.ID]bool{}

	for offset := 0; ; offset += models.MaxPageLimit {
		hits, err := s.archives.Search(ctx, text, access,
			models.Page{Limit: models.MaxPageLimit, Offset: offset})
		if err != nil {
			return nil, err
		}

		if len(hits) == 0 {
			return out, nil
		}

		for _, h := range hits {
			if h.Type != models.TypeArchive {
				continue
			}

			got, err := s.archives.GetArchive(ctx, h.ID)
			if err != nil {
				if errors.Is(err, models.ErrNotFound) {
					continue
				}

				return nil, err
			}

			if access != models.AccessFull {
				hidden, err := archiveReferencesPrivateCitation(ctx, s.archives, got, cache)
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

// archiveReferencesPrivateCitation сообщает, ссылается ли архив (через
// Sources[i].CitationID) хотя бы на одну приватную цитату. Независимая
// копия одноимённой функции get_archive/list_archives: пакеты сценариев в
// этом проекте самодостаточны и не делятся кодом друг с другом. cache —
// мемоизация в рамках одного вызова SearchArchives.
func archiveReferencesPrivateCitation(ctx context.Context, repo ArchiveRepo, rec *models.Archive, cache map[models.ID]bool) (bool, error) {
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
func citationIsPrivate(ctx context.Context, repo ArchiveRepo, cache map[models.ID]bool, id models.ID) (bool, error) {
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
