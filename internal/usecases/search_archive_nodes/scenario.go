package search_archive_nodes

import (
	"context"
	"errors"
	"strings"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «поиск узлов архивного дерева».
type Scenario struct {
	archiveNodes ArchiveNodeRepo
}

// New создаёт сценарий.
func New(archiveNodes ArchiveNodeRepo) *Scenario {
	return &Scenario{archiveNodes: archiveNodes}
}

// SearchArchiveNodes находит узлы, чьи label/name (индексируются вместе под
// полем "name", см. internal/store/sqlstore/archive.go) начинаются с текста
// запроса, и возвращает их целиком. Результат — в том порядке, в каком их
// отдаёт Search, без фильтра по архиву (поиск глобальный по всем архивам —
// сужение по архиву, если понадобится, веб-сторона делает сама). Пустой
// текст (после обрезки) — пустой результат без обращения к репозиторию.
// Узел, ссылающийся (через Sources[i].CitationID) на приватную цитату, для
// вызывающего без полного доступа прячется, даже если сам он не приватен
// (см. комментарий get_archive_node.GetArchiveNode).
//
// Порядок проверок — как в search_events: сначала загрузка узла (пропуск
// хита без учёта offset-бюджета при ErrNotFound), затем проверка приватной
// цитаты (тот же пропуск), и только после этого — учёт offset; иначе
// скрытый хит перед запрошенным offset "съедает" часть offset-бюджета
// вслепую, и соседние страницы дублируют/теряют записи.
func (s *Scenario) SearchArchiveNodes(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.ArchiveNode, error) {
	if err := q.Validate(); err != nil {
		return nil, err
	}

	text := strings.TrimSpace(q.Text)
	if text == "" {
		return []models.ArchiveNode{}, nil
	}

	page := q.Page.Normalized()
	out := []models.ArchiveNode{}
	matched := 0 // сколько хитов узлов прошло (для сдвига окна)
	// cache — мемоизация Private по id цитаты в рамках ОДНОГО вызова
	// SearchArchiveNodes (не переживает вызов, не шарится между запросами).
	cache := map[models.ID]bool{}

	for offset := 0; ; offset += models.MaxPageLimit {
		hits, err := s.archiveNodes.Search(ctx, text, access,
			models.Page{Limit: models.MaxPageLimit, Offset: offset})
		if err != nil {
			return nil, err
		}

		if len(hits) == 0 {
			return out, nil
		}

		for _, h := range hits {
			if h.Type != models.TypeArchiveNode {
				continue
			}

			got, err := s.archiveNodes.GetArchiveNode(ctx, h.ID)
			if err != nil {
				if errors.Is(err, models.ErrNotFound) {
					continue
				}

				return nil, err
			}

			if access != models.AccessFull {
				hidden, err := archiveNodeReferencesPrivateCitation(ctx, s.archiveNodes, got, cache)
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

// archiveNodeReferencesPrivateCitation сообщает, ссылается ли узел (через
// Sources[i].CitationID) хотя бы на одну приватную цитату. Независимая
// копия одноимённой функции get_archive_node/list_archive_nodes: пакеты
// сценариев в этом проекте самодостаточны и не делятся кодом друг с
// другом. cache — мемоизация в рамках одного вызова SearchArchiveNodes.
func archiveNodeReferencesPrivateCitation(ctx context.Context, repo ArchiveNodeRepo, rec *models.ArchiveNode, cache map[models.ID]bool) (bool, error) {
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
// цитата трактуется как приватная, т.е. узел, ссылающийся на неё, тоже
// прячется, а не проваливает весь вызов ошибкой. cache — мемоизация в
// рамках одного вызова.
func citationIsPrivate(ctx context.Context, repo ArchiveNodeRepo, cache map[models.ID]bool, id models.ID) (bool, error) {
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
